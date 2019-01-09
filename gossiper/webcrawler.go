package gossiper

import (
	"fmt"
	"github.com/mvidigueira/Peerster/dht_util"
	"protobuf"
	"time"

	"github.com/mvidigueira/Peerster/dht"
	"github.com/mvidigueira/Peerster/dto"
	"github.com/mvidigueira/Peerster/webcrawler"
)

//webCrawlerListenRouting - deals with indexing of webpages and hyperlink distribution
func (g *Gossiper) webCrawlerListenerRoutine() {
	for packet := range g.webCrawler.OutChan {
		switch {
		case packet.OutBoundLinks != nil:
			g.saveOutboundLinksInDHT(packet.OutBoundLinks)
		case packet.CitationsPackage != nil:
			g.saveCitationsInDHT(packet.CitationsPackage)
		case packet.HyperlinkPackage != nil:
			g.distributeHyperlinks(packet.HyperlinkPackage)
		case packet.IndexPackage != nil:
			g.saveKeywordsInDHT(packet.IndexPackage)
		case packet.PageHash != nil && packet.PageHash.Type == "store":
			g.savePageHashInDHT(packet.PageHash)
		case packet.PageHash != nil && packet.PageHash.Type == "lookup":
			_, found := g.LookupValue(packet.PageHash.Hash, dht.PageHashBucket)
			packet.ResChan <- found
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
		gossiperPacket := &dto.GossipPacket{
			HyperlinkMessage: &webcrawler.HyperlinkPackage{
				Links: tmp,
			},
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
		err = g.sendStore(owner, [20]byte{}, packetBytes, dht.KeywordsBucket)
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
		err = g.sendStore(owner, [20]byte{}, packetBytes, dht.CitationsBucket)
		if err != nil {
			fmt.Printf("Failed to store key.\n")
		}
	}
}

// Identifies the responsible node for each hyperlink. If the hyperlink belongs to this node, then the hyperlinks are simply sent back
// to the local webcrawler. If the hyperlinks belong to another webcrawlers domain, then they will be sent to the corresponding node.
func (g *Gossiper) distributeHyperlinks(hyperlinkPackage *webcrawler.HyperlinkPackage) {
	links := hyperlinkPackage.Links
	domains := map[dht.NodeState][]string{}
	for _, hyperlink := range links {
		hash := dht_util.GenerateKeyHash(hyperlink)
		closestNodes := g.LookupNodes(hash)
		if len(closestNodes) == 0 {
			fmt.Println("LookupNodes returned an empty list... Retrying in 5 seconds.")
			// Retry in 5 seconds.
			go func() {
				time.Sleep(time.Second * 5)
				g.distributeHyperlinks(hyperlinkPackage)
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
			g.webCrawler.InChan <- &webcrawler.CrawlerPacket{
				HyperlinkPackage: &webcrawler.HyperlinkPackage{
					Links: hyperlinks,
				},
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
			Url: outboundLinks.Url,
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
		err = g.sendStore(closest, id, packetBytes, dht.LinksBucket)
		if err != nil {
			fmt.Printf("Failed to store key.\n")
		}
	}
}

func (g *Gossiper) saveCitationsInDHT(citationsPackage *webcrawler.CitationsPackage){
	destinations := make(map[dht.NodeState][]*webcrawler.Citations)
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
			msg := g.newDHTStore(dest.NodeID, packetBytes, dht.CitationsBucket)
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
			msg := g.newDHTStore(dest.NodeID, packetBytes, dht.KeywordsBucket)
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
