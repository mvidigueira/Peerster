package gossiper

import (
	"bytes"
	"fmt"
	"log"
	"time"

	"github.com/dedis/protobuf"

	"github.com/mvidigueira/Peerster/dht_util"
	"github.com/mvidigueira/Peerster/diffie_hellman/aesencryptor"

	"github.com/mvidigueira/Peerster/dht"
	"github.com/mvidigueira/Peerster/dto"
	"github.com/mvidigueira/Peerster/webcrawler"
)

func (g *Gossiper) startCrawler() {
	queue, ok := g.dhtDb.CrawlQueueHeadPointer()
	if !ok {
		log.Fatal("Queue index not found.")
	}
	if queue == 0 {
		g.initiateFreshCrawl()
		return
	}
	g.initiateAndRestoreCrawl()
}

func (g *Gossiper) initiateFreshCrawl() {
	// Fresh start of crawl, initiate queue from start point if leader.
	g.webCrawler.Start(map[string]bool{})
	if g.webCrawler.Leader {
		err := g.dhtDb.UpdateCrawlQueue([]byte("/wiki/Outline_of_academic_disciplines"))
		if err != nil {
			log.Fatal("Error saving root url.")
		}
		g.webCrawler.OutChan <- &webcrawler.CrawlerPacket{
			Done: &webcrawler.DoneCrawl{Delete: false},
		}
	}
}

func (g *Gossiper) initiateAndRestoreCrawl() {
	// Restore bloom filter
	past := g.dhtDb.GetPast()

	g.webCrawler.Start(past)

	g.webCrawler.OutChan <- &webcrawler.CrawlerPacket{
		Done: &webcrawler.DoneCrawl{Delete: false},
	}
}

//webCrawlerListenRouting - deals with indexing of webpages and hyperlink distribution
func (g *Gossiper) webCrawlerListenerRoutine() {
	for packet := range g.webCrawler.OutChan {
		switch {
		case packet.OutBoundLinks != nil:
			go func(outboundLinks *webcrawler.OutBoundLinksPackage) {
				g.saveOutboundLinksInDHT(outboundLinks)
			}(packet.OutBoundLinks)
		case packet.CitationsPackage != nil:
			go func(citation *webcrawler.CitationsPackage) {
				g.saveCitationsInDHT(citation)
			}(packet.CitationsPackage)
		case packet.HyperlinkPackage != nil:
			go func(hyperlinks *webcrawler.HyperlinkPackage) {
				g.distributeHyperlinks(hyperlinks)
			}(packet.HyperlinkPackage)
		case packet.IndexPackage != nil:
			go func(indexPackage *webcrawler.IndexPackage) {
				g.saveKeywordsInDHT(indexPackage)
			}(packet.IndexPackage)
		case packet.PageHash != nil && packet.PageHash.Type == "store":
			go func(pageHash *webcrawler.PageHashPackage) {
				g.savePageHashInDHT(pageHash)
			}(packet.PageHash)
		case packet.PageHash != nil && packet.PageHash.Type == "lookup":
			go func(pageHash *webcrawler.PageHashPackage) {
				_, found := g.LookupValue(pageHash.Hash, dht.PageHashBucket)
				packet.ResChan <- found
			}(packet.PageHash)
		case packet.PastMapPackage != nil:
			g.dhtDb.SavePast(packet.PastMapPackage.Content)
		case packet.Done != nil:
			go func(done *webcrawler.DoneCrawl) {
				// Crawl of page done, get new url from db and feed it to crawler.
				if done.Delete {
					err := g.dhtDb.DeleteCrawlQueueHead()
					if err != nil {
						fmt.Println("failed to delete head.")
					}
				}
				head, err := g.dhtDb.CrawlQueueHead()
				if err != nil {
					time.Sleep(time.Second * 2)
					g.webCrawler.OutChan <- &webcrawler.CrawlerPacket{
						Done: done,
					}
					return
				}
				g.webCrawler.NextCrawl <- head
			}(packet.Done)
		}
	}
}

// The maximum safe size of a UDP packet is 8192 but it could happen that some pages contains a set of urls
// which is larger than 8192 bytes and hence we need to break down the Url packages into batches which are smaller than 8192bytes.
func (g *Gossiper) createUDPBatches(owner *dht.NodeState, items []interface{}) []interface{} {
	udpMaxSize := 7500
	packet := make([]interface{}, 0, udpMaxSize)
	packetSize := 0
	batches := make([]interface{}, 0, 1)
	for _, item := range items {
		var itemLength int
		switch v := item.(type) {
		case string:
			itemLength = len(v)
		default:
			bytes, _ := protobuf.Encode(item)
			itemLength = len(bytes)
		}
		if packetSize+itemLength < udpMaxSize {
			packetSize += itemLength
			packet = append(packet, item)
		} else {
			batches = append(batches, packet)
			packetSize = 0
			packet = make([]interface{}, 0, udpMaxSize)
		}
	}
	if len(packet) > 0 {
		batches = append(batches, packet)
	}
	return batches
}

