package gossiper

import (
	"reflect"
	"time"

	"github.com/mvidigueira/Peerster/dht_util"

	"github.com/mvidigueira/Peerster/dht"
)

// LookupNodes looks for the closest nodes to a specific id (up to k nodes).
// This id can represent a nodeID or a key hash.
// Unlike LookupValue, it does not stop early if it encounters a node with the key stored.
func (g *Gossiper) LookupNodes(id dht_util.TypeID) (closest []*dht.NodeState) {
	snsa := dht.NewSafeNodeStateArray(id)
	results := g.bucketTable.alphaClosest(id, 3)
	snsa.InsertArray(results, g.dhtMyID)
	closest = g.lookupRound(snsa)
	myNode := &dht.NodeState{NodeID: g.dhtMyID, Address: g.address}
	closest = dht.InsertOrdered(id, closest, myNode)
	if len(closest) > bucketSize {
		closest = closest[:bucketSize]
	}
	return
}

func (g *Gossiper) lookupFinalPhase(snsa *dht.SafeNodeStateArray) (closest []*dht.NodeState) {
	unqueried, closest := snsa.GetKClosestUnqueried(bucketSize)

	chans := make([]chan []*dht.NodeState, len(unqueried))
	for i, node := range unqueried {
		chans[i] = make(chan []*dht.NodeState)
		snsa.SetQueried(node)
		go g.lookupSingle(node, snsa, chans[i])
	}

	cases, t := makeSelectCases(chans, 1)
	defer t.Stop()
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
	cases, t := makeSelectCases(chans, 1)
	t.Stop()
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

func makeSelectCases(chans []chan []*dht.NodeState, timeoutSec int) (cases []reflect.SelectCase, t *time.Ticker) {
	cases = make([]reflect.SelectCase, len(chans))
	for i, ch := range chans {
		cases[i] = reflect.SelectCase{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(ch)}
	}
	t = time.NewTicker(time.Duration(timeoutSec) * time.Second)
	cases = append(cases, reflect.SelectCase{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(t.C)})
	return
}

// LookupValue looks for the closest nodes to a specific id (up to k nodes).
// This id can represent a nodeID or a key hash.
// Unlike LookupValue, it does not stop early if it encounters a node with the key stored.
func (g *Gossiper) LookupValue(id dht_util.TypeID, dbBucket string) (data []byte, found bool) {
	if data, found = g.dhtDb.Retrieve(id, dbBucket); found {
		return
	}
	snsa := dht.NewSafeNodeStateArray(id)
	results := g.bucketTable.alphaClosest(id, 3)
	snsa.InsertArray(results, g.dhtMyID)

	return g.lookupRoundValue(snsa, dbBucket)
}

func (g *Gossiper) lookupFinalPhaseValue(snsa *dht.SafeNodeStateArray, dbBucket string) (data []byte, found bool) {
	unqueried, _ := snsa.GetKClosestUnqueried(bucketSize)

	chans := make([]chan *dht.Message, len(unqueried))
	for i, node := range unqueried {
		chans[i] = make(chan *dht.Message)
		snsa.SetQueried(node)
		go g.lookupSingleValue(node, snsa, chans[i], dbBucket)
	}

	cases, t := makeSelectCasesValue(chans, 1)
	defer t.Stop()
	closer := false
	answered := 0
	for len(unqueried) != 0 {

		chosen, v, _ := reflect.Select(cases)
		if chosen == (len(cases) - 1) { //timeout
			if closer {
				return g.lookupRoundValue(snsa, dbBucket)
			}
			return g.lookupFinalPhaseValue(snsa, dbBucket)
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
				return g.lookupRoundValue(snsa, dbBucket)
			}
			return g.lookupFinalPhaseValue(snsa, dbBucket)
		}
	}

	return
}

func (g *Gossiper) lookupRoundValue(snsa *dht.SafeNodeStateArray, dbBucket string) (data []byte, found bool) {
	closestNodes := snsa.GetAlphaUnqueried(3)

	chans := make([]chan *dht.Message, len(closestNodes))
	for i, node := range closestNodes {
		chans[i] = make(chan *dht.Message)
		snsa.SetQueried(node)
		go g.lookupSingleValue(node, snsa, chans[i], dbBucket)
	}

	answered := 0
	closer := false
	cases, t := makeSelectCasesValue(chans, 1)
	defer t.Stop()
	for len(closestNodes) != 0 {
		chosen, v, _ := reflect.Select(cases)
		if chosen == (len(cases) - 1) { //timeout
			if closer {
				return g.lookupRoundValue(snsa, dbBucket)
			}
			return g.lookupFinalPhaseValue(snsa, dbBucket)
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
				return g.lookupRoundValue(snsa, dbBucket)
			}
			return g.lookupFinalPhaseValue(snsa, dbBucket)
		}
	}

	return
}

func (g *Gossiper) lookupSingleValue(ns *dht.NodeState, snsa *dht.SafeNodeStateArray, msg chan *dht.Message, dbBucket string) {
	ch := g.sendLookupKey(ns, snsa.Target, dbBucket, g.encryptDHTOperations)
	v := <-ch
	snsa.SetResponded(ns)
	msg <- v
}

func makeSelectCasesValue(chans []chan *dht.Message, timeoutSec int) (cases []reflect.SelectCase, t *time.Ticker) {
	cases = make([]reflect.SelectCase, len(chans))
	for i, ch := range chans {
		cases[i] = reflect.SelectCase{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(ch)}
	}
	t = time.NewTicker(time.Duration(timeoutSec) * time.Second)
	cases = append(cases, reflect.SelectCase{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(t.C)})
	return
}

// StoreInDHT - stores 'data' with key 'key' and type 'storeType' in the DHT
func (g *Gossiper) StoreInDHT(key dht_util.TypeID, data []byte, storeType string) {
	closest := g.LookupNodes(key)
	for _, node := range closest {
		g.sendStore(node, key, data, storeType)
		break //for now only does the first node
	}
}
