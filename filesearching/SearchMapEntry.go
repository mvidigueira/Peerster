package filesearching

import (
	"github.com/mvidigueira/Peerster/dto"
)

const thresholdTotalMatches = 2

//SearchMapEntry - Entry for the SafeSearchMap
type SearchMapEntry struct {
	NumFullMatches int
	Results        []*dto.SearchResult
	cResults       chan []*dto.SearchResult
}

//NewSearchMapEntry - for the creation of SearchMapEntry
func NewSearchMapEntry(cRes chan []*dto.SearchResult) *SearchMapEntry {
	return &SearchMapEntry{NumFullMatches: 0, Results: make([]*dto.SearchResult, 0), cResults: cRes}
}

//AppendSearchResult - appends a SearchResult to the SearchMapEntry's sr array.
//Returns true if the number of pending search results is greater or equal to some 'thresholdTotalMatches'.
func (sme *SearchMapEntry) AppendSearchResult(sres *dto.SearchResult) (shouldInform bool) {
	sme.Results = append(sme.Results, sres)
	if sres.IsFullMatch() {
		sme.NumFullMatches++
	}
	if sme.NumFullMatches >= thresholdTotalMatches {
		return true
	}
	return false
}

//GetChannel - returns the SearchMapEntry's channel
func (sme *SearchMapEntry) GetChannel() (cRes chan []*dto.SearchResult) {
	return sme.cResults
}

//Inform - passes the SearchMapEntry's pending results through its channel
func (sme *SearchMapEntry) Inform() {
	sme.cResults <- sme.Results
}
