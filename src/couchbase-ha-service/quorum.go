package main

import (
	"context"
	"encoding/json"
	"fmt"
	"errors"
	"log"
	"strconv"
	"strings"
	"time"
)

type quorumChecker struct {
	quorum int
	st *stCached
	*gossipCacheManager
	gnode GossipNode
	notificationScript string
	cmdCh chan timeoutShellCmd
	resultCh chan shellCmdResult
	context context.Context
	Logger *log.Logger
	eventInterval int
}

type timeoutShellCmd struct {
	key string
	status gossipStatus
	timeout int
	cmd string
}

type shellCmdResult struct {
	key string
	status gossipStatus
	stdout string
	stderr string
	err error
}

func (q *quorumChecker) checkEvent(key string ,v *gossipConsensus) {
	if time.Since(v.prevEventTime) < time.Duration(time.Second*time.Duration(q.eventInterval)) {
		return
	}
	hasEvent,pending,status := q.hasEvent(v)
	if pending {
		q.Logger.Printf("quorum.notifyFollower found pending:%v:%v, status Now is %v",key,v,status)
		preTime := logEntryData{CMD_NOTIFY_TIME,key,
			strconv.FormatInt(time.Now().UnixNano(),10)}
		err := q.notifyFollower(preTime)
		if err != nil {
			q.Logger.Printf("quorum.notifyFollower failed:%v:%v",preTime,err)
		}
		preConsensus := logEntryData{CMD_NOTIFY_STATUS,key,string(status)}
		err = q.notifyFollower(preConsensus)
		if err != nil {
			q.Logger.Printf("quorum.notifyFollower failed:%v:%v",preConsensus,err)
		}
		//q.gossipCacheManager.SetPreviousConsensus(key,status)
	}
	if hasEvent {
		q.Logger.Printf("quorum.notifyFollower found hasEvent:%v:%v, status Now is %v",key,v,status)
		preTime := logEntryData{CMD_NOTIFY_TIME,key,
			strconv.FormatInt(time.Now().UnixNano(),10)}
		err := q.notifyFollower(preTime)
		if err != nil {
			q.Logger.Printf("quorum.notifyFollower failed:%v:%v",preTime,err)
		}
		//q.gossipCacheManager.SetPreviousEventTime(key,time.Now())
		//q.gossipCacheManager.SetPreviousConsensus(key,status)
		q.cmdCh <- timeoutShellCmd{key,status,
			notificationScriptTimeout,
			fmt.Sprintf(notificationFormat,q.notificationScript,key,q.st.cm.Get(key),string(status))}
	}
}

func (q *quorumChecker) checkEmpty(key string, v *gossipConsensus) {
	if time.Since(v.emptyTime) < time.Duration(time.Second*time.Duration(q.eventInterval)) {
		return
	}
	emptyRecover,status := q.emptyRecover(v)
	if status == GOSSIP_EMPTY {
		if !v.empty {
			q.Logger.Printf("quorum.notifyFollower found empty:%v:%v, status Now is %v",key,v,status)
			emptyTime := logEntryData{CMD_NOTIFY_EMPTY_TIME,key,
				strconv.FormatInt(time.Now().UnixNano(),10)}
			err := q.notifyFollower(emptyTime)
			if err != nil {
				q.Logger.Printf("quorum.notifyFollower failed:%v:%v",emptyTime,err)
			}
			q.cmdCh <- timeoutShellCmd{key,status,
				notificationScriptTimeout,
				fmt.Sprintf(notificationFormat,q.notificationScript,key,q.st.cm.Get(key),string(status))}
		}
	}
	if emptyRecover {
		if v.empty {
			q.Logger.Printf("quorum.notifyFollower found emptyRecover:%v:%v, status Now is %v",key,v,status)
			emptyTime := logEntryData{CMD_NOTIFY_EMPTY_TIME,key,
				strconv.FormatInt(time.Now().UnixNano(),10)}
			err := q.notifyFollower(emptyTime)
			if err != nil {
				q.Logger.Printf("quorum.notifyFollower failed:%v:%v",emptyTime,err)
			}
			q.cmdCh <- timeoutShellCmd{key,GOSSIP_EMPTY_RECOVER,
				notificationScriptTimeout,
				fmt.Sprintf(notificationFormat,q.notificationScript,key,q.st.cm.Get(key),
					string(GOSSIP_EMPTY_RECOVER))}
		}
	}
}

func (q *quorumChecker) quorumCheck1() {
	if ! q.st.hs.checkWritePermission() {
		return
	}
	barrierFuture := q.st.raft.raft.Barrier(time.Second*time.Duration(30))
	err := barrierFuture.Error()
	if err != nil {
		q.Logger.Printf("raft.Barrier failed:%v", err)
		return
	}
	data := q.gossipCacheManager.Clone()
	for key,v := range data {
		q.checkEvent(key,v)
		q.checkEmpty(key,v)
	}
}

