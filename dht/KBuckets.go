package dht

const idByteSize = 160 / 8
const bucketSize = 20

type nodeState struct {
	NodeID  [idByteSize]byte
	Address string // IP address with the form "host:port"
}

func ping(Address string) bool {
	return true
}

type bucket struct {
	Nodes []*nodeState
}

func (b *bucket) updateNode(ns *nodeState) {
	for i, node := range b.Nodes {
		if node.NodeID == ns.NodeID {
			temp := append([]*nodeState{node}, b.Nodes[:i]...)
			b.Nodes = append(temp, b.Nodes[i+1:]...)
			return
		}
	}

	if len(b.Nodes) >= bucketSize {
		if ok := ping(b.Nodes[bucketSize-1].Address); ok {
			return
		}
		b.Nodes = b.Nodes[:bucketSize-1]
	}

	b.Nodes = append([]*nodeState{ns}, b.Nodes...)
}

type bucketTable struct {
	MyNode *nodeState
}
