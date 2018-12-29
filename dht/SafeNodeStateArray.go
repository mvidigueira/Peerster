package dht

import "sync"

//SafeNodeStateArray - safe array for node states
type SafeNodeStateArray struct {
	Target    [IDByteSize]byte
	exists    map[[IDByteSize]byte]bool
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
	exists := make(map[[IDByteSize]byte]bool)
	return &SafeNodeStateArray{Target: target, Queried: queried, exists: exists, Responded: responded, array: &array, mux: sync.Mutex{}}
}

//Insert - inserts in the array, leaving it ordered
func (snsa *SafeNodeStateArray) Insert(potential *NodeState) (closest bool) {
	snsa.mux.Lock()
	defer snsa.mux.Unlock()
	if snsa.exists[potential.NodeID] {
		return false
	}
	snsa.exists[potential.NodeID] = true

	*snsa.array = InsertOrdered(snsa.Target, *snsa.array, potential)
	if (*snsa.array)[0] == potential {
		closest = true
	}
	return
}

// SetQueried - marks node 'ns' as queried
func (snsa *SafeNodeStateArray) SetQueried(ns *NodeState) {
	snsa.mux.Lock()
	defer snsa.mux.Unlock()
	snsa.Queried[ns.NodeID] = true
}

// SetResponded - marks node 'ns' as having responded
func (snsa *SafeNodeStateArray) SetResponded(ns *NodeState) {
	snsa.mux.Lock()
	defer snsa.mux.Unlock()
	snsa.Responded[ns.NodeID] = true
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

func (snsa *SafeNodeStateArray) hasResponded(ns *NodeState) bool {
	return snsa.Responded[ns.NodeID]
}

func (snsa *SafeNodeStateArray) wasQueried(ns *NodeState) bool {
	return snsa.Queried[ns.NodeID]
}

//GetKClosestUnqueried - returns unqueried nodes from the k closest nodes that are responsive
func (snsa *SafeNodeStateArray) GetKClosestUnqueried(k int) (unqueried, responded []*NodeState) {
	unqueried = make([]*NodeState, 0)
	responded = make([]*NodeState, 0)
	snsa.mux.Lock()
	defer snsa.mux.Unlock()
	for _, node := range *snsa.array {
		if snsa.hasResponded(node) {
			responded = append(responded, node)
			k--
		} else if !snsa.wasQueried(node) {
			unqueried = append(unqueried, node)
			k--
		}
		if k == 0 {
			return
		}
	}
	return
}

//GetKClosestResponded - returns the k closest nodes that are responsive, if there are
func (snsa *SafeNodeStateArray) GetKClosestResponded(k int) (responded []*NodeState, ok bool) {
	responded = make([]*NodeState, 0)
	snsa.mux.Lock()
	defer snsa.mux.Unlock()
	for _, node := range *snsa.array {
		if snsa.hasResponded(node) {
			responded = append(responded, node)
			k--
		}
		if k == 0 {
			ok = true
			return
		}
	}
	return
}
