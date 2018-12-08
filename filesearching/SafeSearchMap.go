package filesearching

import (
	"fmt"
	"strings"
	"sync"

	"github.com/mvidigueira/Peerster/dto"
	"github.com/mvidigueira/Peerster/fileparsing"
)

const ThresholdTotalMatches = 2

func keywordsToHashable(ss []string) string {
	return strings.Join(ss, ",")
}
func hashableToKeywords(s string) []string {
	return strings.Split(s, ",")
}

//SafeSearchMap - for keeping information regarding current keyword searches
type SafeSearchMap struct {
	searchesMap sync.Map
}

//NewSafeSearchMap - for the creation of SafeSearchMap
func NewSafeSearchMap() *SafeSearchMap {
	return &SafeSearchMap{searchesMap: sync.Map{}}
}

//AddListener - adds a Listener to the SafeSearchMap.
//Returns the channel to be used to listen for the file hashes and names map.
//The isNew value is true if there is no concurrent search for the exact same keywords, false otherwise.
func (ssm *SafeSearchMap) AddListener(keywords []string) (cFiles chan map[[32]byte]*dto.SafeStringArray, isNew bool) {
	cFiles = make(chan map[[32]byte]*dto.SafeStringArray)
	sme := NewSearchMapEntry(cFiles)
	_, notNew := ssm.searchesMap.LoadOrStore(keywordsToHashable(keywords), sme)
	return cFiles, !notNew
}

//RemoveListener - removes a Listener from the SafeSearchMap.
func (ssm *SafeSearchMap) RemoveListener(keywords []string) {
	ssm.searchesMap.Delete(keywordsToHashable(keywords))
	return
}

//AddListenerWithFiles - adds a Listener to the SafeSearchMap with locally presearched files.
//Returns the channel to be used to listen for the file hashes and names map.
//The isNew value is true if there is no concurrent search for the exact same keywords, false otherwise.
func (ssm *SafeSearchMap) AddListenerWithFiles(keywords []string, files map[[32]byte]*dto.SafeStringArray) (cFiles chan map[[32]byte]*dto.SafeStringArray, isNew bool) {
	cFiles = make(chan map[[32]byte]*dto.SafeStringArray)
	sme := NewSearchMapEntryWithFiles(cFiles, files)
	_, notNew := ssm.searchesMap.LoadOrStore(keywordsToHashable(keywords), sme)
	return cFiles, !notNew
}

//CheckIfMatches - TODO write this comment
func (ssm *SafeSearchMap) CheckIfMatches(filenames []string, metahash [32]byte) (madeAnyMatch bool) {
	appendResultAndInform := func(keywordsI interface{}, smeI interface{}) bool {
		keywords := hashableToKeywords(keywordsI.(string))

		for _, name := range filenames {
			matches := make([]string, 0)
			if fileparsing.ContainsKeyword(name, keywords) {
				matches = append(matches, name)
				madeAnyMatch = true
				return false
			}
		}

		return true
	}

	ssm.searchesMap.Range(appendResultAndInform)
	return madeAnyMatch
}

//InformListeners - for each SearchResult in a SearchReply, AppendsSearchResult to a search (entry)
//if the filename of the SearchResult contains any keyword of that search (entry key).
//Also informs the listener (for that entry) of all search results found so far, and removes the entry,
//if the #totalMatches (SearchResults with complete chunkMaps) is greater or equal than the defined threshold.
func (ssm *SafeSearchMap) InformListeners(filenames []string, metahash [32]byte) {
	appendResultAndInform := func(keywordsI interface{}, smeI interface{}) bool {
		keywords := hashableToKeywords(keywordsI.(string))
		sme := smeI.(*SearchMapEntry)

		//fmt.Printf("keywords: %v\n", keywords)

		for _, name := range filenames {
			matches := make([]string, 0)
			if fileparsing.ContainsKeyword(name, keywords) {
				matches = append(matches, name)
			}
			//fmt.Printf("matches: %v\n", matches)
			if len(matches) > 0 {
				shouldInform := sme.AppendFiles(matches, metahash)
				//fmt.Printf("shouldInform: %v\n", shouldInform)
				if shouldInform {
					ssm.searchesMap.Delete(keywordsI.(string))
					sme.Inform()
					//fmt.Printf("Informed\n")
					break
				}
			}
		}

		return true
	}

	ssm.searchesMap.Range(appendResultAndInform)
	return
}

//SearchMapEntry - Entry for the SafeSearchMap
type SearchMapEntry struct {
	Files  map[[32]byte]*dto.SafeStringArray
	cFiles chan map[[32]byte]*dto.SafeStringArray
}

//NewSearchMapEntry - for the creation of SearchMapEntry
func NewSearchMapEntry(cRes chan map[[32]byte]*dto.SafeStringArray) *SearchMapEntry {
	return &SearchMapEntry{Files: make(map[[32]byte]*dto.SafeStringArray), cFiles: cRes}
}

//NewSearchMapEntryWithFiles - for the creation of SearchMapEntry with a prearranged files map
func NewSearchMapEntryWithFiles(cRes chan map[[32]byte]*dto.SafeStringArray, files map[[32]byte]*dto.SafeStringArray) *SearchMapEntry {
	return &SearchMapEntry{Files: files, cFiles: cRes}
}

//AppendFiles - appends a metahash (total match) to the SearchMapEntry's array.
//Returns true if the number of pending matches is greater or equal to some 'ThresholdTotalMatches'.
func (sme *SearchMapEntry) AppendFiles(filenames []string, metahash [32]byte) (shouldInform bool) {
	arr, ok := sme.Files[metahash]
	if ok {
		for _, name := range filenames {
			arr.AppendUniqueToArray(name)
		}
	} else {
		sme.Files[metahash] = dto.NewSafeStringArray([]string{})
	}
	fmt.Printf("len(sme.Files): %v\n", len(sme.Files))
	if len(sme.Files) >= ThresholdTotalMatches {
		return true
	}
	return false
}

//GetChannel - returns the SearchMapEntry's channel
func (sme *SearchMapEntry) GetChannel() (cFiles chan map[[32]byte]*dto.SafeStringArray) {
	return sme.cFiles
}

//Inform - passes the SearchMapEntry's pending results through its channel
func (sme *SearchMapEntry) Inform() {
	sme.cFiles <- sme.Files
}
