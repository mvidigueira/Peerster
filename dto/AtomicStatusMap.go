package dto

import "sync"

//AtomicStatusMap - vector clock for nextIDs according to origin
type AtomicStatusMap struct {
	nextIDmap map[string]*SpecialID
	mux       sync.Mutex
}

//NewAtomicStatusMap - for the creation of Atomic Status Maps
func NewAtomicStatusMap() *AtomicStatusMap {
	return &AtomicStatusMap{nextIDmap: make(map[string]*SpecialID), mux: sync.Mutex{}}
}

func (asm *AtomicStatusMap) getSpecialID(origin string) *SpecialID {
	asm.mux.Lock()
	defer asm.mux.Unlock()
	spID, ok := asm.nextIDmap[origin]
	if !ok {
		spID = NewSpecialID(1)
		asm.nextIDmap[origin] = spID
		return spID
	}
	return spID
}

func (asm *AtomicStatusMap) safeUpdate(sp StatusPacket) {
	for _, pair := range sp.Want {
		spID := asm.getSpecialID(pair.Identifier)
		spID.setIfGreater(pair.NextID)
	}
	return
}

func (asm *AtomicStatusMap) getIDOutdatedForOrigin(asmNew *AtomicStatusMap, origin string) (NextID uint32, outdated bool) {
	spIDOld := asm.getSpecialID(origin)
	spIDNew := asmNew.getSpecialID(origin)

	return spIDOld.isOutdatedComparedTo(spIDNew)
}

func (asm *AtomicStatusMap) acquireSendRightsIfOutdatedForOrigin(asmNew *AtomicStatusMap, origin string) (success bool) {
	spIDOld := asm.getSpecialID(origin)
	spIDNew := asmNew.getSpecialID(origin)

	return spIDOld.acquireSendRightsIfOutdatedComparedTo(spIDNew)
}

func (asm *AtomicStatusMap) releaseSendRightsIfNotOutdatedForOrigin(smNew *AtomicStatusMap, origin string) (success bool) {
	spIDOld := asm.getSpecialID(origin)
	spIDNew := smNew.getSpecialID(origin)

	return spIDOld.releaseSendRightsIfNotOutdatedComparedTo(spIDNew)
}

func (asm *AtomicStatusMap) toStatusPacket() *StatusPacket {
	asm.mux.Lock()
	defer asm.mux.Unlock()
	i := 0
	wantsList := make([]PeerStatus, len(asm.nextIDmap))
	for origin, spID := range asm.nextIDmap {
		wantsList[i] = PeerStatus{Identifier: origin, NextID: spID.getNextID()}
	}
	return &StatusPacket{Want: wantsList}
}
