package gossiper

import (
	"reflect"
	"time"

	"github.com/mvidigueira/Peerster/dht"
)

// LookupNodes looks for the closest nodes to a specific id (up to k nodes).
// This id can represent a nodeID or a key hash.
// Unlike LookupValue, it does not stop early if it encounters a node with the key stored.
func (g *Gossiper) LookupNodes(id [dht.IDByteSize]byte) (closest []*dht.NodeState) {
	snsa := dht.NewSafeNodeStateArray(id)
	results := g.bucketTable.alphaClosest(id, 1)
	node := results[0]
	snsa.Insert(node)

	return g.lookupNodesAux(id, snsa)
}

// lookupNodesAux - simple version that assumes that nodes don't crash
func (g *Gossiper) lookupNodesAux(id [dht.IDByteSize]byte, snsa *dht.SafeNodeStateArray) (closest []*dht.NodeState) {
	results := snsa.GetAlphaUnqueried(1)
	node := results[0]

	c := g.sendLookupNode(node, id)
	snsa.SetQueried(node)

	msg := <-c
	closer := false
	for _, node := range msg.NodeReply.NodeStates {
		closer = closer || snsa.Insert(node)
	}

	if closer {
		return g.lookupNodesAux2(id, snsa)
	}

	closest = snsa.GetArrayCopy()
	return closest[:1]
}

// FUNCTION GRAVEYARD - BE WARNED

// LookupNodes2 looks for the closest nodes to a specific id (up to k nodes).
// This id can represent a nodeID or a key hash.
// Unlike LookupValue, it does not stop early if it encounters a node with the key stored.
func (g *Gossiper) LookupNodes2(id [dht.IDByteSize]byte) (closest []*dht.NodeState) {
	snsa := dht.NewSafeNodeStateArray(id)
	results := g.bucketTable.alphaClosest(id, 1)
	node := results[0]
	snsa.Insert(node)

	return g.lookupNodesAux2(id, snsa)
}

// lookupNodesAux2 implementation is evil incarnate, and it's in its "simple" form
// I might change this to be much simpler, without replication (only returns the closest node)
func (g *Gossiper) lookupNodesAux2(id [dht.IDByteSize]byte, snsa *dht.SafeNodeStateArray) (closest []*dht.NodeState) {
	results := snsa.GetAlphaUnqueried(1)
	node := results[0]

	c := g.sendLookupNode(node, id)
	snsa.SetQueried(node)

	msg := <-c
	closer := false
	for _, node := range msg.NodeReply.NodeStates {
		closer = closer || snsa.Insert(node)
	}

	if closer {
		return g.lookupNodesAux2(id, snsa)
	}

	//BEWARE - UGLIEST PIECE OF CODE I HAVE EVER WRITTEN

	t := time.NewTicker(1 * time.Second)
	cases := make([]reflect.SelectCase, 1)
	cases[0] = reflect.SelectCase{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(t.C)}
	queried := make([]*dht.NodeState, 0)
	chans := make([]chan *dht.Message, 0)

	unqueried, closest := snsa.GetKClosestUnqueried(bucketSize)
	for len(unqueried) == 0 {

		for _, node := range unqueried {
			ch := g.sendLookupNode(node, id)
			snsa.SetQueried(node)
			queried = append(queried, node)
			chans = append(chans, ch)
			cases = append(cases, reflect.SelectCase{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(ch)})
		}

		for {
			chosen, _, _ := reflect.Select(cases)
			if chosen == 0 {
				unqueried, closest = snsa.GetKClosestUnqueried(bucketSize)
				break
			} else {
				snsa.SetResponded(queried[chosen-1])
				if resp, ok := snsa.GetKClosestResponded(bucketSize); ok {
					return resp
				}
			}
		}
	}

	return
}
