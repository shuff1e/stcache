package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"
)

type gossipStatus string

const (
// ping发出的状态
GOSSIP_UP  = gossipStatus("UP")
GOSSIP_DOWN = gossipStatus("DOWN")
GOSSIP_EMPTY = gossipStatus("EMPTY")
// 其他状态，例如pending

// phi发出的状态
GOSSIP_LESS_THAN_MINSAMPLE = gossipStatus("LESS_THAN_MINSAMPLE")
GOSSIP_NOT_FILL_SAMPLES = gossipStatus("NOT_FILL_SAMPLES")
GOSSIP_NO_EMPTY_IN_SAMPLES = gossipStatus("NO_EMPTY_IN_SAMPLES")

// quorum发出的状态
GOSSIP_EMPTY_RECOVER = gossipStatus("EMPTY_RECOVER")

// gcm发出的状态
GOSSIP_KEY_NODE_NOT_EXISTS = gossipStatus("KEY_NODE_NOT_EXISTS")
)

type gossipConsensus struct {
	prevEventTime time.Time
	prevConsensus gossipStatus
	empty bool
	emptyTime time.Time
	consensus map[string]gossipStatusAndTimeStamp
}

type gossipStatusAndTimeStamp struct {
	gossipStatus gossipStatus
	timeStamp int64
}

type gossipCacheManager struct {
	// key -> node -> status
	data map[string]*gossipConsensus
	sync.RWMutex
	gnode GossipNode
}

func NewGossipCacheManager(gnode GossipNode) *gossipCacheManager {
	cm := &gossipCacheManager{}
	cm.data = make(map[string]*gossipConsensus)
	cm.RWMutex = sync.RWMutex{}
	cm.gnode = gnode
	return cm
}

func (c *gossipCacheManager) GetKeyNode(key , nodeAddr string) (ret gossipStatus) {
	c.RLock()
	defer c.RUnlock()
	if _,ok := c.data[key];ok {
		if _,ok = c.data[key].consensus[nodeAddr];ok {
			ret = c.data[key].consensus[nodeAddr].gossipStatus
		}
	}
	ret = GOSSIP_KEY_NODE_NOT_EXISTS
	return
}

func (c *gossipCacheManager) Set(key,value, nodeAddr string, timestamp int64, status gossipStatus ) error {
	c.Lock()
	defer c.Unlock()
	if _,ok := c.data[key];ok {
		if _,ok := c.data[key].consensus[nodeAddr];ok {
			if timestamp > c.data[key].consensus[nodeAddr].timeStamp {
				c.data[key].consensus[nodeAddr] = gossipStatusAndTimeStamp{status,timestamp}
				c.gnode.SetBroadcasts([]MyBroadcastMessage{MyBroadcastMessage{timestamp,nodeAddr,key,value,status}})
			}
		} else {
			c.data[key].consensus[nodeAddr] = gossipStatusAndTimeStamp{status,timestamp}
			c.gnode.SetBroadcasts([]MyBroadcastMessage{MyBroadcastMessage{timestamp,nodeAddr,key,value,status}})
		}
	} else {
		c.data[key] = &gossipConsensus{
			consensus:map[string]gossipStatusAndTimeStamp{nodeAddr:{status,timestamp}}}
		c.gnode.SetBroadcasts([]MyBroadcastMessage{MyBroadcastMessage{timestamp,nodeAddr,key,value,status}})
	}
	return nil
}

func (c *gossipCacheManager) SetPreviousConsensus(key string, status gossipStatus ) error {
	c.Lock()
	defer c.Unlock()
	if _,ok := c.data[key];ok {
		c.data[key].prevConsensus = status
	} else {
		c.data[key] = &gossipConsensus{prevConsensus:status}
	}
	return nil
}

func (c *gossipCacheManager) GetPreviousConsensus(key string) (gossipStatus,bool) {
	c.Lock()
	defer c.Unlock()
	if consensus,ok := c.data[key];ok {
		return consensus.prevConsensus,ok
	}
	return GOSSIP_KEY_NODE_NOT_EXISTS,false
}

func (c *gossipCacheManager) SetPreviousEventTime(key string, eventTime time.Time) error {
	c.Lock()
	defer c.Unlock()
	if _,ok := c.data[key];ok {
		c.data[key].prevEventTime = eventTime
	} else {
		c.data[key] = &gossipConsensus{prevEventTime:eventTime}
	}
	return nil
}

