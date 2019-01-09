package gossiper

import (
	"github.com/mvidigueira/Peerster/dht"
	"github.com/mvidigueira/Peerster/dht_util"
)

const bucketSize = 20

type bucket struct {
	Nodes    []*dht.NodeState
	Gossiper *Gossiper
}

func newBucket(g *Gossiper) *bucket {
	return &bucket{Nodes: make([]*dht.NodeState, 0), Gossiper: g}
}

func (b *bucket) updateNode(ns *dht.NodeState) (changed bool) {
	for i, node := range b.Nodes {
		if node.NodeID == ns.NodeID {
			temp := append([]*dht.NodeState{node}, b.Nodes[:i]...)
			copy(b.Nodes[:i+1], temp)
			return
		}
	}

	if len(b.Nodes) >= bucketSize {
		lastNode := b.Nodes[bucketSize-1]
		if ok := b.Gossiper.sendPing(lastNode); ok {
			b.Nodes = append([]*dht.NodeState{lastNode}, b.Nodes[:bucketSize-1]...)
			return
		}
		b.Nodes = b.Nodes[:bucketSize-1] // drop unresponsive node
	}

	b.Nodes = append([]*dht.NodeState{ns}, b.Nodes...)
	return true
}

type bucketTable struct {
	MyNodeID dht_util.TypeID
	Buckets  [dht_util.IDByteSize * 8]*bucket
}

func newBucketTable(myNodeID dht_util.TypeID, g *Gossiper) *bucketTable {
	buckets := [dht_util.IDByteSize * 8]*bucket{}
	for i := range buckets {
		buckets[i] = newBucket(g)
	}
	return &bucketTable{MyNodeID: myNodeID, Buckets: buckets}
}

func (bt *bucketTable) updateNode(ns *dht.NodeState) (changed bool) {
	clb := dht.CommonLeadingBits(bt.MyNodeID, ns.NodeID)
	if clb < dht_util.IDByteSize*8 {
		return bt.Buckets[clb].updateNode(ns)
	}
	return false
}

func (bt *bucketTable) alphaClosest(target dht_util.TypeID, alpha int) (results []*dht.NodeState) {
	results = make([]*dht.NodeState, 0)
	for _, bucket := range bt.Buckets {
		for _, node := range bucket.Nodes {
			if len(results) < alpha {
				results = append(results, node)
			} else {
				results = dht.InsertOrdered(target, results, node)
				if len(results) > alpha {
					results = results[:alpha]
				}
			}
		}
	}
	return
}
