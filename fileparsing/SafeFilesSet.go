package fileparsing

import (
	"sync"
)

//SafeFileSet - Atomic Set for files being downloaded
//(made for stopping double downloads of the same file)
type SafeFileSet struct {
	filesSet sync.Map
}

//NewSafeFileSet - for the creation of SafeFileSets
func NewSafeFileSet() *SafeFileSet {
	return &SafeFileSet{filesSet: sync.Map{}}
}

//AddUnique - adds a file (identified by its metahash) to the set
//returns true if it was not present before, false otherwise
func (sfs *SafeFileSet) AddUnique(metahash [32]byte) (isNew bool) {
	_, notNew := sfs.filesSet.LoadOrStore(metahash, true)
	return !notNew
}

//Delete - deletes a file from the set (identified by its metahash)
func (sfs *SafeFileSet) Delete(metahash [32]byte) {
	sfs.filesSet.Delete(metahash)
}
