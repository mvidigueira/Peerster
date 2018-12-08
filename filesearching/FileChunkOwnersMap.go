package filesearching

import (
	"sync"

	"github.com/mvidigueira/Peerster/fileparsing"

	"github.com/mvidigueira/Peerster/dto"
)

//MetahashToChunkOwnersMap - for keeping information regarding owners of chunks for different files
type MetahashToChunkOwnersMap struct {
	chunkOwnersMaps sync.Map //metahash to chunk owners entry
}

//NewMetahashToChunkOwnersMap - for the creation of MetahashToChunkOwnersMap
func NewMetahashToChunkOwnersMap() *MetahashToChunkOwnersMap {
	return &MetahashToChunkOwnersMap{chunkOwnersMaps: sync.Map{}}
}

//UpdateWithSearchResult - updates the underlying maps given
func (mtcom *MetahashToChunkOwnersMap) UpdateWithSearchResult(origin string, sr *dto.SearchResult) (madeTotalMatch bool) {
	hash32, ok := fileparsing.ConvertToHash32(sr.MetafileHash)
	if ok {
		//fmt.Printf("chunkCount: %v\n", sr.ChunkCount)
		comi, _ := mtcom.chunkOwnersMaps.LoadOrStore(hash32, NewChunkOwnersMap(sr.ChunkCount))
		com := comi.(*ChunkOwnersMap)
		madeTotalMatch = com.UpdateMap(origin, sr.ChunkMap)
	}
	return
}

func (mtcom *MetahashToChunkOwnersMap) GetMapCopy(metahash [32]byte) (chunkOwners map[uint64]*dto.SafeStringArray, hasTotalMatch bool, loaded bool) {
	comi, loaded := mtcom.chunkOwnersMaps.Load(metahash)
	if loaded {
		com := comi.(*ChunkOwnersMap)
		chunkOwners, hasTotalMatch = com.GetMapCopy()
		return chunkOwners, hasTotalMatch, true
	}
	return nil, false, false
}

//ChunkOwnersMap - holds information on who has what chunks for each metafile
type ChunkOwnersMap struct {
	numChunks   uint64
	chunkOwners map[uint64]*dto.SafeStringArray
	mux         sync.RWMutex
}

//NewChunkOwnersMap - for the creation of SafeSearchMap
func NewChunkOwnersMap(numChunks uint64) *ChunkOwnersMap {
	chunkOwners := make(map[uint64]*dto.SafeStringArray)
	return &ChunkOwnersMap{numChunks: numChunks, chunkOwners: chunkOwners, mux: sync.RWMutex{}}
}

//UpdateMap - updates map information on what origins have those chunks
func (com *ChunkOwnersMap) UpdateMap(origin string, chunkMap []uint64) (madeTotalMatch bool) {
	com.mux.Lock()
	defer com.mux.Unlock()
	hadTotalMatch := len(com.chunkOwners) == int(com.numChunks)
	//fmt.Printf("len(com.chunkOwners): %v. numChunks: %v\n", len(com.chunkOwners), int(com.numChunks))
	//fmt.Printf("hadTotalMatch %v\n", hadTotalMatch)
	for _, v := range chunkMap {
		array, ok := com.chunkOwners[v]
		if ok {
			array.AppendUniqueToArray(origin)
		} else {
			com.chunkOwners[v] = dto.NewSafeStringArray([]string{origin})
		}
	}
	hasTotalMatch := len(com.chunkOwners) == int(com.numChunks)
	//fmt.Printf("hasTotalMatch %v\n", hasTotalMatch)
	if (!hadTotalMatch) && hasTotalMatch {
		//fmt.Printf("TRUE %v\n", hasTotalMatch)
		return true
	}
	return false
}

//GetMapCopy - returns an atomic copy of the map
func (com *ChunkOwnersMap) GetMapCopy() (chunkOwners map[uint64]*dto.SafeStringArray, hasTotalMatch bool) {
	com.mux.RLock()
	defer com.mux.RUnlock()
	chunkOwners = make(map[uint64]*dto.SafeStringArray)
	hasTotalMatch = len(com.chunkOwners) == int(com.numChunks)
	for k, v := range com.chunkOwners {
		chunkOwners[k] = v
	}
	return
}
