package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	gorilla "github.com/gorilla/mux"
	"github.com/hashicorp/raft"
)

const (
	ENABLE_WRITE_TRUE  = int32(1)
	ENABLE_WRITE_FALSE = int32(0)
)

type command string

const (
	CMD_SET = command("SET")
	CMD_DELETE = command("DELETE")
	//CMD_ACK_EVENT_NOTIFIED = command("ACK_EVENT_NOTIFIED")
	CMD_NOTIFY_STATUS = command("NOTIFY_STATUS")
	CMD_NOTIFY_TIME = command("NOTIFY_TIME")
	CMD_NOTIFY_EMPTY_TIME = command("NOTIFY_EMPTY_TIME")
	CMD_NOTIFY_EMPTY = command("NOTIFY_EMPTY")
)

type httpServer struct {
	ctx         *stCachedContext
	log         *log.Logger
	//mux         *http.ServeMux
	mux         *gorilla.Router
	enableWrite int32
}

func NewHttpServer(ctx *stCachedContext, log *log.Logger) *httpServer {
	//mux := http.NewServeMux()
	mux := gorilla.NewRouter()
	s := &httpServer{
		ctx:         ctx,
		log:         log,
		mux:         mux,
		enableWrite: ENABLE_WRITE_FALSE,
	}

	mux.HandleFunc("/set", s.doSet)
	mux.HandleFunc("/get", s.doGet)
	mux.HandleFunc("/del", s.doDel)

	mux.HandleFunc("/join", s.doJoin)
	mux.HandleFunc("/leave", s.doLeave)
	// new
	mux.HandleFunc("/gossipjoin/{nodes}", s.doGossipJoin)
	mux.HandleFunc("/gossipleave", s.doGossipLeave)

	mux.HandleFunc("/status", s.doStatus)
	mux.HandleFunc("/status/{key}", s.doStatusForKey)

	mux.HandleFunc("/gossipstatus", s.doGossipStatus)
	mux.HandleFunc("/gossipstatus/{key}", s.doGossipStatusForKey)

	// new
	mux.HandleFunc("/leader", s.doLeader)
	mux.HandleFunc("/membersconsensus", s.doMembersConsensus)

	mux.HandleFunc("/gossipnodes", s.doGossipNodes)
	mux.HandleFunc("/raftnodes", s.doNodes)

	switch t := s.ctx.st.veri.(type) {
	case *commonVerifier:
		mux.HandleFunc("/detectordata", t.detector.doDetectorData)
		mux.HandleFunc("/detectordata/{key}", t.detector.doDetectorDataForKey)
	default:
	}
	return s
}

func (h *httpServer) checkWritePermission() bool {
	return atomic.LoadInt32(&h.enableWrite) == ENABLE_WRITE_TRUE
}

func (h *httpServer) setWriteFlag(flag bool) {
	if flag {
		atomic.StoreInt32(&h.enableWrite, ENABLE_WRITE_TRUE)
	} else {
		atomic.StoreInt32(&h.enableWrite, ENABLE_WRITE_FALSE)
	}
}

func (h *httpServer) doGet(w http.ResponseWriter, r *http.Request) {
	vars := r.URL.Query()

	key := vars.Get("key")
	if key == "" {
		h.log.Println("doGet() error, get nil key")
		fmt.Fprint(w, "")
		return
	}

	ret := h.ctx.st.cm.Get(key)
	fmt.Fprintf(w, "%s\n", ret)
}

// doSet saves data to cache, only raft master node provides this api
func (h *httpServer) doSet(w http.ResponseWriter, r *http.Request) {
	if !h.checkWritePermission() {
		fmt.Fprintf(w, "write method not allowed, leader is %s\n", h.ctx.st.raft.raft.Leader())
		return
	}
	vars := r.URL.Query()

	key := vars.Get("key")
	value := vars.Get("value")
	if key == "" || value == "" {
		h.log.Println("doSet() error, get nil key or nil value")
		fmt.Fprint(w, "param error\n")
		return
	}

	event := logEntryData{Cmd: CMD_SET, Key: key, Value: value}
	eventBytes, err := json.Marshal(event)
	if err != nil {
		h.log.Printf("json.Marshal failed, err:%v", err)
		fmt.Fprint(w, "internal error\n")
		return
	}

	applyFuture := h.ctx.st.raft.raft.Apply(eventBytes, 5*time.Second)
	if err := applyFuture.Error(); err != nil {
		h.log.Printf("raft.Apply failed:%v", err)
		fmt.Fprint(w, "internal error\n")
		return
	}

	fmt.Fprintf(w, "ok\n")
}

