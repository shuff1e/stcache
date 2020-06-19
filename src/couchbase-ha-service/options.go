package main

import (
	"flag"
)

type options struct {
	dataDir        string // data directory
	httpAddress    string // http server address
	raftTCPAddress string // construct Raft Address
	joinAddress    string // peer address to join
	bootstrap      bool   // start as master or not
	// gossip
	gossipTCPAddress string
	gossipJoinAddress string
	gossipQuorum int
	// notification script
	verificationScript string
	notificationScript string
	//
	eventInterval int
}

func NewOptions() *options {
	opts := &options{}

	var node = flag.String("node", "node1", "raft node name")
	var httpAddress = flag.String("http", "127.0.0.1:6000", "Http address")
	var raftTCPAddress = flag.String("raft", "127.0.0.1:7000", "raft tcp address")
	var joinAddress = flag.String("join", "", "join address for raft cluster")
	var bootstrap = flag.Bool("bootstrap", false, "start as raft cluster")
	// gossip
	var gossipTCPAdderess = flag.String("gossip", "127.0.0.1:8000", "gossip tcp address")
	var gossipJoinAddress = flag.String("gossipJoin", "", "join address for gossip cluster")
	var gossipQuorum = flag.Int("gossipQuorum", 3, "gossip cluster quorum")
	// notification script
	var verificationScript = flag.String("verificationScript","./verification_script","verification script")
	var notificationScript = flag.String("notificationScript","./notification_script","notification script")
	//
	var eventInterval = flag.Int("eventInterval",60,"event interval in seconds")
	flag.Parse()

	// raft
	opts.dataDir = "./" + *node
	opts.httpAddress = *httpAddress
	opts.raftTCPAddress = *raftTCPAddress
	opts.joinAddress = *joinAddress
	opts.bootstrap = *bootstrap
	// gossip
	opts.gossipTCPAddress = *gossipTCPAdderess
	opts.gossipJoinAddress = *gossipJoinAddress
	opts.gossipQuorum = *gossipQuorum
	// notification script
	opts.verificationScript = *verificationScript
	opts.notificationScript = *notificationScript
	//
	opts.eventInterval = *eventInterval
	return opts
}
