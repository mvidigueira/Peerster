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
}

type KeywordToURLMap struct {
	Keyword  string
	LinkData map[string]int //url:keywordOccurrences
}

// You cannot serialize plain lists with protobuf but you have to wrap it in a struct
type BatchMessage struct {
	UrlMapList            []*KeywordToURLMap
	OutBoundLinksPackages []*OutBoundLinksPackage
	Citations             []*Citations
}

type HyperlinkPackage struct {
	Links []string
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
