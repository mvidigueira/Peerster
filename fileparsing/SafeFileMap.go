package fileparsing

import (
	"sync"

	"github.com/mvidigueira/Peerster/dto"
)

//SafeFileMap - for keeping file information
type SafeFileMap struct {
	filesMap sync.Map
}

//NewSafeFileMap - for the creation of SafeFileMap
func NewSafeFileMap() *SafeFileMap {
	return &SafeFileMap{filesMap: sync.Map{}}
}

//AddEntry - adds a SafeFileEntry to the SafeFileMap
//returns true if there was no element with that SafeFileEntry's metahash
func (sfm *SafeFileMap) AddEntry(name string, size int, metafile []byte, metahash [32]byte) (isNew bool) {
	sfe := NewSafeFileEntry(name, size, metafile, metahash)
	_, notNew := sfm.filesMap.LoadOrStore(sfe.metahash, sfe)
	return !notNew
}

//GetEntry - returns the element that maps to the provided metahash
//ok's value is true if the element was present, false otherwise
func (sfm *SafeFileMap) GetEntry(metahash [32]byte) (sfe *SafeFileEntry, ok bool) {
	sfei, ok := sfm.filesMap.Load(metahash)
	if !ok {
		return nil, ok
	}
	return sfei.(*SafeFileEntry), ok
}

//GetMatches - returns a list of search results for file names matching at least one of the keywords (substring)
func (sfm *SafeFileMap) GetMatches(keywords []string) (sress []*dto.SearchResult) {
	sress = make([]*dto.SearchResult, 0)

	keywordMatcher := func(metahashI interface{}, sfeI interface{}) bool {
		sfe := sfeI.(*SafeFileEntry)
		name, _, metafile, metahash, chunkIndices := sfe.SafeGetContents()
		if ContainsKeyword(name, keywords) {
			chunkCount := uint64(len(metafile) / 32)
			sres := &dto.SearchResult{FileName: name, MetafileHash: metahash[:], ChunkMap: chunkIndices, ChunkCount: chunkCount}
			sress = append(sress, sres)
		}
		return true
	}
	sfm.filesMap.Range(keywordMatcher)

	return
}
