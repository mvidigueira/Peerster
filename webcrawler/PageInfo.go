package webcrawler

import (
	"github.com/mvidigueira/Peerster/dht_util"
)

type PageInfo struct {
	Hyperlinks         []string
	KeywordFrequencies map[string]int
	Hash               [dht_util.IDByteSize]byte
}
