package webcrawler

import (
	"github.com/mvidigueira/Peerster/dht"
)

type SearchResults []SearchResult

type SearchResult struct {
	Link string
	KeywordOccurences int
}

func NewSearchResults(urlMap *dht.KeywordToURLMap) (results SearchResults){
	for k, v := range urlMap.LinkData {
		results = append(results, SearchResult{k, v})
	}
	return
}

func (rs SearchResults) Len() int{
	return len(rs)
}

func (rs SearchResults) Swap(i, j int) {
	rs[i], rs[j] = rs[j], rs[i]
}

func (rs SearchResults) Less(i, j int) bool {
	return rs[i].KeywordOccurences > rs[j].KeywordOccurences
}

func Join(this SearchResults, urlMap *dht.KeywordToURLMap) (results SearchResults){
	for _, result := range this {
		_, found := urlMap.LinkData[result.Link]
		if found {
			results = append(results, result)
		}
	}
	return
}