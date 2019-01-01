package gossiper

import (
	"crypto/sha1"
	"fmt"
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
func (g *Gossiper) batchSendURLS(owner string, hyperlinks []string) {
	udpMaxSize := 8192 // Maximum safe UDP size.
	packet := make([]string, 0, udpMaxSize)
	packetSize := 0
	for _, hyperlink := range hyperlinks {
		if packetSize+len(hyperlink) < udpMaxSize {
			packetSize += len(hyperlink)
			packet = append(packet, hyperlink)
		} else {
			gossiperPacket := &dto.GossipPacket{
				HyperlinkMessage: &webcrawler.HyperlinkPackage{
					Links: packet,
				},
			}
			fmt.Printf("SENDING UPD WITH SIZE %d\n", len(packet))
			g.sendUDP(gossiperPacket, owner)
			packetSize = 0
			packet = make([]string, 0, udpMaxSize)
		}
	}
	if len(packet) > 0 {
		gossiperPacket := &dto.GossipPacket{
			HyperlinkMessage: &webcrawler.HyperlinkPackage{
				Links: packet,
			},
		}
		g.sendUDP(gossiperPacket, owner)
	}
	fmt.Printf("SENDING UPD WITH SIZE %d\n", len(packet))
}

// Identifies the responsible node for each hyperlink. If the hyperlink belongs to this node, then the hyperlinks are simply sent back
// to the local webcrawler. If the hyperlinks belong to another webcrawlers domain, then they will be sent to the corresponding node.
func (g *Gossiper) distributeHyperlinks(hyperlinkPackage *webcrawler.HyperlinkPackage) {
	links := hyperlinkPackage.Links
	domains := map[string][]string{}
	for _, hyperlink := range links {
		hash := sha1.Sum([]byte(hyperlink))
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
		responsibleNode := closestNodes[0].Address
		hyperlinks, found := domains[responsibleNode]
		if !found {
			domains[responsibleNode] = []string{hyperlink}
		} else {
			hyperlinks = append(hyperlinks, hyperlink)
			domains[responsibleNode] = hyperlinks
		}
	}

	for owner, hyperlinks := range domains {
		if owner == g.address {
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
	for k, v := range frequencies {
		urlToKeywordMap := &dht.KeywordToURLMap{
			Keyword: k,
			Urls:    map[string]int{url: v},
		}
		keyHash := dht.GenerateKeyHash(k)
		kClosest := g.LookupNodes(keyHash)
		if len(kClosest) == 0 {
			fmt.Printf("Could not perform store since no neighbours found.\n")
			break
		}
		closest := kClosest[0]

		packetBytes, err := protobuf.Encode(urlToKeywordMap)
		if err != nil {
			fmt.Printf("Error encoding urlToKeywordMap\n")
			break
		}
		err = g.sendStore(closest, keyHash, packetBytes, "PUT")
		if err != nil {
			fmt.Printf("Failed to store key %s.\n", keyHash)
		}
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
