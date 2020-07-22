# CouchbaseHAService

最初目的是为了Couchabse两地三中心，切动态客户端配置，实际实现与具体服务解耦，不仅可以监控Couchbase，还可以监控其他服务。  
主要基于如下两个开源仓库：
+ hashicorp/raft
+ hashicorp/memberlist 

# Architecture
```
                                        +------------+
                                        |     HTTP   |
                                        +------+------+
                                               |      
                                        +------v------+   
                                        |   FSM DB    |   
                                        +------+------+   
                                               |      
                    +------v------+     +------v------+       
                    |    gossip   |<----| verification|       
                    +------+------+     |    script   |       
                           |            +------+------+   
                           |                   |      
                           |            +------v------+   
                           |----------->|  gossip DB  |  
                                        +------+------+           
                                               |                  
                                               |                  
                    +------v------+            |           +------v---------------+ 
                    |  scanvenger |<-----------|---------->|  notificationscript  | 
                    +------+------+                        +------+---------------+ 
```

主要架构如上图

+ 第一个启动的节点，需要设置bootstrap参数为true，该节点会成为raft集群的leader，并建立一个raft集群和一个gossip集群，参考**stCached.go:NewStCachedNode**，**gossip.go:NewGossipNode**，
随后启动的节点bootstrap参数为false，并需要设置相应的join和gossipJoin参数，从而加入第一个节点的raft集群和gossip集群
+ 通过HTTP端口向节点写入数据（key value对），只有leader会响应写请求，并将key value转化成log entry，然后通过一致性模块把log同步到各个节点，让各个节点的log一致。
参考**http.go:doSet**
+ 节点从log entry中恢复出key value对，并写入到FSM DB，参考**fsm.go:Apply**
+ 启动一个goroutine，每隔一定时间，调用verification script对key value对，进行检测，参考`main.go:nimo.GoRoutineInTimer(time.Second*60,bri.makeGossip)`
+ verification script需要用户自己实现，key value对会作为参数传给script，script输出UP表示服务正常，输出DOWN表示服务异常，参考**verify.sh**
+ 将脚本的输出结果写入到gossip DB，表示对于某个key，该节点检测到的状态为UP或者DOWN，同时将结果通过gossip协议传播到集群其他节点，参考**bridge.go:makeGossip**。
+ 该节点也会通过gossip协议接收到其他节点关于某个key的检测结果，并写入到gossip DB，参考 `main.go:nimo.GoRoutineInTimer(time.Second*1,bri.consumeGossip)`
+ 由于FSM DB中的数据可能被delete，或者可能有节点退出集群，所以定时清理gossip DB中的过期数据，参考`main.go:nimo.GoRoutineInTimer(time.Second*60,sc.scavenge)`
+ 定期对gossip DB中的数据进行检测，如果发现有事件发生（例如某个key之前的状态是UP，现在有至少quorum个节点认为是DOWN，则发生DOWN事件），调用notification script，
参考`main.go:nimo.GoRoutineInTimer(time.Second*60,qc.quorumCheck)`
+ nitification script需要用户自己实现，节点的角色（leader或者follower），key value对，以及key的最新状态，会作为参数传给脚本，参考**notify.sh**

# Example

## local
+ go build .
+ ./run.sh 1
+ ./run.sh 2
+ ./run.sh 3
+ ./run.sh 4
+ ./run.sh 5
+ 写入一条key value对
```
curl "http://127.0.0.1:6000/set?key=ping&value=pong"
```
+ 查看FSM DB中的数据
```
curl "http://127.0.0.1:6000/status" -s -S | python -mjson.tool
```
+ 查看detector中的数据
```
curl "http://127.0.0.1:6000/detectordata" -s -S | python -mjson.tool
```
+ 查看gossip DB中的数据
```
curl "http://127.0.0.1:6000/gossipstatus" -s -S | python -mjson.tool
```

## ansible

+ run（run.hosts中的节点使用IP，不要使用域名）

```
cd ansible
ansible-playbook -i run.hosts run.yml
```

+ 写入3条key value对

```
./write.sh xdcr-test-1:user-follow-test-slave xdcr-test-1:user-follow-test-slave
./write.sh xdcr-test-1:xdcr-test xdcr-test-1:xdcr-test

curl "http://10.41.157.110:6000/set?key=xdcr-test-2:user-follow-test-slave&value=xdcr-test-2:user-follow-test-slave"
curl "http://10.41.157.110:6000/set?key=xdcr-test-1:user-follow-test-slave&value=xdcr-test-1:user-follow-test-slave"
curl "http://10.41.157.110:6000/set?key=xdcr-test-3:user-follow-test-slave&value=xdcr-test-3:user-follow-test-slave"
```

+ 删除1条key

```
./delete.sh xdcr-test-1:xdcr-test
curl "http://10.41.157.110:6000/del?key=xdcr-test-1:user-follow-test-slave"
```

+ 查看FSM DB中的数据

```
./status.sh 0
./status.sh xdcr-test-1:user-follow-test-slave 0
curl "http://10.128.245.96:6000/status" -s -S | python -mjson.tool
```

+ 查看detector中的数据

```
./status.sh 1
./status.sh xdcr-test-1:user-follow-test-slave 1
curl "http://10.128.245.96:6000/detectordata" -s -S | python -mjson.tool
```

+ 查看gossip DB中的数据

```
./status.sh 2
./status.sh xdcr-test-1:user-follow-test-slave 2
curl "http://10.128.245.96:6000/gossipstatus" -s -S | python -mjson.tool
```

+ kill

```
ansible-playbook -i run.hosts kill.yml
```


# Reference

+ [hashicorp/raft](https://github.com/hashicorp/raft)
+ [基于hashicorp/raft的分布式一致性实战教学](https://cloud.tencent.com/developer/article/1183490)
+ [KunTjz/stacache](https://github.com/KunTjz/stcache)
+ [octu0/example-memberlist](https://github.com/octu0/example-memberlist)
+ [hashicorp/memberlist](https://github.com/hashicorp/memberlist)
+ [memberlist使用的SWIM协议](https://www.jianshu.com/p/25dbfeff03f0)
+ [raft协议](https://www.jianshu.com/p/10ea73d45d4b)

# Note

+ `另外fork一个分支，不要在main分支上开发`      
+ `Java2.x（官方版本）异步接口不加超时时间，在集群故障时，访问会block住，切换动态客户端配置之后，流量切不到新的库上。要向业务说明，一定要加超时时间`
+ `Java2.x（官方版本）同步接口默认会加超时时间，在集群挂掉时，会抛java.lang.RuntimeException: java.util.concurrent.TimeoutException: {"b":"bucketName","s":"kv","t":2500000,"i":"0x13ff"}，切换动态客户端配置之后，可以将流量切换到新库上`
