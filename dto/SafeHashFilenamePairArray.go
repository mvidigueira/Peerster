package dto

import (
	"fmt"
	"sync"
)

type MetahashFilenamePair struct {
	Metahash string
	Filename string
}

//SafeHashFilenamePairArray - safe array for MetahashFilenamePair
type SafeHashFilenamePairArray struct {
	exists map[MetahashFilenamePair]bool
	array  []MetahashFilenamePair
	mux    sync.Mutex
}

//NewSafeHashFilenamePairArray - creates a SafeHashFilenamePairArray containing the provided members
func NewSafeHashFilenamePairArray() *SafeHashFilenamePairArray {
	m := make(map[MetahashFilenamePair]bool)
	return &SafeHashFilenamePairArray{exists: m, array: []MetahashFilenamePair{}, mux: sync.Mutex{}}
}

//AppendUniqueToArray - appends element to array if unique
func (shfpa *SafeHashFilenamePairArray) AppendUniqueToArray(metahash [32]byte, filename string) (isNew bool) {
	hash := fmt.Sprintf("%x", metahash)
	item := MetahashFilenamePair{Metahash: hash, Filename: filename}
	shfpa.mux.Lock()
	defer shfpa.mux.Unlock()
	_, ok := shfpa.exists[item]
	if !ok {
		shfpa.exists[item] = true
		shfpa.array = append(shfpa.array, item)
		return true
	}
	return false
}

//GetArrayCopy - returns a copy of the underlying array (atomically)
func (shfpa *SafeHashFilenamePairArray) GetArrayCopy() []MetahashFilenamePair {
	shfpa.mux.Lock()
	defer shfpa.mux.Unlock()
	arrayCopy := make([]MetahashFilenamePair, len(shfpa.array))
	copy(arrayCopy, shfpa.array)
	return arrayCopy
}

//GetLength - returns the length of the underlying array (atomically)
func (shfpa *SafeHashFilenamePairArray) GetLength() int {
	shfpa.mux.Lock()
	defer shfpa.mux.Unlock()
	return len(shfpa.array)
}
