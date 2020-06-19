package main

import (
	"fmt"
	"gopkg.in/natefinch/lumberjack.v2"
	"log"
	"net"
	"net/http"
	"strings"
)

type stCached struct {
	hs   *httpServer
	opts *options
	log  *log.Logger
	cm   *cacheManager
	raft *raftNodeInfo
	gossip GossipNode
	gcm  *gossipCacheManager
	veri verifier
}

type stCachedContext struct {
	st *stCached
}

func NewStCachedNode(opts *options,gcm *gossipCacheManager,veri verifier,logOutput *lumberjack.Logger,gnode GossipNode) *stCached {
	st := &stCached{
		opts: opts,
		log:  log.New(logOutput, "stCached: ", log.Ldate|log.Ltime),
		cm:   NewCacheManager(),
		gcm: gcm,
		gossip: gnode,
		veri: veri,
	}
	ctx := &stCachedContext{st}

	var l net.Listener
	var err error
	l, err = net.Listen("tcp", st.opts.httpAddress)
	if err != nil {
		st.log.Fatal(fmt.Sprintf("listen %s failed: %s", st.opts.httpAddress, err))
	}
	st.log.Printf("http server listen:%s", l.Addr())

	logger := log.New(logOutput, "httpserver: ", log.Ldate|log.Ltime)
	httpServer := NewHttpServer(ctx, logger)
	st.hs = httpServer
	go func() {
		http.Serve(l, httpServer.mux)
	}()

	raft, err := newRaftNode(st.opts, ctx, gcm, logOutput)
	if err != nil {
		st.log.Fatal(fmt.Sprintf("new raft node failed:%v", err))
	}
	st.raft = raft

	if st.opts.joinAddress != "" {
		err = joinRaftCluster(st.opts)
		if err != nil {
			st.log.Fatal(fmt.Sprintf("join raft cluster failed:%v", err))
		}
	}
	return st
}

// monitor leadership
func (st *stCached) monitorLeadrship() {
	for {
		select {
		case leader := <-st.raft.leaderNotifyCh:
			if leader {
				st.log.Println("become leader, enable write api")
				st.hs.setWriteFlag(true)
			} else {
				st.log.Println("become follower, close write api")
				st.hs.setWriteFlag(false)
			}
		}
	}
}

func (st *stCached) Members() []string {
	result := []string{}
	future := st.raft.raft.GetConfiguration()
	if err := future.Error();err != nil {
		st.log.Printf("error get raft Members: %s", err)
		return result
	}
	configuration := future.Configuration()
	for _,server :=  range configuration.Servers {
		result = append(result,string(server.Address))
	}
	return result
}

func (st *stCached) reJoinGossipNodes() {
	raftMembers := st.Members()
	raftMembers = st.getIP(raftMembers)
	gossipMembers := st.gossip.Members()
	gossipMembers = st.getIP(gossipMembers)

	// we can not make other members leave
	//needToLeave := []string{}
	//for _,node := range gossipMembers {
	//	if !Exists(raftMembers,node) {
	//		needToLeave = append(needToLeave,node)
	//	}
	//}

	needToJoin := []string{}
	for _,node := range raftMembers {
		if !Exists(gossipMembers,node) {
			needToJoin = append(needToJoin,node)
		}
	}

	for _,node := range needToJoin {
		n,err := st.gossip.Join(node)
		if err != nil {
			st.log.Printf("error join member %s : %s", node, err)
		}
		if n != 1 {
			st.log.Printf("error join member %s : %d", node, n)
		}
	}

}

func (st *stCached) getIP(nodes []string) []string {
	result := []string{}
	for _,node := range nodes {
		temp := strings.Split(node,":")
		if len(temp) != 2 {
			st.log.Printf("error get ip : %s",node)
			continue
		}
		result = append(result,temp[0])
	}
	return result
}