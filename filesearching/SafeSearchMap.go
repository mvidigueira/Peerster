package filesearching

import (
	"sync"

	"github.com/mvidigueira/Peerster/dto"
)

//SafeSearchMap - for keeping information regarding current keyword searches
type SafeSearchMap struct {
	searchesMap sync.Map
}

//NewSafeSearchMap - for the creation of SafeSearchMap
func NewSafeSearchMap() *SafeSearchMap {
	return &SafeSearchMap{searchesMap: sync.Map{}}
}

//AddListener - adds a Listener to the SafeSearchMap.
//Returns the channel to be used to listen for the SearchResults.
//The isNew value is true if there is no concurrent search for the exact same keywords, false otherwise.
func (ssm *SafeSearchMap) AddListener(keywords []string) (cRes chan []*dto.SearchResult, isNew bool) {
	cRes = make(chan []*dto.SearchResult)
	sme := NewSearchMapEntry(cRes)
	_, notNew := ssm.searchesMap.LoadOrStore(keywords, sme)
	return cRes, !notNew
}

//InformListeners - for each SearchResult in a SearchReply, AppendsSearchResult to a search (entry)
//if the filename of the SearchResult contains any keyword of that search (entry key).
//Also informs the listener (for that entry) of all search results found so far, and removes the entry,
//if the #totalMatches (SearchResults with complete chunkMaps) is greater or equal than the defined threshold.
func (ssm *SafeSearchMap) InformListeners(srep dto.SearchReply) {
	appendResultAndInform := func(keywordsI interface{}, smeI interface{}) bool {
		keywords := keywordsI.([]string)
		sme := smeI.(*SearchMapEntry)

		for _, sres := range srep.Results {
			if ContainsKeyword(sres.FileName, keywords) {
				shouldInform := sme.AppendSearchResult(sres)
				if shouldInform {
					ssm.searchesMap.Delete(keywords)
					sme.Inform()
					break
				}
			}
		}

		return true
	}

	ssm.searchesMap.Range(appendResultAndInform)
}
