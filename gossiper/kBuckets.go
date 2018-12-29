package gossiper

import "github.com/mvidigueira/Peerster/dht"

const bucketSize = 20

type bucket struct {
	Nodes    []*dht.NodeState
	Gossiper *Gossiper
}

func newBucket(g *Gossiper) *bucket {
	return &bucket{Nodes: make([]*dht.NodeState, 0), Gossiper: g}
}

func (b *bucket) updateNode(ns *dht.NodeState) {
	for i, node := range b.Nodes {
		if node.NodeID == ns.NodeID {
			temp := append([]*dht.NodeState{node}, b.Nodes[:i]...)
			copy(b.Nodes[:i+1], temp)
			return
		}
	}

	if len(b.Nodes) >= bucketSize {
		lastNode := b.Nodes[bucketSize-1]
		if ok := b.Gossiper.Ping(lastNode); ok {
			b.Nodes = append([]*dht.NodeState{lastNode}, b.Nodes[:bucketSize-1]...)
			return
		}
		b.Nodes = b.Nodes[:bucketSize-1] // drop unresponsive node
	}

	b.Nodes = append([]*dht.NodeState{ns}, b.Nodes...)
}

type bucketTable struct {
	MyNodeID [dht.IDByteSize]byte
	Buckets  [dht.IDByteSize * 8]*bucket
}

func newBucketTable(myNodeID [dht.IDByteSize]byte, g *Gossiper) *bucketTable {
	buckets := [dht.IDByteSize * 8]*bucket{}
	for i := range buckets {
		buckets[i] = newBucket(g)
	}
	return &bucketTable{MyNodeID: myNodeID, Buckets: buckets}
}

func (bt *bucketTable) updateNode(ns *dht.NodeState) {
	clb := dht.CommonLeadingBits(bt.MyNodeID, ns.NodeID)
	if clb < dht.IDByteSize*8 {
		bt.Buckets[clb].updateNode(ns)
	}
}

func (bt *bucketTable) alphaClosest(target [dht.IDByteSize]byte, alpha int) (results []*dht.NodeState) {
	results = make([]*dht.NodeState, 0)
	for _, bucket := range bt.Buckets {
		for _, node := range bucket.Nodes {
			if len(results) < alpha {
				results = append(results, node)
			} else {
				results = dht.KeepClosest(target, results, node)
			}
		}
	}
	return
}
