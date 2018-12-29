package gossiper

import (
	"math/rand"
	"time"

	"github.com/mvidigueira/Peerster/dht"
	"github.com/mvidigueira/Peerster/dto"
)

const pingTimeout = 1

// Ping - pings node 'ns'
func (g *Gossiper) Ping(ns *dht.NodeState) (alive bool) {
	msg := g.newDHTPing()
	rpcNum := msg.Nonce
	packet := &dto.GossipPacket{DHTMessage: msg}
	c, isNew := g.dhtChanMap.AddListener(rpcNum)
	if !isNew {
		panic("Something went very wrong")
	}

	g.sendUDP(packet, ns.Address)

	t := time.NewTicker(pingTimeout * time.Second)
	for {
		select {
		case <-c:
			return true
		case <-t.C:
			g.dhtChanMap.RemoveListener(rpcNum)
			return false
		}
	}
}

// Store - sends a RPC to node 'ns' for storing the KV pair ('key' - 'data')
func (g *Gossiper) store(ns *dht.NodeState, key [dht.IDByteSize]byte, data []byte) (err error) {
	msg := g.newDHTStore(key, data)
	packet := &dto.GossipPacket{DHTMessage: msg}

	g.sendUDP(packet, ns.Address)

	return nil
}

// LookupNode - sends a RPC to node 'ns' for lookup of node with nodeID 'id'
func (g *Gossiper) sendLookupNode(ns *dht.NodeState, id [dht.IDByteSize]byte) (nonce uint64) {
	// WIP
	return //TODO
}

// LookupKey - sends a RPC to node 'ns' for lookup of key 'key'
func (g *Gossiper) sendLookupKey(ns *dht.NodeState, key [dht.IDByteSize]byte) (nonce uint64) {
	return //TODO
}

func (g *Gossiper) newDHTStore(key [dht.IDByteSize]byte, data []byte) *dht.Message {
	store := &dht.Store{Key: key, Data: data}
	return &dht.Message{Nonce: rand.Uint64(), SenderID: g.dhtMyID, Store: store}
}

func (g *Gossiper) newDHTPing() *dht.Message {
	return &dht.Message{Nonce: rand.Uint64(), SenderID: g.dhtMyID, Ping: &dht.Ping{}}
}

func (g *Gossiper) newDHTPingReply(nonce uint64) *dht.Message {
	return &dht.Message{Nonce: nonce, SenderID: g.dhtMyID, PingReply: &dht.PingReply{}}
}

func (g *Gossiper) newDHTNodeLookup(id [dht.IDByteSize]byte) *dht.Message {
	lookup := &dht.NodeLookup{NodeID: id}
	return &dht.Message{Nonce: rand.Uint64(), SenderID: g.dhtMyID, NodeLookup: lookup}
}

func (g *Gossiper) newDHTNodeReply(nodeStates []*dht.NodeState, nonce uint64) *dht.Message {
	reply := &dht.NodeReply{NodeStates: nodeStates}
	return &dht.Message{Nonce: nonce, SenderID: g.dhtMyID, NodeReply: reply}
}

func (g *Gossiper) newDHTValueLookup(key [dht.IDByteSize]byte) *dht.Message {
	lookup := &dht.ValueLookup{Key: key}
	return &dht.Message{Nonce: rand.Uint64(), SenderID: g.dhtMyID, ValueLookup: lookup}
}

func (g *Gossiper) newDHTValueReplyData(data []byte, nonce uint64) *dht.Message {
	reply := &dht.ValueReply{Data: data}
	return &dht.Message{Nonce: nonce, SenderID: g.dhtMyID, ValueReply: reply}
}

func (g *Gossiper) newDHTValueReplyNodes(nodeStates []*dht.NodeState, nonce uint64) *dht.Message {
	reply := &dht.ValueReply{NodeStates: nodeStates}
	return &dht.Message{Nonce: nonce, SenderID: g.dhtMyID, ValueReply: reply}
}
