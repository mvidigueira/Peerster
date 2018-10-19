package routing

import (
	"sync"
)

//SafeRoutingTable - safe map for known messages
type SafeRoutingTable struct {
	originsMap sync.Map
}

//NewSafeRoutingTable - for the creation of SafeMessagesMap
func NewSafeRoutingTable() *SafeRoutingTable {
	return &SafeRoutingTable{originsMap: sync.Map{}}
}

//UpdateEntry - updates entry for msg's origin, using its seqID and the sender's address
func (srt *SafeRoutingTable) UpdateEntry(seqID uint32, origin string, addressSender string) (updated bool) {
	srte := NewSafeRoutingTableEntry(seqID, addressSender)
	srtei, loaded := srt.originsMap.LoadOrStore(origin, srte)
	if loaded {
		srte := srtei.(*SafeRoutingTableEntry)
		return srte.Update(seqID, addressSender)
	}
	return true
}

//GetNextHop - returns the address of the next hop peer for 'origin'
func (srt *SafeRoutingTable) GetNextHop(origin string) (address string, ok bool) {
	srtei, ok := srt.originsMap.Load(origin)
	if !ok {
		return
	}
	srte := srtei.(*SafeRoutingTableEntry)
	address = srte.GetAddress()
	return
}
