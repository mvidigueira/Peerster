package webcrawler

import (
	"math"
)

type SearchResults struct {
	Results []SearchResult
}

type SearchResult struct {
	Link string
	KeywordOccurences int
	TdIdf float64
}

func NewSearchResults(urlMap *KeywordToURLMap, numberOfDocs int) (results SearchResults){
	idf := float64(numberOfDocs)/float64(1+len(urlMap.LinkData))
	idf = math.Log(idf)
	for k, v := range urlMap.LinkData {
		results.Results = append(results.Results, SearchResult{k, v, math.Log(float64(v))*idf})
	}
	return
}

func (rs SearchResults) Len() int{
	return len(rs.Results)
}

func (rs SearchResults) Swap(i, j int) {
	rs.Results[i], rs.Results[j] = rs.Results[j], rs.Results[i]
}

func (rs SearchResults) Less(i, j int) bool {
	return rs.Results[i].TdIdf > rs.Results[j].TdIdf
}

func Join(this SearchResults, urlMap *KeywordToURLMap) (results SearchResults){
	for _, result := range this.Results {
		_, found := urlMap.LinkData[result.Link]
		if found {
			results.Results = append(results.Results, result)
		}
	}
	return
}