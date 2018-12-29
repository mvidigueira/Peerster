package dht

import "sync"

//SafeNodeStateArray - safe array for node states
type SafeNodeStateArray struct {
	target    [IDByteSize]byte
	Queried   map[[IDByteSize]byte]bool
	Responded map[[IDByteSize]byte]bool
	array     *[]*NodeState
	mux       sync.Mutex
}

//NewSafeNodeStateArray - for the creation of an empty SafeNodeStateArray
func NewSafeNodeStateArray(target [IDByteSize]byte) *SafeNodeStateArray {
	array := make([]*NodeState, 0)
	queried := make(map[[IDByteSize]byte]bool)
	responded := make(map[[IDByteSize]byte]bool)
	return &SafeNodeStateArray{target: target, Queried: queried, Responded: responded, array: &array, mux: sync.Mutex{}}
}

//Insert - inserts in the array, leaving it ordered
func (snsa *SafeNodeStateArray) Insert(potential *NodeState) (closest bool) {
	snsa.mux.Lock()
	defer snsa.mux.Unlock()
	*snsa.array = InsertOrdered(snsa.target, *snsa.array, potential)
	if (*snsa.array)[0] == potential {
		closest = true
	}
	return
}

//GetAlphaUnqueried - returns alpha nodes (if there are at least alpha nodes) that have not been queried
func (snsa *SafeNodeStateArray) GetAlphaUnqueried(alpha int) (closest []*NodeState) {
	closest = make([]*NodeState, 0)
	snsa.mux.Lock()
	defer snsa.mux.Unlock()
	for _, ns := range *snsa.array {
		if !snsa.Queried[ns.NodeID] {
			closest = append(closest, ns)
			if alpha--; alpha == 0 {
				return
			}
		}
	}
	return
}

//GetArrayCopy - returns the current elements of the array and resets it
func (snsa *SafeNodeStateArray) GetArrayCopy() []*NodeState {
	snsa.mux.Lock()
	oldArray := *snsa.array
	snsa.mux.Unlock()
	return oldArray
}
