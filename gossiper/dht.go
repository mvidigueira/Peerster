package gossiper

import (
	"fmt"
	"github.com/mvidigueira/Peerster/dht_util"
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

func (g *Gossiper) newDHTStore(key dht_util.TypeID, data []byte, storeType string) *dht.Message {
	store := &dht.Store{Key: key, Data: data, Type: storeType}
	return &dht.Message{Nonce: rand.Uint64(), SenderID: g.dhtMyID, Store: store}
}

func (g *Gossiper) newDHTPing() *dht.Message {
	return &dht.Message{Nonce: rand.Uint64(), SenderID: g.dhtMyID, Ping: &dht.Ping{}}
}

func (g *Gossiper) newDHTPingReply(nonce uint64) *dht.Message {
	return &dht.Message{Nonce: nonce, SenderID: g.dhtMyID, PingReply: &dht.PingReply{}}
}

func (g *Gossiper) newDHTNodeLookup(id dht_util.TypeID) *dht.Message {
	lookup := &dht.NodeLookup{NodeID: id}
	return &dht.Message{Nonce: rand.Uint64(), SenderID: g.dhtMyID, NodeLookup: lookup}
}

func (g *Gossiper) newDHTNodeReply(nodeStates []*dht.NodeState, nonce uint64) *dht.Message {
	reply := &dht.NodeReply{NodeStates: nodeStates}
	return &dht.Message{Nonce: nonce, SenderID: g.dhtMyID, NodeReply: reply}
}

func (g *Gossiper) newDHTValueLookup(key dht_util.TypeID, dbBucket string) *dht.Message {
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
func (g *Gossiper) sendStore(ns *dht.NodeState, key dht_util.TypeID, data []byte, storeType string) (err error) {
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
		batchTemp := &webcrawler.BatchMessage{}
		protobuf.Decode(msg.Store.Data, batchTemp)
		for _, item := range batchTemp.UrlMapList {
			stores++
			ok := g.dhtDb.AddLinksForKeyword(item.Keyword, *item)
			if !ok {
				fmt.Printf("Failed to add keywords.\n")
			}
		}
	case dht.PageHashBucket:
		var err error
		g.dhtDb.Db.Update(func(tx *bbolt.Tx) error {
			b := tx.Bucket([]byte(storeType))
			err = b.Put(msg.Store.Key[:], msg.Store.Data)
			return err
		})
		if err != nil {
			log.Fatal(err)
		}

	case dht.LinksBucket:
		batchTemp := &webcrawler.BatchMessage{}
		err := protobuf.Decode(msg.Store.Data, batchTemp)
		if err != nil {
			panic(err)
		}
		var links []string
		for _, item := range batchTemp.OutBoundLinksPackages {
			stores++
			links = append(links, item.OutBoundLinks...)
		}
		url := batchTemp.OutBoundLinksPackages[0].Url

		idArray := dht_util.GenerateKeyHash(url)
		id := idArray[:]

		outboundPackage := &webcrawler.OutBoundLinksPackage{batchTemp.OutBoundLinksPackages[0].Url, links}
		g.dhtDb.Db.Update(func(tx *bbolt.Tx) error {
			b := tx.Bucket([]byte(dht.LinksBucket))
			data, err := protobuf.Encode(outboundPackage)
			err = b.Put(id, data)
			return err
		})
		if err != nil {
			log.Fatal(err)
		}

		numberOfLinks := len(links)
		rankInfo := &RankInfo{Url: url, Rank: InitialRank, NumberOfOutboundLinks: numberOfLinks}
		relerr := rankInfo.updatePageRank(g)
		g.setRank(outboundPackage, rankInfo, relerr)

	case dht.CitationsBucket:
		batchTemp := &webcrawler.BatchMessage{}
		err := protobuf.Decode(msg.Store.Data, batchTemp)
		if err != nil {
			panic(err)
		}
		g.dhtDb.Db.Update(func(tx *bbolt.Tx) error {
			b := tx.Bucket([]byte(dht.CitationsBucket))

		itemLoop:
			for _, item := range batchTemp.Citations {
				idArray := dht_util.GenerateKeyHash(item.Url)
				id := idArray[:]
				oldCitationsData := b.Get(id)
				oldCitations := &webcrawler.Citations{}
				if oldCitationsData != nil {
					err = protobuf.Decode(oldCitationsData, oldCitations)
					if err != nil {
						return err
					}
					for _, citation := range oldCitations.CitedBy {
						if citation == item.CitedBy[0] {
							continue itemLoop
						}
					}
				} else {
					oldCitations.Url = item.Url
				}
				oldCitations.CitedBy = append(oldCitations.CitedBy, item.CitedBy...)
				data, err := protobuf.Encode(oldCitations)
				err = b.Put(id, data)
				if err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			log.Fatal(err)
		}
	case dht.PageRankBucket:
		update := &RankUpdate{}
		err := protobuf.Decode(msg.Store.Data, update)
		if err != nil {
			panic(err)
		}
		g.receiveRankUpdate(update)

	default:
		fmt.Printf("Unknown store type: %s.", storeType)
	}

}

// LookupNode - sends a RPC to node 'ns' for lookup of node with nodeID 'id'
func (g *Gossiper) sendLookupNode(ns *dht.NodeState, id dht_util.TypeID) chan *dht.Message {
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
func (g *Gossiper) sendLookupKey(ns *dht.NodeState, key dht_util.TypeID, dbBucket string) chan *dht.Message {
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
				//fmt.Printf("VALUE REPLY with data: %x from %x\n", *msg.ValueReply.Data, msg.SenderID)
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
	for i := 0; i < dht_util.IDByteSize*8; i++ {
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

func (g *Gossiper) lookupWord(token string) (urlMap *webcrawler.KeywordToURLMap) {
	word := porterstemmer.StemString(token)
	data, found := g.LookupValue(dht_util.GenerateKeyHash(word), dht.KeywordsBucket)
	if !found {
		log.Printf("Keyword not found: %s\n", token)
		return nil
	}
	urlMap = &webcrawler.KeywordToURLMap{}
	err := protobuf.Decode(data, urlMap)
	if err != nil {
		panic(err)
	}
	return
}

const (
	MaxResults     = 10
	pageRankWeight = 0.3
)

func (g *Gossiper) DoSearch(query string) (rankedResults webcrawler.RankedResults) {
	log.Printf("Starting lookup for %s \n", query)

	tokens := strings.Split(query, " ")
	if len(tokens) < 0 {
		return
	}
	var numberOfDocuments int //TODO: we should improve this estimate

	g.dhtDb.Db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(dht.KeywordsBucket))
		stats := b.Stats()
		numberOfDocuments = stats.KeyN
		return nil
	})

	newResults := g.lookupWord(tokens[0])
	if newResults == nil {
		log.Printf("TERM %s NOT FOUND \n", tokens[0])
		return
	}
	results := webcrawler.NewSearchResults(newResults, numberOfDocuments)
	sort.Sort(results)
	for _, t := range tokens[1:] {
		newResults = g.lookupWord(t)
		if newResults != nil {
			results = webcrawler.Join(results, newResults)
		}
	}
	sort.Sort(results)

	maxResults := len(results.Results)
	if maxResults > MaxResults {
		maxResults = MaxResults
	}
	for _, result := range results.Results[:maxResults] {
		data, found := g.LookupValue(dht_util.GenerateKeyHash(result.Link), dht.PageRankBucket)
		rank := 0.0
		if found {
			pageRank := &RankInfo{}
			err := protobuf.Decode(data, pageRank)
			if err != nil {
				panic(err)
			}
			rank = pageRankWeight * pageRank.Rank //TODO: normalize before weighting
		}
		rank += (1 - pageRankWeight) * result.SimScore
		result := result //make a copy
		rankedResults.Results = append(rankedResults.Results, webcrawler.RankedResult{&result, rank})
	}
	sort.Sort(rankedResults)
	for _, result := range rankedResults.Results {
		log.Printf("RESULT: %s, (%.2f)\n", result.Result.Link, result.Rank)
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
				g.DoSearch(*lookup.Key)
			}
		case request.Store != nil:
			storeReq := request.Store
			g.StoreInDHT(*storeReq.Key, storeReq.Value, storeReq.Type)
		default:
			fmt.Printf("Unknown dht client message\n")
		}
	}
}