func (g *Gossiper) batchSendURLS(owner *dht.NodeState, hyperlinks []string) {
	s := make([]interface{}, len(hyperlinks))
	for i, v := range hyperlinks {
		s[i] = v
	}
	batches := g.createUDPBatches(owner, s)
	for _, batch := range batches {
		tmp := make([]string, len(batch.([]interface{})))
		for k, b := range batch.([]interface{}) {
			tmp[k] = b.(string)
		}
		var gossiperPacket *dto.GossipPacket
		if g.encryptDHTOperations {
			ch := g.getKey(owner.Address, owner.NodeID)
			key := <-ch
			if key == nil {
				return
			}
			if len(key) == 0 {
				log.Fatal("Fetching of key failed.")
			}
			links := &webcrawler.HyperlinkPackage{
				Links: tmp,
			}
			packetBytes, err := protobuf.Encode(links)
			if err != nil {
				panic(err)
			}
			aesEncrypter := aesencryptor.New(key)
			cipherText := aesEncrypter.Encrypt(packetBytes)

			gossiperPacket = &dto.GossipPacket{
				EncryptedWebCrawlerPacket: &webcrawler.EncryptedCrawlerPacket{
					Packet: cipherText,
					Origin: g.dhtMyID,
				},
			}
		} else {
			gossiperPacket = &dto.GossipPacket{
				HyperlinkMessage: &webcrawler.HyperlinkPackage{
					Links: tmp,
				},
			}
		}

		g.sendUDP(gossiperPacket, owner.Address)
	}
}

func (g *Gossiper) batchSendKeywords(owner *dht.NodeState, items []*webcrawler.KeywordToURLMap) {
	s := make([]interface{}, len(items))
	for i, v := range items {
		s[i] = v
	}
	batches := g.createUDPBatches(owner, s)
	for _, batch := range batches {
		tmp := make([]*webcrawler.KeywordToURLMap, len(batch.([]interface{})))
		for k, b := range batch.([]interface{}) {
			tmp[k] = b.(*webcrawler.KeywordToURLMap)
		}
		packetBytes, err := protobuf.Encode(&BatchMessage{UrlMapList: tmp})
		if err != nil {
			fmt.Println(err)
			fmt.Printf("Error encoding urlToKeywordMap.\n")
			break
		}

		if g.encryptDHTOperations {
			err = g.sendEncryptedStore(owner, [dht_util.IDByteSize]byte{}, packetBytes, dht.KeywordsBucket)
		} else {
			err = g.sendStore(owner, [dht_util.IDByteSize]byte{}, packetBytes, dht.KeywordsBucket)
		}
		if err != nil {
			fmt.Printf("Failed to store key.\n")
		}
	}
}

func (g *Gossiper) batchSendCitations(owner *dht.NodeState, items []*webcrawler.Citations) {
	s := make([]interface{}, len(items))
	for i, v := range items {
		s[i] = v
	}
	batches := g.createUDPBatches(owner, s)
	for _, batch := range batches {
		tmp := make([]*webcrawler.Citations, len(batch.([]interface{})))
		for k, b := range batch.([]interface{}) {
			tmp[k] = b.(*webcrawler.Citations)
		}
		packetBytes, err := protobuf.Encode(&BatchMessage{Citations: tmp})
		if err != nil {
			panic(err)
		}
		if g.encryptDHTOperations {
			err = g.sendEncryptedStore(owner, [dht_util.IDByteSize]byte{}, packetBytes, dht.CitationsBucket)
		} else {
			err = g.sendStore(owner, [dht_util.IDByteSize]byte{}, packetBytes, dht.CitationsBucket)
		}
		if err != nil {
			fmt.Printf("Failed to store key.\n")
		}
	}
}

