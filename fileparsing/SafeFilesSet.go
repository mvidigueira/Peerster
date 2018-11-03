package fileparsing

import (
	"sync"
)

type SafeFileSet struct {
	filesSet sync.Map
}

func NewSafeFileSet() *SafeFileSet {
	return &SafeFileSet{filesSet: sync.Map{}}
}

func (sfs *SafeFileSet) AddUnique(metahash [32]byte) (isNew bool) {
	_, notNew := sfs.filesSet.LoadOrStore(metahash, true)
	return !notNew
}

func (sfs *SafeFileSet) Delete(metahash [32]byte) {
	sfs.filesSet.Delete(metahash)
}