func (h *httpServer) doDel(w http.ResponseWriter, r *http.Request) {
	if !h.checkWritePermission() {
		fmt.Fprintf(w, "delete method not allowed, leader is %s\n", h.ctx.st.raft.raft.Leader())
		return
	}
	vars := r.URL.Query()

	key := vars.Get("key")
	if key == "" {
		h.log.Println("doDel() error, get nil key")
		fmt.Fprint(w, "")
		return
	}

	event := logEntryData{Cmd:CMD_DELETE, Key: key, Value: ""}
	eventBytes, err := json.Marshal(event)
	if err != nil {
		h.log.Printf("json.Marshal failed, err:%v", err)
		fmt.Fprint(w, "internal error\n")
		return
	}

	applyFuture := h.ctx.st.raft.raft.Apply(eventBytes, 5*time.Second)
	if err := applyFuture.Error(); err != nil {
		h.log.Printf("raft.Apply failed:%v", err)
		fmt.Fprint(w, "internal error\n")
		return
	}

	fmt.Fprintf(w, "ok\n")

}

// doJoin handles joining cluster request
func (h *httpServer) doJoin(w http.ResponseWriter, r *http.Request) {
	if !h.checkWritePermission() {
		fmt.Fprintf(w, "raft join method not allowed, leader is %s\n", h.ctx.st.raft.raft.Leader())
		return
	}
	vars := r.URL.Query()

	peerAddress := vars.Get("peerAddress")
	if peerAddress == "" {
		h.log.Println("invalid PeerAddress")
		fmt.Fprint(w, "invalid peerAddress\n")
		return
	}
	addPeerFuture := h.ctx.st.raft.raft.AddVoter(raft.ServerID(peerAddress), raft.ServerAddress(peerAddress), 0, time.Second*10)
	if err := addPeerFuture.Error(); err != nil {
		h.log.Printf("Error joining peer to raft, peeraddress:%s, err:%v, code:%d", peerAddress, err, http.StatusInternalServerError)
		fmt.Fprint(w, "internal error\n")
		return
	}
	fmt.Fprint(w, "ok\n")
}

// doLeave handles leaving cluster request
func (h *httpServer) doLeave(w http.ResponseWriter, r *http.Request) {
	if !h.checkWritePermission() {
		fmt.Fprintf(w, "raft leave method not allowed, leader is %s\n", h.ctx.st.raft.raft.Leader())
		return
	}
	vars := r.URL.Query()

	peerAddress := vars.Get("peerAddress")
	if peerAddress == "" {
		h.log.Println("invalid PeerAddress")
		fmt.Fprint(w, "invalid peerAddress\n")
		return
	}
	removeServerFuture := h.ctx.st.raft.raft.RemoveServer(raft.ServerID(peerAddress),0,time.Second*10)
	if err := removeServerFuture.Error(); err != nil {
		h.log.Printf("Error remove peer from raft, peeraddress:%s, err:%v, code:%d", peerAddress, err, http.StatusInternalServerError)
		fmt.Fprint(w, "internal error\n")
		return
	}
	fmt.Fprint(w, "ok\n")
}

func (h *httpServer) doGossipJoin(w http.ResponseWriter, r *http.Request) {
	vars := gorilla.Vars(r)
	nodes := vars["nodes"]

	peers := strings.Split(nodes,",")
	n,err := h.ctx.st.gossip.Join(peers...)
	if err != nil {
		h.log.Printf("doGossipJoin failed, err:%v", err)
		fmt.Fprint(w, "internal error\n")
		return
	}
	if n != len(peers) {
		h.log.Printf("doGossipJoin failed, %d:%v", n,peers)
		fmt.Fprint(w, "internal error\n")
		return
	}
	fmt.Fprint(w, "ok\n")
}


