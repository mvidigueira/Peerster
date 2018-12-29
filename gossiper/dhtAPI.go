package gossiper

import "github.com/mvidigueira/Peerster/dht"

// LookupNodes looks for the closest nodes to a specific id (up to k nodes).
// This id can represent a nodeID or a key hash.
// Unlike LookupValue, it does not stop early if it encounters a node with the key stored.
func (g *Gossiper) LookupNodes(id [dht.IDByteSize]byte) (closest []*dht.NodeState) {
	//results := g.bucketTable.alphaClosest(id, 3)
	//g.sendLookupNode()
	return
}
