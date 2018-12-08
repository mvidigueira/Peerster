package fileparsing

import (
	"sort"
	"sync"
)

//SafeFileEntry - for keeping information per file
type SafeFileEntry struct {
	name         string
	size         int
	metafile     []byte
	metahash     [32]byte
	chunkIndices []uint64
	mux          sync.RWMutex
}

//NewSafeFileEntry - for the creation of SafeFileEntry
func NewSafeFileEntry(name string, size int, metafile []byte, metahash [32]byte) *SafeFileEntry {
	chunkNum := len(metafile) / 32
	chunkIndices := make([]uint64, chunkNum)
	for i := 1; i <= chunkNum; i++ {
		chunkIndices[i-1] = uint64(i)
	}
	return &SafeFileEntry{name: name, size: size, metafile: metafile, metahash: metahash, chunkIndices: chunkIndices, mux: sync.RWMutex{}}
}

//SafeGetContents - atomically retrieves contents of SafeFileEntry
func (sfe *SafeFileEntry) SafeGetContents() (name string, size int, metafile []byte, metahash [32]byte, chunkIndices []uint64) {
	sfe.mux.RLock()
	defer sfe.mux.RUnlock()
	name = sfe.name
	size = sfe.size
	metafile = sfe.metafile
	metahash = sfe.metahash
	chunkIndices = sfe.chunkIndices
	sort.Slice(chunkIndices, func(i, j int) bool { return chunkIndices[i] < chunkIndices[j] })
	return
}

//SafeSetSize - atomically changes the SafeFileEntry's size field to 'newSize'
func (sfe *SafeFileEntry) SafeSetSize(newSize int) {
	sfe.mux.Lock()
	defer sfe.mux.Unlock()
	sfe.size = newSize
}

//SafeAddChunkIndex - atomically adds a chunk index to the SafeFileEntry's chunkIndices
func (sfe *SafeFileEntry) SafeAddChunkIndex(index uint64) {
	sfe.mux.Lock()
	defer sfe.mux.Unlock()
	sfe.chunkIndices = append(sfe.chunkIndices, index)
}
