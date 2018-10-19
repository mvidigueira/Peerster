package routing

import "sync"

//SafeRoutingTableEntry - simple int counter protected by mutex
type SafeRoutingTableEntry struct {
	seqNum  uint32
	address string
	mux     sync.RWMutex
}

//NewSafeRoutingTableEntry - for the creation of SafeRoutingTableEntry
func NewSafeRoutingTableEntry(seqNum uint32, address string) *SafeRoutingTableEntry {
	return &SafeRoutingTableEntry{seqNum: seqNum, address: address, mux: sync.RWMutex{}}
}

//Update - atomically updates the entry if seqNum is more recent that current one
func (srte *SafeRoutingTableEntry) Update(seqNum uint32, address string) (updated bool) {
	srte.mux.Lock()
	defer srte.mux.Unlock()
	if seqNum > srte.seqNum {
		srte.seqNum = seqNum
		srte.address = address
		return true
	}
	return false
}

//GetAddress - atomically return current value of address in entry
func (srte *SafeRoutingTableEntry) GetAddress() string {
	srte.mux.RLock()
	defer srte.mux.RUnlock()
	return srte.address
}
