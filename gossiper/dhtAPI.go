package gossiper

import (
	"reflect"
	"time"

	"github.com/mvidigueira/Peerster/dht"
)

// LookupNodes looks for the closest nodes to a specific id (up to k nodes).
// This id can represent a nodeID or a key hash.
// Unlike LookupValue, it does not stop early if it encounters a node with the key stored.
func (g *Gossiper) LookupNodes(id dht.TypeID) (closest []*dht.NodeState) {
	snsa := dht.NewSafeNodeStateArray(id)
	results := g.bucketTable.alphaClosest(id, 3)
	snsa.InsertArray(results, g.dhtMyID)

	return g.lookupRound(snsa)
}

func (g *Gossiper) lookupFinalPhase(snsa *dht.SafeNodeStateArray) (closest []*dht.NodeState) {
	unqueried, closest := snsa.GetKClosestUnqueried(bucketSize)

	chans := make([]chan []*dht.NodeState, len(unqueried))
	for i, node := range unqueried {
		chans[i] = make(chan []*dht.NodeState)
		snsa.SetQueried(node)
		go g.lookupSingle(node, snsa, chans[i])
	}

	cases := makeSelectCases(chans, 1)
	closer := false
	answered := 0
	for len(unqueried) != 0 {

		chosen, v, _ := reflect.Select(cases)
		if chosen == (len(cases) - 1) { //timeout
			if closer {
				return g.lookupRound(snsa)
			}
			return g.lookupFinalPhase(snsa)
		}

		nodes := v.Interface().([]*dht.NodeState)
		closer = snsa.InsertArray(nodes, g.dhtMyID)
		answered++
		if answered == len(unqueried) {
			if closer {
				return g.lookupRound(snsa)
			}
			return g.lookupFinalPhase(snsa)
		}
	}

	return
}

func (g *Gossiper) lookupRound(snsa *dht.SafeNodeStateArray) (closest []*dht.NodeState) {
	closestNodes := snsa.GetAlphaUnqueried(3)

	chans := make([]chan []*dht.NodeState, len(closestNodes))
	for i, node := range closestNodes {
		chans[i] = make(chan []*dht.NodeState)
		snsa.SetQueried(node)
		go g.lookupSingle(node, snsa, chans[i])
	}

	answered := 0
	closer := false
	cases := makeSelectCases(chans, 1)
	for len(closestNodes) != 0 {
		chosen, v, _ := reflect.Select(cases)
		if chosen == (len(cases) - 1) { //timeout
			if closer {
				return g.lookupRound(snsa)
			}
			return g.lookupFinalPhase(snsa)
		}
		nodes := v.Interface().([]*dht.NodeState)
		closer = snsa.InsertArray(nodes, g.dhtMyID)
		answered++
		if answered == len(closestNodes) {
			if closer {
				return g.lookupRound(snsa)
			}
			return g.lookupFinalPhase(snsa)
		}
	}

	return
}

func (g *Gossiper) lookupSingle(ns *dht.NodeState, snsa *dht.SafeNodeStateArray, results chan []*dht.NodeState) {
	ch := g.sendLookupNode(ns, snsa.Target)
	v := <-ch
	snsa.SetResponded(ns)
	results <- v.NodeReply.NodeStates
}

func makeSelectCases(chans []chan []*dht.NodeState, timeoutSec int) (cases []reflect.SelectCase) {
	cases = make([]reflect.SelectCase, len(chans))
	for i, ch := range chans {
		cases[i] = reflect.SelectCase{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(ch)}
	}
	t := time.NewTicker(time.Duration(timeoutSec) * time.Second)
	cases = append(cases, reflect.SelectCase{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(t.C)})
	return
}

//WIP from here on

// LookupValue looks for the closest nodes to a specific id (up to k nodes).
// This id can represent a nodeID or a key hash.
// Unlike LookupValue, it does not stop early if it encounters a node with the key stored.
func (g *Gossiper) LookupValue(id dht.TypeID) (data []byte, found bool) {
	snsa := dht.NewSafeNodeStateArray(id)
	results := g.bucketTable.alphaClosest(id, 3)
	snsa.InsertArray(results, g.dhtMyID)

	return g.lookupRoundValue(snsa)
}

