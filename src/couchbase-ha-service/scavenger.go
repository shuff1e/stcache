package main

type scavenger struct {
	st *stCached
	node GossipNode
	gcm *gossipCacheManager
	verifier verifier
}

func (sc *scavenger) scavenge() {
	data := sc.st.cm.Clone()
	// NewGossipNode -> conf.Name ->
	// memberlist.Create(conf) -> setAlive ->
	// Node: m.config.Name, -> aliveNode ->
	// Name: a.Node -> m.nodeMap[a.Node] = state
	// -> m.nodes = append(m.nodes, state)
	members := sc.node.Members()
	for key,v := range sc.gcm.Clone() {
		if _,ok := data[key];!ok {
			sc.gcm.DelKey(key)
			continue
		}
		for node,_ := range v.consensus {
			if !Exists(members,node) {
				sc.st.log.Printf("delete %s:%s from gossip members:%v\n", key, node, members)
				sc.gcm.DelNodeAddr(key,node)
			}
		}
	}

	if tempVeri,ok := sc.verifier.(*commonVerifier);ok {
		for key := range tempVeri.detector.Clone() {
			if _,ok := data[key];!ok {
				tempVeri.detector.DelKey(key)
			}
		}
	}
}

func Exists(slice []string,ele string) bool {
	for _,v := range slice {
		if v == ele {
			return true
		}
	}
	return false
}

