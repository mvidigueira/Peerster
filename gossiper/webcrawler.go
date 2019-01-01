package gossiper

import (
	"fmt"
	"log"
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
		case packet.HyperlinkPackage != nil:
			g.distributeHyperlinks(packet.HyperlinkPackage)
		case packet.IndexPackage != nil:
			g.indexKeywordsInDHT(packet.IndexPackage)
		case packet.PageHash != nil && packet.PageHash.Type == "store":
			g.savePageHashInDHT(packet.PageHash)
		case packet.PageHash != nil && packet.PageHash.Type == "lookup":
			_, found := g.LookupValue(packet.PageHash.Hash)
			packet.ResChan <- found
		}
	}
}

// The maximum safe size of a UDP packet is 8192 but it could happen that some pages contains a set of urls
// which is larger than 8192 bytes and hence we need to break down the url packages into batches which are smaller than 8192bytes.
func (g *Gossiper) createUDPBatches(owner *dht.NodeState, items []interface{}) []interface{} {
	udpMaxSize := 7500 // Maximum safe UDP size.
	packet := make([]interface{}, 0, udpMaxSize)
	packetSize := 0
	batches := make([]interface{}, 0, 1)
	for _, item := range items {
		var itemLength int
		switch v := item.(type) {
		case string:
			itemLength = len(v)
		case *dht.KeywordToURLMap:
			bytes, _ := protobuf.Encode(item)
			itemLength = len(bytes)
		default:
			log.Fatal("invalid interface type")
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

func (g *Gossiper) batchSendKeywords(owner *dht.NodeState, items []*dht.KeywordToURLMap) {
	s := make([]interface{}, len(items))
	for i, v := range items {
		s[i] = v
	}
	batches := g.createUDPBatches(owner, s)
	for _, batch := range batches {
		tmp := make([]*dht.KeywordToURLMap, len(batch.([]interface{})))
		for k, b := range batch.([]interface{}) {
			tmp[k] = b.(*dht.KeywordToURLMap)
		}
		packetBytes, err := protobuf.Encode(&dht.KeywordToURLBatchStruct{List: tmp})
		if err != nil {
			fmt.Println(err)
			fmt.Printf("Error encoding urlToKeywordMap.\n")
			break
		}
		err = g.sendStore(owner, [20]byte{}, packetBytes, "PUT")
		if err != nil {
			fmt.Printf("Failed to store key.\n")
		}
	}
}

// Identifies the responsible node for each hyperlink. If the hyperlink belongs to this node, then the hyperlinks are simply sent back
// to the local webcrawler. If the hyperlinks belong to another webcrawlers domain, then they will be sent to the corresponding node.
func (g *Gossiper) distributeHyperlinks(hyperlinkPackage *webcrawler.HyperlinkPackage) {
	links := hyperlinkPackage.Links
	domains := map[*dht.NodeState][]string{}
	for _, hyperlink := range links {
		hash := dht.GenerateKeyHash(hyperlink)
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
		responsibleNode := closestNodes[0]
		hyperlinks, found := domains[responsibleNode]
		if !found {
			domains[responsibleNode] = []string{hyperlink}
		} else {
			hyperlinks = append(hyperlinks, hyperlink)
			domains[responsibleNode] = hyperlinks
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
			// url belong to another crawlers domain.
			g.batchSendURLS(owner, hyperlinks)
		}
	}
}

// Save each keyword of a url at the responsible node
func (g *Gossiper) indexKeywordsInDHT(indexPackage *webcrawler.IndexPackage) {
	frequencies, url := indexPackage.KeywordFrequencies, indexPackage.Url
	batches := map[string][]*dht.KeywordToURLMap{}
	addressToNodeState := map[string]*dht.NodeState{}
	for k, v := range frequencies {

		keyHash := dht.GenerateKeyHash(k)
		kClosest := g.LookupNodes(keyHash)
		if len(kClosest) == 0 {
			fmt.Printf("Could not perform store since no neighbours found.\n")
			break
		}
		closest := kClosest[0]

		addressToNodeState[closest.Address] = closest

		urlToKeywordMap := &dht.KeywordToURLMap{
			KeywordHash: keyHash,
			Urls:        map[string]int{url: v},
		}

		val, found := batches[closest.Address]
		if !found {
			batches[closest.Address] = []*dht.KeywordToURLMap{
				urlToKeywordMap,
			}
		} else {
			batches[closest.Address] = append(val, urlToKeywordMap)
		}
	}

	for k, batch := range batches {
		addr, _ := addressToNodeState[k]
		g.batchSendKeywords(addr, batch)

	}
}

// Saves the hash of a page in the DHT. This is used to prevent duplicate crawling of the same page.
// The reason why we hash the content instead of the url is becasue there might be different urls which points to the same page.
func (g *Gossiper) savePageHashInDHT(pageHash *webcrawler.PageHashPackage) {
	kClosest := g.LookupNodes(pageHash.Hash)
	if len(kClosest) == 0 {
		fmt.Printf("Could not perform store since no neighbours found.\n")
		return
	}
	closest := kClosest[0]
	err := g.sendStore(closest, pageHash.Hash, []byte(""), "POST")
	if err != nil {
		fmt.Printf("Failed to store key %s.\n", pageHash)
	}
}