func (g *Gossiper) lookupFinalPhaseValue(snsa *dht.SafeNodeStateArray) (data []byte, found bool) {
	unqueried, _ := snsa.GetKClosestUnqueried(bucketSize)

	chans := make([]chan *dht.Message, len(unqueried))
	for i, node := range unqueried {
		chans[i] = make(chan *dht.Message)
		snsa.SetQueried(node)
		go g.lookupSingleValue(node, snsa, chans[i])
	}

	cases := makeSelectCasesValue(chans, 1)
	closer := false
	answered := 0
	for len(unqueried) != 0 {

		chosen, v, _ := reflect.Select(cases)
		if chosen == (len(cases) - 1) { //timeout
			if closer {
				return g.lookupRoundValue(snsa)
			}
			return g.lookupFinalPhaseValue(snsa)
		}

		msg := v.Interface().(*dht.Message)
		if msg.ValueReply.Data != nil {
			return *msg.ValueReply.Data, true
		}
		nodes := *msg.ValueReply.NodeStates
		closer = snsa.InsertArray(nodes, g.dhtMyID)
		answered++
		if answered == len(unqueried) {
			if closer {
				return g.lookupRoundValue(snsa)
			}
			return g.lookupFinalPhaseValue(snsa)
		}
	}

	return
}

func (g *Gossiper) lookupRoundValue(snsa *dht.SafeNodeStateArray) (data []byte, found bool) {
	closestNodes := snsa.GetAlphaUnqueried(3)

	chans := make([]chan *dht.Message, len(closestNodes))
	for i, node := range closestNodes {
		chans[i] = make(chan *dht.Message)
		snsa.SetQueried(node)
		go g.lookupSingleValue(node, snsa, chans[i])
	}

	answered := 0
	closer := false
	cases := makeSelectCasesValue(chans, 1)
	for len(closestNodes) != 0 {
		chosen, v, _ := reflect.Select(cases)
		if chosen == (len(cases) - 1) { //timeout
			if closer {
				return g.lookupRoundValue(snsa)
			}
			return g.lookupFinalPhaseValue(snsa)
		}

		msg := v.Interface().(*dht.Message)
		if msg.ValueReply.Data != nil {
			return *msg.ValueReply.Data, true
		}
		nodes := *msg.ValueReply.NodeStates

		closer = snsa.InsertArray(nodes, g.dhtMyID)
		answered++
		if answered == len(closestNodes) {
			if closer {
				return g.lookupRoundValue(snsa)
			}
			return g.lookupFinalPhaseValue(snsa)
		}
	}

	return
}

func (g *Gossiper) lookupSingleValue(ns *dht.NodeState, snsa *dht.SafeNodeStateArray, msg chan *dht.Message) {
	ch := g.sendLookupKey(ns, snsa.Target)
	v := <-ch
	snsa.SetResponded(ns)
	msg <- v
}

func makeSelectCasesValue(chans []chan *dht.Message, timeoutSec int) (cases []reflect.SelectCase) {
	cases = make([]reflect.SelectCase, len(chans))
	for i, ch := range chans {
		cases[i] = reflect.SelectCase{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(ch)}
	}
	t := time.NewTicker(time.Duration(timeoutSec) * time.Second)
	cases = append(cases, reflect.SelectCase{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(t.C)})
	return
}

/*
// LookupValue - stops early if it encounters a node with the key stored.
func (g *Gossiper) LookupValue(key dht.TypeID) (data []byte, found bool) {
	if data, found = g.storage.Retrieve(key); found {
		return
	}

	snsa := dht.NewSafeNodeStateArray(key)
	results := g.bucketTable.alphaClosest(key, 1)
	node := results[0]
	snsa.Insert(node)

	return g.lookupValueAux(key, snsa)
}

// lookupNodesAux - simple version that assumes that nodes don't crash
func (g *Gossiper) lookupValueAux(key dht.TypeID, snsa *dht.SafeNodeStateArray) (data []byte, found bool) {
	results := snsa.GetAlphaUnqueried(1)
	node := results[0]

	c := g.sendLookupNode(node, key)
	snsa.SetQueried(node)

	msg := <-c
	closer := false

	if msg.ValueReply.NodeStates == nil {
		return msg.ValueReply.Data, true
	}

	for _, node := range msg.ValueReply.NodeStates {
		closer = closer || snsa.Insert(node)
	}

	if closer {
		return g.lookupValueAux(key, snsa)
	}

	return nil, false
}
*/