// Identifies the responsible node for each hyperlink. If the hyperlink belongs to this node, then the hyperlinks are simply sent back
// to the local webcrawler. If the hyperlinks belong to another webcrawlers domain, then they will be sent to the corresponding node.
func (g *Gossiper) distributeHyperlinks(packet *webcrawler.HyperlinkPackage) {
	links := packet.Links
	domains := map[dht.NodeState][]string{}
	for _, hyperlink := range links {
		hash := dht_util.GenerateKeyHash(hyperlink)
		closestNodes := g.LookupNodes(hash)
		if len(closestNodes) == 0 {
			fmt.Println("LookupNodes returned an empty list... Retrying in 5 seconds.")
			// Retry in 5 seconds.
			go func() {
				time.Sleep(time.Second * 5)
				g.distributeHyperlinks(packet)
			}()
			break
		}
		owner := closestNodes[0]
		hyperlinks, found := domains[*owner]
		if !found {
			domains[*owner] = []string{hyperlink}
		} else {
			hyperlinks = append(hyperlinks, hyperlink)
			domains[*owner] = hyperlinks
		}
	}

	for owner, hyperlinks := range domains {
		if owner.Address == g.address {
			// Send back the urls belonging to this nodes domain
			for _, link := range hyperlinks {
				err := g.dhtDb.UpdateCrawlQueue([]byte(link))
				if err != nil {
					log.Fatal("Error saving link")
				}
			}
		} else {
			// Url belong to another crawlers domain.
			g.batchSendURLS(&owner, hyperlinks)
		}
	}
}

func (g *Gossiper) saveOutboundLinksInDHT(outboundLinks *webcrawler.OutBoundLinksPackage) {
	id := dht_util.GenerateKeyHash(outboundLinks.Url)
	kClosest := g.LookupNodes(id)
	if len(kClosest) == 0 {
		fmt.Printf("Could not perform store since no neighbours found.\n")
		return
	}
	closest := kClosest[0]

	s := make([]interface{}, len(outboundLinks.OutBoundLinks))
	for i, v := range outboundLinks.OutBoundLinks {
		outBoundLink := &webcrawler.OutBoundLinksPackage{
			Url:           outboundLinks.Url,
			OutBoundLinks: []string{v},
		}
		s[i] = outBoundLink
	}
	batches := g.createUDPBatches(closest, s)
	for _, batch := range batches {
		tmp := make([]*webcrawler.OutBoundLinksPackage, len(batch.([]interface{})))
		for k, b := range batch.([]interface{}) {
			tmp[k] = b.(*webcrawler.OutBoundLinksPackage)
		}
		packetBytes, err := protobuf.Encode(&BatchMessage{OutBoundLinksPackages: tmp})
		if err != nil {
			panic(err)
		}

		if bytes.Equal(closest.NodeID[:], g.dhtMyID[:]) {
			msg := g.newDHTStore(g.dhtMyID, packetBytes, dht.LinksBucket, false)
			g.replyStore(msg)
		} else {
			if g.encryptDHTOperations {
				err = g.sendEncryptedStore(closest, id, packetBytes, dht.LinksBucket)

			} else {
				err = g.sendStore(closest, id, packetBytes, dht.LinksBucket)
			}
			if err != nil {
				fmt.Printf("Failed to store key.\n")
			}
		}
	}
}

func (g *Gossiper) saveCitationsInDHT(citationsPackage *webcrawler.CitationsPackage) {
	destinations := make(map[dht.NodeState][]*webcrawler.Citations)
	fmt.Printf("Size of citation: %d\n", len(citationsPackage.CitationsList))
	for _, citation := range citationsPackage.CitationsList {
		citation := citation //create a copy of the variable
		id := dht_util.GenerateKeyHash(citation.Url)
		kClosest := g.LookupNodes(id)
		if len(kClosest) == 0 {
			fmt.Printf("Could not perform store since no neighbours found.\n")
			break
		}
		closest := kClosest[0]
		val, _ := destinations[*closest]
		destinations[*closest] = append(val, &citation)
	}

	for dest, batch := range destinations {
		if dest.Address == g.address {
			// This node is the destination
			packetBytes, err := protobuf.Encode(&BatchMessage{Citations: batch})
			if err != nil {
				fmt.Println(err)
				return
			}
			msg := g.newDHTStore(dest.NodeID, packetBytes, dht.CitationsBucket, false)
			g.replyStore(msg)
			continue
		}
		g.batchSendCitations(&dest, batch)
	}
}

// Save each keyword of a Url at the responsible node
func (g *Gossiper) saveKeywordsInDHT(indexPackage *webcrawler.IndexPackage) {
	frequencies, url := indexPackage.KeywordFrequencies, indexPackage.Url
	destinations := map[dht.NodeState][]*webcrawler.KeywordToURLMap{}
	for k, v := range frequencies {
		keyHash := dht_util.GenerateKeyHash(k)
		kClosest := g.LookupNodes(keyHash)
		if len(kClosest) == 0 {
			fmt.Printf("Could not perform store since no neighbours found.\n")
			break
		}
		closest := kClosest[0]
		urlToKeywordMap := &webcrawler.KeywordToURLMap{
			Keyword:  k,
			LinkData: map[string]int{url: v},
		}
		val, _ := destinations[*closest]
		destinations[*closest] = append(val, urlToKeywordMap)
	}

	for dest, batch := range destinations {
		if dest.Address == g.address {
			// This node is the destination
			packetBytes, err := protobuf.Encode(&BatchMessage{UrlMapList: batch})
			if err != nil {
				fmt.Println(err)
				return
			}
			msg := g.newDHTStore(dest.NodeID, packetBytes, dht.KeywordsBucket, false)
			g.replyStore(msg)
			continue
		}
		g.batchSendKeywords(&dest, batch)
	}
}

