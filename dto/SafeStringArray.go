package dto

import "sync"

type SafeStringArray struct {
	exists map[string]bool
	array  []string
	mux    sync.Mutex
}

func NewSafeStringArray(members []string) *SafeStringArray {
	m := make(map[string]bool)
	for _, member := range members {
		m[member] = true
	}
	return &SafeStringArray{exists: m, array: members, mux: sync.Mutex{}}
}

func (ssa *SafeStringArray) AppendUniqueToArray(item string) (isNew bool) {
	ssa.mux.Lock()
	defer ssa.mux.Unlock()
	_, ok := ssa.exists[item]
	if ok {
		return false
	} else {
		ssa.exists[item] = true
		ssa.array = append(ssa.array, item)
		return true
	}
}

func (ssa *SafeStringArray) GetArrayCopy() []string {
	ssa.mux.Lock()
	defer ssa.mux.Unlock()
	arrayCopy := make([]string, len(ssa.array))
	copy(arrayCopy, ssa.array)
	return arrayCopy
}

func (ssa *SafeStringArray) GetLength() int {
	ssa.mux.Lock()
	defer ssa.mux.Unlock()
	return len(ssa.array)
}
