package dto

import "sync"

//SafeStringArray - safe array for strings
type SafeStringArray struct {
	exists map[string]bool
	array  []string
	mux    sync.Mutex
}

//NewSafeStringArray - creates a SafeStringArray containing the provided members
func NewSafeStringArray(members []string) *SafeStringArray {
	m := make(map[string]bool)
	for _, member := range members {
		m[member] = true
	}
	return &SafeStringArray{exists: m, array: members, mux: sync.Mutex{}}
}

//AppendUniqueToArray - appends element to array if unique
func (ssa *SafeStringArray) AppendUniqueToArray(item string) (isNew bool) {
	ssa.mux.Lock()
	defer ssa.mux.Unlock()
	_, ok := ssa.exists[item]
	if !ok {
		ssa.exists[item] = true
		ssa.array = append(ssa.array, item)
		return true
	}
	return false
}

//GetArrayCopy - returns a copy of the underlying array (atomically)
func (ssa *SafeStringArray) GetArrayCopy() []string {
	ssa.mux.Lock()
	defer ssa.mux.Unlock()
	arrayCopy := make([]string, len(ssa.array))
	copy(arrayCopy, ssa.array)
	return arrayCopy
}

//GetLength - returns the length of the underlying array (atomically)
func (ssa *SafeStringArray) GetLength() int {
	ssa.mux.Lock()
	defer ssa.mux.Unlock()
	return len(ssa.array)
}
