package gossiper

import (
	"fmt"
	"github.com/mvidigueira/Peerster/webcrawler"
	"github.com/reiver/go-porterstemmer"
	"go.etcd.io/bbolt"
	"log"
	"math/rand"
	"protobuf"
	"sort"
	"strings"
	"time"

	"github.com/mvidigueira/Peerster/dht"
	"github.com/mvidigueira/Peerster/dto"
)

func (g *Gossiper) newDHTStore(key dht.TypeID, data []byte, storeType string) *dht.Message {
	store := &dht.Store{Key: key, Data: data, Type: storeType}
	return &dht.Message{Nonce: rand.Uint64(), SenderID: g.dhtMyID, Store: store}
}

func (g *Gossiper) newDHTPing() *dht.Message {
	return &dht.Message{Nonce: rand.Uint64(), SenderID: g.dhtMyID, Ping: &dht.Ping{}}
}

func (g *Gossiper) newDHTPingReply(nonce uint64) *dht.Message {
	return &dht.Message{Nonce: nonce, SenderID: g.dhtMyID, PingReply: &dht.PingReply{}}
}

func (g *Gossiper) newDHTNodeLookup(id dht.TypeID) *dht.Message {
	lookup := &dht.NodeLookup{NodeID: id}
	return &dht.Message{Nonce: rand.Uint64(), SenderID: g.dhtMyID, NodeLookup: lookup}
}

func (g *Gossiper) newDHTNodeReply(nodeStates []*dht.NodeState, nonce uint64) *dht.Message {
	reply := &dht.NodeReply{NodeStates: nodeStates}
	return &dht.Message{Nonce: nonce, SenderID: g.dhtMyID, NodeReply: reply}
}

func (g *Gossiper) newDHTValueLookup(key dht.TypeID, dbBucket string) *dht.Message {
	lookup := &dht.ValueLookup{Key: key, DbBucket: dbBucket}
	return &dht.Message{Nonce: rand.Uint64(), SenderID: g.dhtMyID, ValueLookup: lookup}
}

func (g *Gossiper) newDHTValueReplyData(data []byte, nonce uint64) *dht.Message {
	reply := &dht.ValueReply{Data: &data}
	return &dht.Message{Nonce: nonce, SenderID: g.dhtMyID, ValueReply: reply}
}

func (g *Gossiper) newDHTValueReplyNodes(nodeStates []*dht.NodeState, nonce uint64) *dht.Message {
	reply := &dht.ValueReply{NodeStates: &nodeStates}
	return &dht.Message{Nonce: nonce, SenderID: g.dhtMyID, ValueReply: reply}
}

const pingTimeout = 1

