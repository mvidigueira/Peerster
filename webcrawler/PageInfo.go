package webcrawler

import "github.com/mvidigueira/Peerster/dht"

type PageInfo struct {
	Hyperlinks         []string
	KeywordFrequencies map[string]int
	Hash               [dht.IDByteSize]byte
}
