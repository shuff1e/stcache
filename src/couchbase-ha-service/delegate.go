package main

import (
	"context"
	"encoding/json"
	"github.com/hashicorp/memberlist"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

type MyDelegate struct {
	mu          sync.Mutex
	meta        []byte
	state       []byte
	remoteState []byte

	broadcasts *memberlist.TransmitLimitedQueue
	msgs        []MyBroadcastMessage
	Logger *log.Logger
}

// meta
func (m *MyDelegate) setMeta(meta string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.meta = []byte(meta)
}

func (m *MyDelegate) NodeMeta(limit int) []byte {
	m.mu.Lock()
	defer m.mu.Unlock()

	// MetaMaxSize is 512 bytes
	if len(m.meta) > limit {
		return m.meta[0:limit]
	}

	return m.meta
}

// state
func (m *MyDelegate) setState(state string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.state = []byte(state)
}

func (m *MyDelegate) LocalState(join bool) []byte {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.state
}

// broadcast
func (m *MyDelegate) setBroadcasts(broadcasts []MyBroadcastMessage) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _,msg := range broadcasts {
		m.broadcasts.QueueBroadcast(msg)
	}
}

func (d *MyDelegate) GetBroadcasts(overhead, limit int) [][]byte {
	return d.broadcasts.GetBroadcasts(overhead, limit)
}

// remote state
func (m *MyDelegate) getRemoteState() string {
	m.mu.Lock()
	defer m.mu.Unlock()

	out := make([]byte, len(m.remoteState))
	copy(out, m.remoteState)
	return string(out)
}

func (m *MyDelegate) MergeRemoteState(buf []byte, join bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.remoteState = buf
}

// msg
func (m *MyDelegate) getMessages() []MyBroadcastMessage {
	m.mu.Lock()
	defer m.mu.Unlock()

	out := m.msgs
	m.msgs = make([]MyBroadcastMessage,0)
	return out
}

func (m *MyDelegate) NotifyMsg(msg []byte) {
	m.mu.Lock()
	defer m.mu.Unlock()

	cp := make([]byte, len(msg))
	copy(cp, msg)
	if v,ok := ParseMyBroadcastMessage(cp);ok {
		m.msgs = append(m.msgs, *v)
	}
}


// Broadcast
type MyBroadcastMessage struct {
	TimeStamp int64 `json:"timestamp"`
	Node   string  `json:"node"`
	Key    string  `json:"key"`
	Value  string  `json:"value"`
	Status gossipStatus  `json:"status"`
}
func (m MyBroadcastMessage) Invalidates(other memberlist.Broadcast) bool {
	// Check if that broadcast is a memberlist type
	mb, ok := other.(MyBroadcastMessage)
	if !ok {
		return false
	}

	// Invalidates any message about the same node
	return (m.Node == mb.Node) && (m.Key == mb.Key) && (m.TimeStamp > mb.TimeStamp)
}

func (m MyBroadcastMessage) Finished() {
	// nop
}
func (m MyBroadcastMessage) Message() []byte {
	data, err := json.Marshal(m)
	if err != nil {
		return []byte("")
	}
	return data
}

func ParseMyBroadcastMessage(data []byte) (*MyBroadcastMessage, bool) {
	msg := new(MyBroadcastMessage)
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, false
	}
	return msg, true
}

func wait_signal(cancel context.CancelFunc){
	signal_chan := make(chan os.Signal, 1)

	signal.Notify(signal_chan, syscall.SIGINT)
	for {
		select {
		case s := <-signal_chan:
			log.Printf("signal %s happen", s.String())
			cancel()
		}
	}
}

//
type MyEventDelegate struct {
	Num int
}
func (d *MyEventDelegate) NotifyJoin(node *memberlist.Node) {
	d.Num += 1
}
func (d *MyEventDelegate) NotifyLeave(node *memberlist.Node) {
	d.Num -= 1
}
func (d *MyEventDelegate) NotifyUpdate(node *memberlist.Node) {
	// skip
}