// Saves the hash of a page in the DHT. This is used to prevent duplicate crawling of the same page.
// The reason why we hash the content instead of the Url is because there might be different urls which points to the same page.
func (g *Gossiper) savePageHashInDHT(pageHash *webcrawler.PageHashPackage) {
	kClosest := g.LookupNodes(pageHash.Hash)
	if len(kClosest) == 0 {
		fmt.Printf("Could not perform store since no neighbours found.\n")
		return
	}
	closest := kClosest[0]
	err := g.sendStore(closest, pageHash.Hash, []byte(""), dht.PageHashBucket)
	if err != nil {
		fmt.Printf("Failed to store key %s.\n", pageHash)
	}
}

func (g *Gossiper) getKey(dest string, nodeID [dht_util.IDByteSize]byte) chan []byte {
	resChan := make(chan []byte)
	go func() {
		// Get currently active session if any
		g.activeOutgoingDiffieHellmanMutex.Lock()
		diffieSessions, f := g.activeOutgoingDiffieHellmans[nodeID]
		g.activeOutgoingDiffieHellmanMutex.Unlock()

		// Check if all current sessions are expired
		var allSessionsExpired = true
		var indexNonExpired = -1
		for index, session := range diffieSessions {
			allSessionsExpired = session.expired()
			if !session.expired() {
				indexNonExpired = index
			}
		}

		if f && len(diffieSessions) > 0 && !allSessionsExpired {
			// We have a valid session
			resChan <- diffieSessions[indexNonExpired].Key
		} else {
			// We need to negotiate new session
			g.negotiationMapMutex.Lock()
			// We only want 1 negotiation at a time
			ch, f := g.negotiationMap[nodeID]
			if !f {
				fmt.Println("Negotiation new key.")
				negotiationChannel := make(chan [dht_util.IDByteSize]byte)
				g.negotiationMap[nodeID] = negotiationChannel
				g.negotiationMapMutex.Unlock()

				// Start negotiation
				select {
				case k := <-g.negotiateDiffieHellmanInitiator(dest, nodeID):
					if k == nil {
						fmt.Println("Failed to initiate diffie-hellman. Retrying...")
						resChan <- nil
					}
					fmt.Println("Succeded with new key.")

					resChan <- k
					g.negotiationMapMutex.Lock()
					delete(g.negotiationMap, nodeID)
					g.negotiationMapMutex.Unlock()
				case <-time.After(time.Second * 30):
					g.negotiationMapMutex.Lock()
					delete(g.negotiationMap, nodeID)
					g.negotiationMapMutex.Unlock()
					fmt.Println("Failed to initiate diffie-hellman.2")
					resChan <- nil
				}
			} else {
				// we need a new key but a negotiation is already active -> subscribe to the channel
				g.negotiationMapMutex.Unlock()
				select {
				case newKey := <-ch:
					resChan <- newKey[:]
				case <-time.After(time.Second * 30):
					resChan <- nil
				}
			}
		}
	}()
	return resChan
}

func (g *Gossiper) decryptHyperlinkPackage(sender string, encryptedPacket *webcrawler.EncryptedCrawlerPacket) chan *webcrawler.HyperlinkPackage {
	callBackChan := make(chan *webcrawler.HyperlinkPackage)
	go func() {

		g.activeIngoingDiffieHellmanMutex.Lock()
		diffieSessions, f := g.activeIngoingDiffieHellmans[encryptedPacket.Origin]
		g.activeIngoingDiffieHellmanMutex.Unlock()

		if !f {
			fmt.Println("No key found, discarding...")
			return
		}

		encrypted := false
		for i := 0; i < len(diffieSessions); i++ {
			session := diffieSessions[len(diffieSessions)-1]
			aesEncrypter := aesencryptor.New(session.Key)
			data, err := aesEncrypter.Decrypt(encryptedPacket.Packet)
			if err == nil {
				encrypted = true
				session.LastTimeUsed = time.Now()
				hyperlinkPackage := &webcrawler.HyperlinkPackage{}
				err = protobuf.Decode(data, hyperlinkPackage)
				if err != nil {
					log.Fatal("error decrypting crawler packet")
				}
				callBackChan <- hyperlinkPackage
				break
			}
		}
		if !encrypted {
			fmt.Println("failed to decrypt message, discarding..")
			return
		}
	}()

	return callBackChan
}
