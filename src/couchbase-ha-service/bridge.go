package main

import (
	"context"
	"time"
	"log"
)

type bridge struct {
	st *stCached
	verifier verifier
	gcm *gossipCacheManager
	gnode GossipNode
	inputCh chan verifyInput
	outputCh chan verifyOutput
	context context.Context
	lastTimeStamp int64
	Logger *log.Logger
}

func(b *bridge) makeGossip1() {
	data := b.st.cm.Clone()
	for key,value := range data {
		select {
		case b.inputCh <- verifyInput{key,value}:
		}
	}
}

func(b *bridge) makeGossip2() {
	for {
		select {
		case output := <-b.outputCh:
			node := b.gnode.Addr()
			key := output.key
			value := output.value
			status := output.status
			// better user lamport.go
			// my poor coding
			timeStamp := time.Now().UnixNano()
			timeStamp = Max(timeStamp,b.lastTimeStamp) + 1
			b.lastTimeStamp = timeStamp
			prevConsensus,ok := b.gcm.GetPreviousConsensus(key)
			if ok {
				if (prevConsensus == GOSSIP_DOWN && status == GOSSIP_UP) ||
					(prevConsensus == GOSSIP_UP && status == GOSSIP_DOWN) {
					b.Logger.Printf("may event happened, found %s,%s status from %s to %s",
						key,value,
						prevConsensus,status)
				}
			}
			prevEmpty,ok := b.gcm.GetEmpty(key)
			if ok {
				if (prevEmpty == true &&
					status != GOSSIP_LESS_THAN_MINSAMPLE &&
					status != GOSSIP_NOT_FILL_SAMPLES &&
					status != GOSSIP_EMPTY) ||
					(prevEmpty == false && status == GOSSIP_EMPTY) {
					b.Logger.Printf("may empty changed, found %s,%s empty from %v to %s",
						key,value,
						prevEmpty,status)
				}
			}
			//b.gnode.SetBroadcasts([]MyBroadcastMessage{MyBroadcastMessage{timeStamp,node,key,value,status}})
			b.gcm.Set(key,value,node,timeStamp,status)
		case <- b.context.Done():
			return
		}
	}
}

func(b *bridge) consumeGossip() {
	for _,msg := range b.gnode.GetMessages() {
		b.gcm.Set(msg.Key,msg.Value,msg.Node,msg.TimeStamp,msg.Status)
	}
}

type verifyInput struct {
	key string
	value string
}

type verifyOutput struct {
	key string
	value string
	status gossipStatus
}