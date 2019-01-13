package gossiper

import (
	"errors"
	"fmt"

	"github.com/dedis/protobuf"

	"github.com/mvidigueira/Peerster/dht_util"
	"github.com/mvidigueira/Peerster/webcrawler"
	"github.com/reiver/go-porterstemmer"
	"go.etcd.io/bbolt"

	"log"
	"math/rand"
	"sort"
	"strings"
	"time"

	"github.com/mvidigueira/Peerster/diffie_hellman/aesencryptor"

	"github.com/mvidigueira/Peerster/dht"
	"github.com/mvidigueira/Peerster/dto"
)

func (g *Gossiper) newDHTStore(key dht_util.TypeID, data []byte, storeType string, encrypted bool) *dht.Message {
	store := &dht.Store{Key: key, Data: data, Type: storeType, Encrypted: encrypted}
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

func (g *Gossiper) newDHTValueLookup(key dht_util.TypeID, dbBucket string, encrypt bool) *dht.Message {
	lookup := &dht.ValueLookup{Key: key, DbBucket: dbBucket, Encrypted: encrypt}
	return &dht.Message{Nonce: rand.Uint64(), SenderID: g.dhtMyID, ValueLookup: lookup}
}

func (g *Gossiper) newDHTValueReplyData(data []byte, nonce uint64, encrypt bool) *dht.Message {
	reply := &dht.ValueReply{Data: &data, Encrypted: encrypt}
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
	defer t.Stop()
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
	msg := g.newDHTStore(key, data, storeType, false)
	packet := &dto.GossipPacket{DHTMessage: msg}

	g.sendUDP(packet, ns.Address)

	return nil
}

// sendEncryptedStore - sends a RPC to node 'ns' for storing the KV pair ('key' - 'data')
func (g *Gossiper) sendEncryptedStore(ns *dht.NodeState, key dht_util.TypeID, data []byte, storeType string) (err error) {
	ch := g.getKey(ns.Address, ns.NodeID)
	select {
	case k := <-ch:
		if k == nil {
			fmt.Println("failed to get key, sending unencrypted...")
			g.sendStore(ns, key, data, storeType)
			return
		}
		aesEncrypter := aesencryptor.New(k)
		cipherText := aesEncrypter.Encrypt(data)
		msg := g.newDHTStore(key, cipherText, storeType, true)
		packet := &dto.GossipPacket{DHTMessage: msg}
		g.sendUDP(packet, ns.Address)
	case <-time.After(time.Second * 5):
		return errors.New("Failed to get key")
	}
	return nil
}

var stores = 0

type BatchMessage struct {
	UrlMapList            []*webcrawler.KeywordToURLMap
	OutBoundLinksPackages []*webcrawler.OutBoundLinksPackage
	Citations             []*webcrawler.Citations
	PageRankUpdates       []*RankUpdate
}

// replyStore - "replies" to a store rpc (stores the data locally)
func (g *Gossiper) replyStore(msg *dht.Message) {
	storeType := msg.Store.Type
	switch storeType {
	case dht.KeywordsBucket:
		go func() {
			batchTemp := &BatchMessage{}
			protobuf.Decode(msg.Store.Data, batchTemp)
			ok := g.dhtDb.BulkAddLinksForKeyword(batchTemp.UrlMapList)
			if !ok {
				fmt.Printf("Failed to add keywords.\n")
			}
		}()
	case dht.PageHashBucket:
		go func() {
			var err error
			g.dhtDb.Db.Batch(func(tx *bbolt.Tx) error {
				b := tx.Bucket([]byte(storeType))
				err = b.Put(msg.Store.Key[:], msg.Store.Data)
				return err
			})
			if err != nil {
				log.Fatal(err)
			}
		}()

	case dht.LinksBucket:
		go func() {
			batchTemp := &BatchMessage{}
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
			data, err := protobuf.Encode(outboundPackage)
			g.dhtDb.Db.Batch(func(tx *bbolt.Tx) error {
				b := tx.Bucket([]byte(dht.LinksBucket))
				err = b.Put(id, data)
				return err
			})
			if err != nil {
				log.Fatal(err)
			}

			numberOfLinks := len(links)
			rankInfo := &RankInfo{Url: url, Rank: InitialRank, NumberOfOutboundLinks: numberOfLinks}
			for _, outLink := range links {
				update := &RankUpdate{outLink, rankInfo}
				g.receiveRankUpdate(update, true, true)
			}
		}()

	case dht.CitationsBucket:
		go func() {
			batchTemp := &BatchMessage{}
			err := protobuf.Decode(msg.Store.Data, batchTemp)
			if err != nil {
				panic(err)
			}
			g.dhtDb.Db.Batch(func(tx *bbolt.Tx) error {
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
		}()
	case dht.PageRankBucket:
		go func() {
			batchTemp := &BatchMessage{}
			err := protobuf.Decode(msg.Store.Data, batchTemp)
			if err != nil {
				panic(err)
			}
			for _, update := range batchTemp.PageRankUpdates {
				g.receiveRankUpdate(update, false, false)
			}
		}()
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
func (g *Gossiper) sendLookupKey(ns *dht.NodeState, key dht_util.TypeID, dbBucket string, encrypt bool) chan *dht.Message {
	msg := g.newDHTValueLookup(key, dbBucket, encrypt)
	if g.encryptDHTOperations {
		ch := g.getKey(ns.Address, ns.NodeID)
		select {
		case k := <-ch:
			if k == nil {
				fmt.Println("failed to get key, falling back on default.")
				msg.ValueLookup.Encrypted = false
			} else {
				aesEncrypter := aesencryptor.New(k)
				cipherText := aesEncrypter.Encrypt(msg.ValueLookup.Key[:])
				msg.ValueLookup.EncryptedKey = cipherText
			}
		case <-time.After(time.Second * 5):
			fmt.Println("failed to get key, falling back on default.")
			msg.ValueLookup.Encrypted = false
		}
	}
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
	var key [dht_util.IDByteSize]byte
	if lookupKey.ValueLookup.Encrypted {
		g.activeDiffieHellmanMutex.Lock()
		diffieSessions, f := g.activeDiffieHellmans[lookupKey.SenderID]
		g.activeDiffieHellmanMutex.Unlock()
		if !f {
			fmt.Println("No key found, descarding lookup...")
			return
		}
		encrypted := false
		for i := 0; i < len(diffieSessions); i++ {
			session := diffieSessions[len(diffieSessions)-1]
			aesEncrypter := aesencryptor.New(session.Key)
			data, err := aesEncrypter.Decrypt(lookupKey.ValueLookup.EncryptedKey[:])
			if err == nil {
				encrypted = true
				session.LastTimeUsed = time.Now()
				copy(key[:], data)
				break
			}
		}
		if !encrypted {
			fmt.Println("failed to decrypt lookup message, disgarding..")
		}
	} else {
		key = lookupKey.ValueLookup.Key
	}
	var msg *dht.Message
	if data, ok := g.dhtDb.Retrieve(key, lookupKey.ValueLookup.DbBucket); ok {
		msg = g.newDHTValueReplyData(data, lookupKey.Nonce, lookupKey.ValueLookup.Encrypted)
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
			go func(msg *dht.Message) {
				fmt.Printf("PING from %x\n", msg.SenderID)
				g.replyPing(sender, msg)
			}(msg)
		case dht.NodeLookupT:
			//fmt.Printf("NODE LOOKUP for node %x from %x\n", msg.NodeLookup.NodeID, msg.SenderID)
			go func(msg *dht.Message) {
				g.replyLookupNode(sender, msg)
			}(msg)
		case dht.ValueLookupT:
			go func(msg *dht.Message) {
				fmt.Printf("VALUE LOOKUP %s\n", msg.ValueLookup.DbBucket)
				g.replyLookupKey(sender, msg)
			}(msg)
		case dht.PingReplyT:
			go func(msg *dht.Message) {
				fmt.Printf("PING REPLY from %x\n", msg.SenderID)
				g.dhtChanMap.InformListener(msg.Nonce, msg)
			}(msg)
		case dht.NodeReplyT:
			go func(msg *dht.Message) {
				//fmt.Printf("NODE REPLY with results %s from %x\n", dht.String(msg.NodeReply.NodeStates), msg.SenderID)
				g.dhtChanMap.InformListener(msg.Nonce, msg)
			}(msg)
		case dht.ValueReplyT:
			go func(msg *dht.Message) {
				if msg.ValueReply.Data != nil {
					//fmt.Printf("VALUE REPLY with data: %x from %x\n", *msg.ValueReply.Data, msg.SenderID)
				} else {
					fmt.Println("VALUE REPLY")
				}
				g.dhtChanMap.InformListener(msg.Nonce, msg)
			}(msg)
		case dht.StoreT:
			if msg.Store.Encrypted {
				go func(msg *dht.Message) {
					g.activeDiffieHellmanMutex.Lock()
					diffieSessions, f := g.activeDiffieHellmans[msg.SenderID]
					g.activeDiffieHellmanMutex.Unlock()

					if !f {
						fmt.Println("No key found, discarding...")
						return
					}

					encrypted := false
					for i := 0; i < len(diffieSessions); i++ {
						session := diffieSessions[len(diffieSessions)-1]
						aesEncrypter := aesencryptor.New(session.Key)
						data, err := aesEncrypter.Decrypt(msg.Store.Data)
						if err == nil {
							encrypted = true
							session.LastTimeUsed = time.Now()
							msg.Store.Data = data
							g.replyStore(msg)
							break
						}
					}
					if !encrypted {
						fmt.Println("failed to decrypt message, discarding..")
					}
				}(msg)
			} else {
				go func(msg *dht.Message) {
					//fmt.Printf("STORE REQUEST from %x\n", msg.SenderID)
					g.replyStore(msg)
				}(msg)
			}
		}
	}
}

func (g *Gossiper) dhtJoin(bootstrap string) {
	ns := &dht.NodeState{Address: bootstrap}
	fmt.Printf("Attempting to join dht network using %s as bootstrap.\n", bootstrap)
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
		log.Printf("Keyword not found: %s (stem: %s)\n", token, word)
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

	i := 0
	var newResults *webcrawler.KeywordToURLMap
	for ; i < len(tokens); i++ {
		t := tokens[i]
		if len(t) < webcrawler.MinWordLen {
			continue
		}
		newResults = g.lookupWord(t)
		if newResults == nil {
			log.Printf("TERM %s NOT FOUND \n", t)
		} else {
			break
		}
	}
	if newResults == nil {
		return
	}
	numberOfDocuments := g.GetDocumentEstimate()
	results := webcrawler.NewSearchResults(newResults, numberOfDocuments)
	sort.Sort(results)
	for _, t := range tokens[i:] {
		if len(t) < webcrawler.MinWordLen {
			continue
		}
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

	simScoreMax := 0.0
	pageRankMax := 0.0
	for _, result := range results.Results[:maxResults] {
		data, found := g.LookupValue(dht_util.GenerateKeyHash(result.Link), dht.PageRankBucket)
		pageRankVal := 0.0
		if found {
			pageRank := &RankInfo{}
			err := protobuf.Decode(data, pageRank)
			if err != nil {
				panic(err)
			}
			pageRankVal = pageRank.Rank
		}
		result := result //make a copy
		if result.SimScore > simScoreMax {
			simScoreMax = result.SimScore
		}
		if pageRankVal > pageRankMax {
			pageRankMax = pageRankVal
		}
		rankedResults.Results = append(rankedResults.Results, &webcrawler.RankedResult{&result, pageRankVal, 0})
	}

	for _, rankedResult := range rankedResults.Results {
		if simScoreMax != 0 {
			rankedResult.Result.SimScore = rankedResult.Result.SimScore / simScoreMax
		}
		pageRank := rankedResult.PageRank
		if pageRankMax != 0 {
			pageRank = rankedResult.PageRank / pageRankMax
		}
		rankedResult.Rank = (1-pageRankWeight)*rankedResult.Result.SimScore + pageRankWeight*rankedResult.PageRank*pageRank
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
