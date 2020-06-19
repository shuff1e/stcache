# raft协议部分
+ `st := NewStCachedNode(opts,gcm)`启动一个raft节点
如果设置了bootstrap参数，该节点作为raft集群的master启动
```
	if opts.bootstrap {
		configuration := raft.Configuration{
			Servers: []raft.Server{
				{
					ID:      raftConfig.LocalID,
					Address: transport.LocalAddr(),
				},
			},
		}
		raftNode.BootstrapCluster(configuration)
	}
```

+ `httpServer := NewHttpServer(ctx, logger)`在raft节点上启动一个http server

主要  
1. 响应set请求，写数据到raft集群，只有leader会真正响应set请求
2. 响应join请求，只有leader才能响应join请求，
`addPeerFuture := h.ctx.st.raft.raft.AddVoter(raft.ServerID(peerAddress), raft.ServerAddress(peerAddress), 0, 0)`
将请求的节点加入raft集群

+ 响应set请求就是`applyFuture := h.ctx.st.raft.raft.Apply(eventBytes, 5*time.Second)`，
生成一条日志，leader将日志同步到其他follower，通过日志状态机来实现数据一致性
```
// Apply is used to apply a command to the FSM in a highly consistent
// manner. This returns a future that can be used to wait on the application.
// An optional timeout can be provided to limit the amount of time we wait
// for the command to be started. This must be run on the leader or it
// will fail.
func (r *Raft) Apply(cmd []byte, timeout time.Duration) ApplyFuture {
	metrics.IncrCounter([]string{"raft", "apply"}, 1)
	r.logger.Printf("[INFO] raft: %v apply once (Leader: %q)", r, r.Leader())
	var timer <-chan time.Time
	if timeout > 0 {
		timer = time.After(timeout)
	}

	// Create a log future, no index or term yet
	logFuture := &logFuture{
		log: Log{
			Type: LogCommand,
			Data: cmd,
		},
	}
	logFuture.init()

	select {
	case <-timer:
		return errorFuture{ErrEnqueueTimeout}
	case <-r.shutdownCh:
		return errorFuture{ErrRaftShutdown}
	case r.applyCh <- logFuture:
		return logFuture
	}
}
```

+ 然后follower调用`func (f *FSM) Apply(logEntry *raft.Log) interface{}`方法，从日志中恢复出数据
```
// Apply applies a Raft log entry to the key-value store.
func (f *FSM) Apply(logEntry *raft.Log) interface{} {
	e := logEntryData{}
	if err := json.Unmarshal(logEntry.Data, &e); err != nil {
		panic("Failed unmarshaling Raft log entry. This is a bug.")
	}
	if e.Value == "" {
		ret := f.ctx.st.cm.Del(e.Key)
		f.log.Printf("fsm.Apply(), logEntry:%s, ret:%v\n", logEntry.Data, ret)
		return ret
	}
	ret := f.ctx.st.cm.Set(e.Key, e.Value)
	f.log.Printf("fsm.Apply(), logEntry:%s, ret:%v\n", logEntry.Data, ret)
	return ret
}
```
# gossip协议部分
+ `gnode,err := NewGossipNode(opts.gossipTCPAddress,opts.gossipJoinAddress,opts)`启动一个gossip集群
如果设置了`cluster`参数，加入cluster这个集群

+ `d := new(MyDelegate)`这个`Delegate`接口可以将你自己的消息，搭载到心跳包上，发送到集群其他节点
```
// Delegate is the interface that clients must implement if they want to hook
// into the gossip layer of Memberlist. All the methods must be thread-safe,
// as they can and generally will be called concurrently.
type Delegate interface {
	// NodeMeta is used to retrieve meta-data about the current node
	// when broadcasting an alive message. It's length is limited to
	// the given byte size. This metadata is available in the Node structure.
	NodeMeta(limit int) []byte

	// NotifyMsg is called when a user-data message is received.
	// Care should be taken that this method does not block, since doing
	// so would block the entire UDP packet receive loop. Additionally, the byte
	// slice may be modified after the call returns, so it should be copied if needed
	NotifyMsg([]byte)

	// GetBroadcasts is called when user data messages can be broadcast.
	// It can return a list of buffers to send. Each buffer should assume an
	// overhead as provided with a limit on the total byte size allowed.
	// The total byte size of the resulting data to send must not exceed
	// the limit. Care should be taken that this method does not block,
	// since doing so would block the entire UDP packet receive loop.
	GetBroadcasts(overhead, limit int) [][]byte

	// LocalState is used for a TCP Push/Pull. This is sent to
	// the remote side in addition to the membership information. Any
	// data can be sent here. See MergeRemoteState as well. The `join`
	// boolean indicates this is for a join instead of a push/pull.
	LocalState(join bool) []byte

	// MergeRemoteState is invoked after a TCP Push/Pull. This is the
	// state received from the remote side and is the result of the
	// remote side's LocalState call. The 'join'
	// boolean indicates this is for a join instead of a push/pull.
	MergeRemoteState(buf []byte, join bool)
}
```
主要参考`GetBroadcasts`的实现，将你自己的消息入队，发送心跳包的时候会调用`GetBroadcasts`方法，将消息搭载到心跳包上
```
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
```

+ `Delegate`接口也可以让你获取其他节点通过gossip发送过来的消息，参考`NotifyMsg`的实现
```
func (m *MyDelegate) NotifyMsg(msg []byte) {
	m.mu.Lock()
	defer m.mu.Unlock()

	cp := make([]byte, len(msg))
	copy(cp, msg)
	if v,ok := ParseMyBroadcastMessage(cp);ok {
		m.msgs = append(m.msgs, *v)
	}
}
```
+ `e := new(MyEventDelegate)`这个`EventDelegate`接口会通知节点join，leave等信息