// https://github.com/hashicorp/memberlist/issues/140
func (h *httpServer) doGossipLeave(w http.ResponseWriter, r *http.Request) {
	err := h.ctx.st.gossip.Leave(time.Second*GOSSIP_LEAVE_INTERVAL_IN_SECONDS)
	if err != nil {
		h.log.Printf("doGossipLeave failed, err:%v", err)
		fmt.Fprint(w, "internal error\n")
		return
	}
	fmt.Fprint(w, "ok\n")
}

func (h *httpServer) doStatus(w http.ResponseWriter, r *http.Request) {
	dataBytes,err := h.ctx.st.cm.Marshal()
	if err != nil {
		h.log.Printf("doStatus failed, err:%v", err)
		fmt.Fprint(w, "internal error\n")
		return
	}
	io.WriteString(w,string(dataBytes))
}

func (h *httpServer) doStatusForKey(w http.ResponseWriter, r *http.Request) {
	vars := gorilla.Vars(r)
	key := vars["key"]

	value,err := h.ctx.st.cm.GetKey(key)
	if err != nil {
		h.log.Printf("doStatusForKey failed, err:%v", err)
		fmt.Fprint(w, err)
		return
	}
	dataBytes, err := json.Marshal(map[string]string{key:value})
	if err != nil {
		h.log.Printf("doStatusForKey failed, err:%v", err)
		fmt.Fprint(w, "internal error\n")
		return
	}
	io.WriteString(w,string(dataBytes))
}

func (h *httpServer) doGossipStatus(w http.ResponseWriter, r *http.Request) {
	dataBytes,err := h.ctx.st.gcm.Marshal()
	if err != nil {
		h.log.Printf("doGossipStatus failed, err:%v", err)
		fmt.Fprint(w, "internal error\n")
		return
	}
	io.WriteString(w,string(dataBytes))
}

func (h *httpServer) doGossipStatusForKey(w http.ResponseWriter, r *http.Request) {
	vars := gorilla.Vars(r)
	key := vars["key"]
	value,err := h.ctx.st.gcm.GetKey(key)
	if err != nil {
		h.log.Printf("doGossipStatusForKey failed, err:%v", err)
		fmt.Fprint(w, err)
		return
	}
	dataBytes, err := json.Marshal(map[string]tempGossipConsensus{key:value})
	if err != nil {
		h.log.Printf("doGossipStatusForKey failed, err:%v", err)
		fmt.Fprint(w, "internal error\n")
		return
	}
	io.WriteString(w,string(dataBytes))
}

func (h *httpServer) doGossipNodes(w http.ResponseWriter, r *http.Request) {
	peers := h.ctx.st.gossip.Members()
	dataBytes, err := json.Marshal(peers)
	if err != nil {
		h.log.Printf("doGossipNodes failed, err:%v", err)
		fmt.Fprint(w, "internal error\n")
		return
	}
	fmt.Fprint(w,string(dataBytes))
}

func (h *httpServer) doLeader(w http.ResponseWriter, r *http.Request) {
	leader := h.ctx.st.raft.raft.Leader()
	if string(leader) == "" {
		h.log.Println("doLeader failed, err no leader")
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, "error no leader\n")
	} else {
		fmt.Fprint(w, fmt.Sprintf("%s\n",leader))
	}
}

func (h *httpServer) doNodes(w http.ResponseWriter, r *http.Request) {
	peers := h.ctx.st.Members()
	dataBytes, err := json.Marshal(peers)
	if err != nil {
		h.log.Printf("doNodes failed, err:%v", err)
		fmt.Fprint(w, "internal error\n")
		return
	}
	fmt.Fprint(w,string(dataBytes))
}

func (h *httpServer) doMembersConsensus(w http.ResponseWriter, r *http.Request) {
	raftMembers := h.ctx.st.Members()
	raftMembers = h.ctx.st.getIP(raftMembers)
	sort.Strings(raftMembers)

	gossipMembers := h.ctx.st.gossip.Members()
	gossipMembers = h.ctx.st.getIP(gossipMembers)
	sort.Strings(gossipMembers)

	if !SliceEqual(raftMembers,gossipMembers) {
		h.log.Printf("doMembersConsensu failed, err raft members %v : gossip members %v", raftMembers,gossipMembers)
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, fmt.Sprintf("error raft members %v , gossip members %s differ\n",raftMembers,gossipMembers))
		return
	}

	fmt.Fprint(w, "ok\n")
}