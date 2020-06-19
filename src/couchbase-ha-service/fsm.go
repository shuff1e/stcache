package main

import (
	"encoding/json"
	"io"
	"log"
	"strconv"
	"time"

	"github.com/hashicorp/raft"
)

type FSM struct {
	ctx *stCachedContext
	log *log.Logger
	gcm *gossipCacheManager
}

type logEntryData struct {
	Cmd   command
	Key   string
	Value string
}

// Apply applies a Raft log entry to the key-value store.
func (f *FSM) Apply(logEntry *raft.Log) interface{} {
	e := logEntryData{}
	if err := json.Unmarshal(logEntry.Data, &e); err != nil {
		panic("Failed unmarshaling Raft log entry. This is a bug.")
	}
	var ret error
	switch e.Cmd {
	case CMD_SET:
		ret = f.ctx.st.cm.Set(e.Key, e.Value)
	case CMD_DELETE:
		ret = f.ctx.st.cm.Del(e.Key)
	case CMD_NOTIFY_STATUS:
		ret = f.gcm.SetPreviousConsensus(e.Key,gossipStatus(e.Value))
	case CMD_NOTIFY_TIME:
		timeStamp, err := strconv.ParseInt(e.Value, 10, 64)
		if err != nil {
			f.log.Printf("fsm.Apply(), logEntry:%s, notify_time error:%v\n", logEntry.Data, err)
		} else {
			ret = f.gcm.SetPreviousEventTime(e.Key,time.Unix(timeStamp/1e9,timeStamp%1e9))
		}
	case CMD_NOTIFY_EMPTY:
		empty,err := strconv.ParseBool(e.Value)
		if err != nil {
			f.log.Printf("fsm.Apply(), logEntry:%s, notify_empty error:%v\n", logEntry.Data, err)
		} else {
			ret = f.gcm.SetEmpty(e.Key,empty)
		}
	case CMD_NOTIFY_EMPTY_TIME:
		timeStamp, err := strconv.ParseInt(e.Value, 10, 64)
		if err != nil {
			f.log.Printf("fsm.Apply(), logEntry:%s, notify_time error:%v\n", logEntry.Data, err)
		} else {
			ret = f.gcm.setEmptyTime(e.Key,time.Unix(timeStamp/1e9,timeStamp%1e9))
		}
	default:
	}
	f.log.Printf("fsm.Apply(), logEntry:%s, ret:%v\n", logEntry.Data, ret)
	return ret
}

// Snapshot returns a latest snapshot
func (f *FSM) Snapshot() (raft.FSMSnapshot, error) {
	return &snapshot{cm: f.ctx.st.cm}, nil
}

// Restore stores the key-value store to a previous state.
func (f *FSM) Restore(serialized io.ReadCloser) error {
	return f.ctx.st.cm.UnMarshal(serialized)
}