func (q *quorumChecker) quorumCheck2() {
	for {
		select {
		case output := <-q.resultCh:
			stdout := output.stdout
			stdout = strings.TrimSuffix(stdout,"\n")
			stderr := output.stderr
			stderr = strings.TrimSuffix(stderr,"\n")
			err := output.err
			q.Logger.Println(output.key, output.status, stdout, stderr, err)
			switch gossipStatus(output.status) {
			case GOSSIP_EMPTY:
				emptyMsg := logEntryData{CMD_NOTIFY_EMPTY,output.key,strconv.FormatBool(true)}
				err := q.notifyFollower(emptyMsg)
				if err != nil {
					q.Logger.Printf("quorum.notifyFollower failed:%v:%v",emptyMsg,err)
				}
			case GOSSIP_EMPTY_RECOVER:
				emptyMsg := logEntryData{CMD_NOTIFY_EMPTY,output.key,strconv.FormatBool(false)}
				err := q.notifyFollower(emptyMsg)
				if err != nil {
					q.Logger.Printf("quorum.notifyFollower failed:%v:%v",emptyMsg,err)
				}
			default:
				preConsensus := logEntryData{CMD_NOTIFY_STATUS,output.key,string(output.status)}
				err = q.notifyFollower(preConsensus)
				if err != nil {
					q.Logger.Printf("quorum.notifyFollower failed:%v:%v",preConsensus,err)
				}
			}
			//ack := logEntryData{CMD_ACK_EVENT_NOTIFIED,output.key,string(output.status)}
			//q.notifyFollower(ack)
		case <-q.context.Done():
			return
		}
	}
}

func (q *quorumChecker) notifyFollower(event logEntryData) error {
	if !q.st.hs.checkWritePermission() {
		return errors.New("nofity follower can only be performed on leader")
	}
	eventBytes, err := json.Marshal(event)
	if err != nil {
		q.Logger.Printf("json.Marshal failed, err:%v", err)
		return err
	}
	applyFuture := q.st.raft.raft.Apply(eventBytes,5*time.Second)
	if err := applyFuture.Error(); err != nil {
		q.Logger.Printf("raft.Apply failed:%v", err)
		return err
	}
	return nil
}

func (q *quorumChecker) doTimeoutShellCmd() {
	pool := New(100)
	for {
		select {
		case tsc := <- q.cmdCh:
			pool.Add(1)
			go func(tsc timeoutShellCmd) {
				defer pool.Done()
				if !q.st.hs.checkWritePermission() {
					return
				}
				stdout,stderr,err := ShellCmdTimeout(tsc.timeout,"sh","-c",tsc.cmd)
				//q.Logger.Println(stdout,stderr,err)
				q.resultCh <- shellCmdResult{tsc.key,tsc.status,stdout,stderr,err}
			}(tsc)
		case <-q.context.Done():
			return
		}
	}
}

//type  eventInterval struct {
//	mutex sync.Mutex
//	notify <-chan time.Time
//}
//
//var eveInte *eventInterval
func (q *quorumChecker) hasEvent(v *gossipConsensus) (hasEvent bool,pending bool,status gossipStatus) {
	countMap := make(map[gossipStatus]int)
	for _,status := range v.consensus {
		if status.timeStamp <= v.prevEventTime.UnixNano() {
			continue
		}
		if time.Since(time.Unix(status.timeStamp/1e9,status.timeStamp%1e9)) > time.Second * STALE_INFO_INTERVAL_IN_SECONDS {
			continue
		}
		countMap[status.gossipStatus] += 1
	}
	var statusNow gossipStatus
	for status,count := range countMap {
		if count >= q.quorum {
			statusNow = status
			break
		}
	}
	if statusNow == GOSSIP_DOWN {
		if v.prevConsensus == GOSSIP_UP {
			return true,false,GOSSIP_DOWN
		}
		if v.prevConsensus != GOSSIP_DOWN {
			return false,true,GOSSIP_DOWN
		}
	}
	if statusNow == GOSSIP_UP {
		if v.prevConsensus == GOSSIP_DOWN {
			return true,false,GOSSIP_UP
		}
		if v.prevConsensus != GOSSIP_UP {
			return false,true,GOSSIP_UP
		}
	}
	return false,false,statusNow
}

func (q *quorumChecker) emptyRecover(v *gossipConsensus) (emptyRecover bool,status gossipStatus) {
	recoverCount := 0
	countMap := make(map[gossipStatus]int)
	for _,status := range v.consensus {
		if status.timeStamp <= v.emptyTime.UnixNano() {
			continue
		}
		if time.Since(time.Unix(status.timeStamp/1e9,status.timeStamp%1e9)) > time.Second * STALE_INFO_INTERVAL_IN_SECONDS {
			continue
		}
		if status.gossipStatus != GOSSIP_LESS_THAN_MINSAMPLE &&
			status.gossipStatus != GOSSIP_NOT_FILL_SAMPLES &&
			status.gossipStatus != GOSSIP_EMPTY {
			recoverCount += 1
		}
		countMap[status.gossipStatus] += 1
	}
	var statusNow gossipStatus
	for status,count := range countMap {
		if count >= q.quorum {
			statusNow = status
			break
		}
	}
	if statusNow == GOSSIP_EMPTY {
		return false,statusNow
	}
	if recoverCount >= q.quorum {
		return true,statusNow
	}
	return false,statusNow
}
