package webcrawler

import (
	"github.com/mvidigueira/Peerster/dht_util"
)

type CrawlerPacket struct {
	HyperlinkPackage *HyperlinkPackage
	OutBoundLinks    *OutBoundLinksPackage
	CitationsPackage *CitationsPackage
	IndexPackage     *IndexPackage
	PageHash         *PageHashPackage
	ResChan          chan bool
	Sender           string
	PastMapPackage   *PastMapPackage
	Done             *DoneCrawl
}

type KeywordToURLMap struct {
	Keyword  string
	LinkData map[string]int //url:keywordOccurrences
}

type HyperlinkPackage struct {
	Links          []string
	Encrypted      bool
	EncryptedLinks []byte
}

type EncryptedCrawlerPacket struct {
	Packet []byte // Shall hold a CrawlerPacket
}

type OutBoundLinksPackage struct {
	Url           string
	OutBoundLinks []string
}

type Citations struct {
	Url     string
	CitedBy []string
}

type CitationsPackage struct {
	CitationsList []Citations
}

type IndexPackage struct {
	KeywordFrequencies map[string]int
	Url                string
}

type PageHashPackage struct {
	Hash dht_util.TypeID
	Type string
}

type PastMapPackage struct {
	Content []byte
}

type DoneCrawl struct {
	Delete bool
}
