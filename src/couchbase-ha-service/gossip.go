package main

import (
	"github.com/hashicorp/memberlist"
	"gopkg.in/natefinch/lumberjack.v2"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"
)

type GossipNode interface {
	Addr() string
	Members() []string
	SetBroadcasts([]MyBroadcastMessage)
	GetMessages() []MyBroadcastMessage
	Join(...string) (int,error)
	Leave(timeout time.Duration) error
}

type gossipNode struct {
	addr string
	memberlist *memberlist.Memberlist
	delegate *MyDelegate
}

func (n *gossipNode) Addr() string {
	return n.addr
}

func (n *gossipNode) Members() []string {
	result := []string{}
	for _,v := range n.memberlist.Members() {
		result = append(result,v.String())
	}
	return result
}

func (n *gossipNode) SetBroadcasts(data []MyBroadcastMessage) {
	n.delegate.setBroadcasts(data)
}

func (n *gossipNode) GetMessages() []MyBroadcastMessage {
	return n.delegate.getMessages()
}

func (n *gossipNode) Join(peers ...string) (int,error) {
	return n.memberlist.Join(peers)
}

func (n *gossipNode) Leave(timeout time.Duration) error {
	return n.memberlist.Leave(timeout)
}

func NewGossipNode(addr, cluster string,opts *options,logOutput *lumberjack.Logger) (GossipNode, error) {
	e := new(MyEventDelegate)
	e.Num = 0

	d := new(MyDelegate)
	d.Logger = log.New(logOutput, "delegate: ", log.Ldate|log.Ltime)
	d.setMeta(addr)
	d.broadcasts = new(memberlist.TransmitLimitedQueue)
	d.broadcasts.NumNodes = func() int {
		//log.Printf("broadcast nodes = %d", e.Num)
		return e.Num
	}
	d.msgs = make([]MyBroadcastMessage,0)

	d.meta = []byte{}
	d.remoteState = []byte{}
	d.state = []byte{}
	d.mu = sync.Mutex{}

	conf := memberlist.DefaultLANConfig()
	d.broadcasts.RetransmitMult = conf.RetransmitMult
	conf.Name = addr
	conf.BindAddr = strings.Split(addr,":")[0]
	port,err := strconv.Atoi(strings.Split(addr,":")[1])
	if err != nil {
		return nil,err
	}
	conf.BindPort = port
	conf.Delegate = d
	conf.Events = e
	conf.Logger = log.New(logOutput,"HA service gossip: ",log.Ldate|log.Ltime)
	//conf.LogOutput = ioutil.Discard
	l, err := memberlist.Create(conf)
	if err != nil {
		return nil, err
	}
	//fmt.Println(l.Members())
	if cluster == "" {
		cluster = addr
	}
	clu := []string{cluster}
	_, err = l.Join(clu)
	if err != nil {
		return nil, err
	}
	return &gossipNode{addr,l,d}, nil
}

