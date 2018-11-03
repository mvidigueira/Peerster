package fileparsing

import (
	"crypto/sha256"
	"sync"
)

//SafeChunkMap - for chunks
type SafeChunkMap struct {
	chunksMap sync.Map
}

//NewSafeChunkMap - for the creation of SafeChunkMap
func NewSafeChunkMap() *SafeChunkMap {
	return &SafeChunkMap{chunksMap: sync.Map{}}
}

//AddChunk - adds chunk to map with key equal to the provided checksum
//returns false if there already is a chunk with that checksum
func (scm *SafeChunkMap) AddChunk(checksum [32]byte, chunk []byte) (isNew bool) {
	_, notNew := scm.chunksMap.LoadOrStore(checksum, chunk)
	return !notNew
}

//AddUncheckedChunk - adds chunk to map with key equal to its SHA256 sum
//returns false if there already is a chunk with the computed checksum
func (scm *SafeChunkMap) AddUncheckedChunk(chunk []byte) (isNew bool) {
	_, notNew := scm.chunksMap.LoadOrStore(sha256.Sum256(chunk), chunk)
	return !notNew
}

//GetChunk - returns chunk ([]byte) with SHA256 sum equal to 'checksum'
func (scm *SafeChunkMap) GetChunk(checksum [32]byte) (chunk []byte, ok bool) {
	chunki, ok := scm.chunksMap.Load(checksum)
	if !ok {
		return nil, ok
	}
	return chunki.([]byte), ok
}
