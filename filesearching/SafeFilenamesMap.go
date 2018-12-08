package filesearching

import (
	"sync"

	"github.com/mvidigueira/Peerster/dto"
	"github.com/mvidigueira/Peerster/fileparsing"
)

//SafeFilenamesMap - for keeping information regarding current keyword searches
type SafeFilenamesMap struct {
	filenamesMap sync.Map
}

//NewSafeFileNamesMap - for the creation of SafeSearchMap
func NewSafefileNamesMap() *SafeFilenamesMap {
	return &SafeFilenamesMap{filenamesMap: sync.Map{}}
}

//AddMapping - adds a filename to the metahash
//returns true if there was no element with that SafeFileEntry's metahash
func (sfm *SafeFilenamesMap) AddMapping(metahash [32]byte, filename string) (isNew bool) {
	arr := dto.NewSafeStringArray([]string{filename})
	arrI, notNew := sfm.filenamesMap.LoadOrStore(metahash, arr)
	if notNew {
		arr = arrI.(*dto.SafeStringArray)
		arr.AppendUniqueToArray(filename)
	}
	return !notNew
}

//GetMapping - gets a metahashes filenames
//ok returns true if the metahash was present, false otherwise
func (sfm *SafeFilenamesMap) GetMapping(metahash [32]byte) (arr *dto.SafeStringArray, ok bool) {
	arrI, ok := sfm.filenamesMap.Load(metahash)
	if ok {
		return arrI.(*dto.SafeStringArray), true
	}
	return nil, false
}

//GetMatches - returns a map of metahash and names with hashes with names matching at least one of the keywords (substring)
func (sfm *SafeFilenamesMap) GetMatches(keywords []string) map[[32]byte]*dto.SafeStringArray {
	mp := make(map[[32]byte]*dto.SafeStringArray)

	keywordMatcher := func(metahashI interface{}, ssaI interface{}) bool {
		ssa := ssaI.(*dto.SafeStringArray)
		metahash := metahashI.([32]byte)
		names := ssa.GetArrayCopy()

		newssa := dto.NewSafeStringArray([]string{})
		for _, name := range names {
			if fileparsing.ContainsKeyword(name, keywords) {
				newssa.AppendUniqueToArray(name)
			}
		}
		if newssa.GetLength() > 0 {
			mp[metahash] = newssa
		}

		return true
	}
	sfm.filenamesMap.Range(keywordMatcher)

	return mp
}
