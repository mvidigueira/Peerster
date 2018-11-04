package fileparsing

import (
	"sync"
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