func (c *gossipCacheManager) GetEmpty(key string) (empty bool,ok bool) {
	c.Lock()
	defer c.Unlock()
	if consensus,ok := c.data[key];ok {
		return consensus.empty,ok
	}
	return false,false
}
func (c *gossipCacheManager) SetEmpty(key string, empty bool) error {
	c.Lock()
	defer c.Unlock()
	if _,ok := c.data[key];ok {
		c.data[key].empty = empty
	} else {
		c.data[key] = &gossipConsensus{empty:empty}
	}
	return nil
}

func (c *gossipCacheManager) setEmptyTime(key string,emptyTime time.Time) error {
	c.Lock()
	defer c.Unlock()
	if _,ok := c.data[key];ok {
		c.data[key].emptyTime = emptyTime
	} else {
		c.data[key] = &gossipConsensus{emptyTime:emptyTime}
	}
	return nil
}

func (c *gossipCacheManager) DelNodeAddr(key, nodeAddr string) error {
	c.Lock()
	defer c.Unlock()
	if _,ok := c.data[key];ok {
		delete(c.data[key].consensus, nodeAddr)
	}
	return nil
}

func (c *gossipCacheManager) DelKey(key string) error {
	c.Lock()
	defer c.Unlock()
	delete(c.data,key)
	return nil
}

// copy on write
func (c *gossipCacheManager) Clone() map[string]*gossipConsensus {
	c.RLock()
	defer c.RUnlock()
	result := make(map[string]*gossipConsensus)
	for key,v := range c.data {
		temp := &gossipConsensus{consensus:make(map[string]gossipStatusAndTimeStamp)}
		temp.prevConsensus = v.prevConsensus
		temp.prevEventTime = v.prevEventTime
		temp.emptyTime = v.emptyTime
		temp.empty = v.empty
		for nodeAddr,status := range v.consensus {
			temp.consensus[nodeAddr] = status
		}
		result[key] = temp
	}
	return result
}

type tempGossipStatusAndTimeStamp struct {
	GossipStatus gossipStatus
	TimeStamp string
}

type tempGossipConsensus struct {
	PrevEventTime time.Time
	PrevConsensus gossipStatus
	Empty bool
	EmptyTime time.Time
	Consensus map[string]tempGossipStatusAndTimeStamp
}

// Marshal serializes cache data
func (c *gossipCacheManager) Marshal() ([]byte, error) {
	c.RLock()
	defer c.RUnlock()

	result := make(map[string]tempGossipConsensus)
	for key,v := range c.data {
		temp := tempGossipConsensus{Consensus:make(map[string]tempGossipStatusAndTimeStamp)}
		temp.PrevConsensus = v.prevConsensus
		temp.PrevEventTime = v.prevEventTime
		temp.Empty = v.empty
		temp.EmptyTime = v.emptyTime
		for nodeAddr,status := range v.consensus {
			temp.Consensus[nodeAddr] = tempGossipStatusAndTimeStamp{status.gossipStatus,
				time.Unix(status.timeStamp/1e9,status.timeStamp%1e9).Format("2006-01-02 15:04:05")}
		}
		result[key] = temp
	}
	dataBytes, err := json.Marshal(result)
	return dataBytes, err
}

func (c *gossipCacheManager) GetKey(key string) (tempGossipConsensus, error) {
	c.RLock()
	defer c.RUnlock()

	temp := tempGossipConsensus{Consensus:make(map[string]tempGossipStatusAndTimeStamp)}
	v,ok := c.data[key];if ok {
		temp.PrevConsensus = v.prevConsensus
		temp.PrevEventTime = v.prevEventTime
		temp.Empty = v.empty
		temp.EmptyTime = v.emptyTime
		for nodeAddr,status := range v.consensus {
			temp.Consensus[nodeAddr] = tempGossipStatusAndTimeStamp{status.gossipStatus,
				time.Unix(status.timeStamp/1e9,status.timeStamp%1e9).Format("2006-01-02 15:04:05")}
		}
		return temp,nil
	}
	return temp,errors.New(fmt.Sprintf("key %s not found",key))
}