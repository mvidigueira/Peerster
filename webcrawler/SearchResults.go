package webcrawler

import (
	"log"
	"math"
	"runtime/debug"
)

type SearchResults struct {
	Results      []SearchResult
	numberOfDocs int
}

type SearchResult struct {
	Link     string
	TfIdf    []float64
	SimScore float64
}

func (r *SearchResult) CalculateSimScore() float64 {
	n := float64(len(r.TfIdf))
	r.SimScore = 0
	for _, i := range r.TfIdf {
		r.SimScore += i * i
	}
	r.SimScore /= n + 1
	r.SimScore = math.Sqrt(r.SimScore)
	if math.IsNaN(r.SimScore) {
		log.Println("SimScore is NaN")
		debug.PrintStack()
	}
	return r.SimScore
}

type RankedResult struct {
	Result   *SearchResult
	PageRank float64
	Rank     float64
}

type RankedResults struct {
	Results []*RankedResult
}

func (rrs RankedResults) Len() int {
	return len(rrs.Results)
}

func (rrs RankedResults) Swap(i, j int) {
	rrs.Results[i], rrs.Results[j] = rrs.Results[j], rrs.Results[i]
}

func (rrs RankedResults) Less(i, j int) bool {
	return rrs.Results[i].Rank > rrs.Results[j].Rank
}

func NewSearchResults(urlMap *KeywordToURLMap, numberOfDocs int) (results SearchResults) {
	numberOfResults := len(urlMap.LinkData)
	for k, occurrences := range urlMap.LinkData {
		tfidf := calculateTfIdfForNewResult(occurrences, numberOfResults, numberOfDocs)
		results.Results = append(results.Results, SearchResult{k, []float64{tfidf}, -1})
	}
	results.numberOfDocs = numberOfDocs
	return
}

func calculateTfIdfForNewResult(keywordOccurences int, numberOfResults int, numberOfDocs int) float64 {
	if numberOfDocs <= numberOfResults {
		numberOfDocs = numberOfResults + 3
	}
	idf := math.Log(float64(numberOfDocs) / float64(1+numberOfResults))
	tfidf := math.Log(float64(1+keywordOccurences)) * idf
	if math.IsNaN(tfidf) {
		log.Println("TfIdf is NaN")
		debug.PrintStack()
	}
	return tfidf
}

func (rs SearchResults) Len() int {
	return len(rs.Results)
}

func (rs SearchResults) Swap(i, j int) {
	rs.Results[i], rs.Results[j] = rs.Results[j], rs.Results[i]
}

func (rs SearchResults) Less(i, j int) bool {
	return rs.Results[i].CalculateSimScore() > rs.Results[j].CalculateSimScore()
}

func Join(this SearchResults, urlMap *KeywordToURLMap) (results SearchResults) {
	numberOfResults := len(urlMap.LinkData)
	for _, result := range this.Results {
		otherOccurrences, found := urlMap.LinkData[result.Link]
		if found {
			result.TfIdf = append(result.TfIdf, calculateTfIdfForNewResult(otherOccurrences, numberOfResults, this.numberOfDocs))
			results.Results = append(results.Results, result)
		}
	}
	results.numberOfDocs = this.numberOfDocs
	return
}