// sendPing - pings node 'ns'
func (g *Gossiper) sendPing(ns *dht.NodeState) (alive bool) {
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

// ReplyPing - replies to dht message 'ping', from node with address 'senderAddr'
func (g *Gossiper) replyPing(senderAddr string, ping *dht.Message) {
	msg := g.newDHTPingReply(ping.Nonce)
	packet := &dto.GossipPacket{DHTMessage: msg}
	g.sendUDP(packet, senderAddr)
}

// sendStore - sends a RPC to node 'ns' for storing the KV pair ('key' - 'data')
func (g *Gossiper) sendStore(ns *dht.NodeState, key dht.TypeID, data []byte, storeType string) (err error) {
	msg := g.newDHTStore(key, data, storeType)
	packet := &dto.GossipPacket{DHTMessage: msg}

	g.sendUDP(packet, ns.Address)

	return nil
}

var stores = 0

// replyStore - "replies" to a store rpc (stores the data locally)
func (g *Gossiper) replyStore(msg *dht.Message) {
	storeType := msg.Store.Type
	switch storeType {
	case dht.KeywordsBucket:
		batchTemp := &dht.BatchMessage{}
		protobuf.Decode(msg.Store.Data, batchTemp)
		for _, item := range batchTemp.List {
			stores++
			dat, err := protobuf.Encode(item)
			if err != nil {
				fmt.Printf("Error decode")
				return
			}
			ok := g.dhtDb.AddLinksForKeyword(item.Keyword, dat)
			if !ok {
				fmt.Printf("Failed to add keywords.\n")
			}
		}
	case dht.PageHashBucket:
		g.dhtDb.Db.Update(func(tx *bbolt.Tx) error {
			b := tx.Bucket([]byte(storeType))
			b.Put(msg.Store.Key[:], msg.Store.Data)
			return nil
		})
	case dht.LinksBucket:
		log.Fatal("TODO: links ")

	default:
		fmt.Printf("Unknown store type: %s.", storeType)
	}

}

// LookupNode - sends a RPC to node 'ns' for lookup of node with nodeID 'id'
func (g *Gossiper) sendLookupNode(ns *dht.NodeState, id dht.TypeID) chan *dht.Message {
	msg := g.newDHTNodeLookup(id)
	rpcNum := msg.Nonce
	packet := &dto.GossipPacket{DHTMessage: msg}
	c, isNew := g.dhtChanMap.AddListener(rpcNum)
	if !isNew {
		panic("Something went very wrong")
	}

	//fmt.Printf("Sending node lookup for %x to node %x\n", id, ns.NodeID)

	g.sendUDP(packet, ns.Address)

	return c
}

// ReplyLookupNode - replies to dht message 'lookup', from node with address 'senderAddr'
func (g *Gossiper) replyLookupNode(senderAddr string, lookup *dht.Message) {
	results := g.bucketTable.alphaClosest(lookup.NodeLookup.NodeID, bucketSize)
	msg := g.newDHTNodeReply(results, lookup.Nonce)
	packet := &dto.GossipPacket{DHTMessage: msg}
	g.sendUDP(packet, senderAddr)
}

// LookupKey - sends a RPC to node 'ns' for lookup of key 'key'
func (g *Gossiper) sendLookupKey(ns *dht.NodeState, key dht.TypeID, dbBucket string) chan *dht.Message {
	msg := g.newDHTValueLookup(key, dbBucket)
	rpcNum := msg.Nonce
	packet := &dto.GossipPacket{DHTMessage: msg}
	c, isNew := g.dhtChanMap.AddListener(rpcNum)
	if !isNew {
		panic("Something went very wrong")
	}

	g.sendUDP(packet, ns.Address)

	return c
}

// ReplyLookupKey - replies to dht message 'lookupKey', from node with address 'senderAddr'
func (g *Gossiper) replyLookupKey(senderAddr string, lookupKey *dht.Message) {
	var msg *dht.Message
	if data, ok := g.dhtDb.Retrieve(lookupKey.ValueLookup.Key, lookupKey.ValueLookup.DbBucket); ok {
		msg = g.newDHTValueReplyData(data, lookupKey.Nonce)
	} else {
		results := g.bucketTable.alphaClosest(lookupKey.ValueLookup.Key, bucketSize)
		msg = g.newDHTValueReplyNodes(results, lookupKey.Nonce)
	}
	packet := &dto.GossipPacket{DHTMessage: msg}
	g.sendUDP(packet, senderAddr)
}

//dhtMessageListenRoutine - deals with DHTMessages from other peers
func (g *Gossiper) dhtMessageListenRoutine(cDHTMessage chan *dto.PacketAddressPair) {
	for pap := range cDHTMessage {
		msg := pap.Packet.DHTMessage
		sender := pap.GetSenderAddress()

		ns := &dht.NodeState{NodeID: msg.SenderID, Address: sender}
		if g.bucketTable.updateNode(ns) {
			g.printKnownNodes()
		}

		switch msg.GetUnderlyingType() {
		case dht.PingT:
			fmt.Printf("PING from %x\n", msg.SenderID)
			g.replyPing(sender, msg)
		case dht.NodeLookupT:
			//fmt.Printf("NODE LOOKUP for node %x from %x\n", msg.NodeLookup.NodeID, msg.SenderID)
			g.replyLookupNode(sender, msg)
		case dht.ValueLookupT:
			fmt.Printf("VALUE LOOKUP for key %x from %x\n", msg.ValueLookup.Key, msg.SenderID)
			g.replyLookupKey(sender, msg)
		case dht.PingReplyT:
			fmt.Printf("PING REPLY from %x\n", msg.SenderID)
			g.dhtChanMap.InformListener(msg.Nonce, msg)
		case dht.NodeReplyT:
			//fmt.Printf("NODE REPLY with results %s from %x\n", dht.String(msg.NodeReply.NodeStates), msg.SenderID)
			g.dhtChanMap.InformListener(msg.Nonce, msg)
		case dht.ValueReplyT:
			if msg.ValueReply.Data != nil {
				fmt.Printf("VALUE REPLY with data: %x from %x\n", *msg.ValueReply.Data, msg.SenderID)
			} else {
				fmt.Printf("VALUE REPLY with results %s from %x\n", dht.String(*msg.ValueReply.NodeStates), msg.SenderID)
			}
			g.dhtChanMap.InformListener(msg.Nonce, msg)
		case dht.StoreT:
			//fmt.Printf("STORE REQUEST from %x\n", msg.SenderID)
			g.replyStore(msg)
		}
	}
}

func (g *Gossiper) dhtJoin(bootstrap string) {
	ns := &dht.NodeState{Address: bootstrap}
	fmt.Printf("Attempting to join dht network using %x as bootstrap.\n", bootstrap)
	if !g.sendPing(ns) {
		fmt.Printf("Join failed.\n")
	}
	g.LookupNodes(g.dhtMyID)
	for i := 0; i < dht.IDByteSize*8; i++ {
		id := dht.RandNodeID(g.dhtMyID, i)
		g.LookupNodes(id)
	}
	fmt.Printf("Join complete.\n")
	g.printKnownNodes()
}

func (g *Gossiper) printKnownNodes() {
	nodes := make([]string, 0)
	for _, bucket := range g.bucketTable.Buckets {
		for _, node := range bucket.Nodes {
			nodes = append(nodes, fmt.Sprintf("%x", node.NodeID))
		}
	}
	fmt.Printf("Known DHT nodes: %s\n", strings.Join(nodes, ", "))
}

func (g *Gossiper) lookupWord(token string) (urlMap *dht.KeywordToURLMap){
	word := porterstemmer.StemString(token)
	data, found := g.LookupValue(dht.GenerateKeyHash(word), dht.KeywordsBucket)
	if !found {
		log.Printf("Keyword not found: %s\n", token)
		return
	}
	urlMap = &dht.KeywordToURLMap{}
	err := protobuf.Decode(data, urlMap)
	if  err != nil {
		panic(err)
	}
	return
}

func (g *Gossiper) clientDHTListenRoutine(cCliDHT chan *dto.DHTRequest) {
	for request := range cCliDHT {
		switch {
		case request.Lookup != nil:
			lookup := request.Lookup
			if lookup.Node != nil {
				fmt.Printf("Starting lookup for node: %x\n", lookup.Node)
				closest := g.LookupNodes(*lookup.Node)

				nodes := make([]string, 0)
				for _, node := range closest {
					nodes = append(nodes, fmt.Sprintf("%x", node.NodeID))
				}
				fmt.Printf("Closest DHT nodes found: %s\n", strings.Join(nodes, ", "))
			} else {
				log.Printf("Starting lookup for %s \n", *lookup.Key)

				tokens := strings.Split(*lookup.Key, " ")
				if len(tokens) < 0 {
					continue
				}
				results := webcrawler.NewSearchResults(g.lookupWord(tokens[0]))
				sort.Sort(results)
				for _, t := range tokens[1:] {
					results = webcrawler.Join(results, g.lookupWord(t))
				}
				for _, result := range results {
					log.Printf("RESULT: %s, (%d)\n", result.Link, result.KeywordOccurences)
				}
			}
		case request.Store != nil:
			storeReq := request.Store
			g.StoreInDHT(*storeReq.Key, storeReq.Value, storeReq.Type)
		default:
			fmt.Printf("Unknown dht client message\n")
		}
	}
}
