package webcrawler

type CrawlerPacket struct {
	HyperlinkPackage *HyperlinkPackage
	IndexPackage     *IndexPackage
}

type HyperlinkPackage struct {
	Links []string
}

type IndexPackage struct {
	Keywords []string
	Url      string
}
