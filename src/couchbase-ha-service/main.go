package main

import (
	"context"
	nimo "github.com/gugemichael/nimo4go"
	"gopkg.in/natefinch/lumberjack.v2"
	"log"
	"time"
)

func main() {
	// 解析输入参数
	opts := NewOptions()
	ctx, cancel := context.WithCancel(context.Background())

	commonLogOutput := &lumberjack.Logger{
		Filename:   opts.dataDir + "/ha.log",
		MaxSize:    100, //MB
		MaxBackups: 10,
		MaxAge:     0,
	}

	veri := NewCommonVerifier(opts,commonLogOutput)
	veri.context = ctx

	gnode,err := NewGossipNode(opts.gossipTCPAddress,opts.gossipJoinAddress,opts,commonLogOutput)
	if err != nil {
		panic(err)
	}

	gcm := NewGossipCacheManager(gnode)
	st := NewStCachedNode(opts,gcm,veri,commonLogOutput,gnode)
	nimo.GoRoutine(st.monitorLeadrship)
	GoRoutineInSleep(time.Second*10,st.reJoinGossipNodes)

	// verify
	bri := &bridge{st,veri,gcm,gnode,
		make(chan verifyInput),make(chan verifyOutput),
		ctx,0,
		log.New(commonLogOutput, "bridge: ", log.Ldate|log.Ltime),}
	veri.inputCh = bri.inputCh
	veri.outputCh = bri.outputCh

	GoRoutineInTimeScale(120,st.cm.Len, bri.makeGossip1)
	nimo.GoRoutine(veri.verify)
	nimo.GoRoutine(bri.makeGossip2)

	GoRoutineInSleep(time.Second*10,bri.consumeGossip)

	sc := &scavenger{st,gnode,gcm,veri}
	GoRoutineInSleep(time.Second*10,sc.scavenge)

	notiLogger := log.New(commonLogOutput,"notification : ",log.Ldate|log.Ltime)

	// check,event,notify
	qc := &quorumChecker{opts.gossipQuorum,st,gcm,
		gnode,opts.notificationScript,
		make(chan timeoutShellCmd),make(chan shellCmdResult),
		ctx,notiLogger,opts.eventInterval}
	GoRoutineInSleep(time.Second*10,qc.quorumCheck1)
	nimo.GoRoutine(qc.doTimeoutShellCmd)
	nimo.GoRoutine(qc.quorumCheck2)

	done := make(chan struct{})
	<- done
	cancel()
}
