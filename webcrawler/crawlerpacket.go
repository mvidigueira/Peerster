package webcrawler

import (
	"github.com/mvidigueira/Peerster/dht"
)

type CrawlerPacket struct {
	HyperlinkPackage *HyperlinkPackage
	IndexPackage     *IndexPackage
	PageHash         *PageHashPackage
	ResChan          chan bool
}

type HyperlinkPackage struct {
	Links []string
}

type IndexPackage struct {
	KeywordFrequencies map[string]int
	Url                string
}

type PageHashPackage struct {
	Hash dht.TypeID
	Type string
}

type PagePacket struct {
	Keywords []string
	PageHash []byte
	Links    []string
	Url      string
}
