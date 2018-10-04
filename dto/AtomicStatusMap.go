package dto

import "sync"

type AtomicStatusMap struct {
	nextIDmap map[string]*SyncSendId
	mux       sync.Mutex
}

func NewAtomicStatusMap() *AtomicStatusMap {
	return &AtomicStatusMap{nextIDmap: make(map[string]*SyncSendId), mux: sync.Mutex{}}
}

func (asm AtomicStatusMap) GetSyncID(origin string) *SyncSendId {
	asm.mux.Lock()
	defer asm.mux.Unlock()
	vSync, ok := asm.nextIDmap[origin]
	if !ok {
		vSync := NewSyncSendId(1)
		asm.nextIDmap[origin] = vSync
		return vSync
	}
	return vSync
}

func (asm AtomicStatusMap) SafeGetNextId(origin string) uint32 {
	sync := asm.GetSyncID(origin)
	return sync.SafeGetNextId()
}

func (smOld *AtomicStatusMap) IsOutdatedForOrigin(smNew *AtomicStatusMap, origin string) bool {
	vNew := smNew.nextIDmap[origin].GetNextId()
	vOldS, ok := smOld.nextIDmap[origin]
	if !ok || vNew > vOldS.GetNextId() {
		return true
	}
	return false
}

func (asm AtomicStatusMap) SafeUpdate(sp StatusPacket) (modified bool) {
	modified = false
	asm.mux.Lock()
	defer asm.mux.Unlock()
	for _, pair := range sp.Want {
		value, ok := asm.nextIDmap[pair.Identifier]
		if !ok {
			asm.nextIDmap[pair.Identifier] = NewSyncSendId(value.GetNextId())
			modified = true
		} else {
			modified = (modified || value.SetIfGreater(pair.NextID))
		}
	}
	return
}

func (smOld *AtomicStatusMap) AcquireSendRightsIfOutdatedForOrigin(smNew *AtomicStatusMap, origin string) (success bool) {
	syncOld := smOld.GetSyncID(origin)
	syncNew := smNew.GetSyncID(origin)

	return syncOld.AcquireSendRightsIfOutdatedComparedTo(syncNew)
}

func (smOld *AtomicStatusMap) ReleaseSendRightsIfNotOutdatedForOrigin(smNew *AtomicStatusMap, origin string) (success bool) {
	syncOld := smOld.GetSyncID(origin)
	syncNew := smNew.GetSyncID(origin)

	return syncOld.ReleaseSendRightsIfNotOutdatedComparedTo(syncNew)
}

/*
	peerStatusMap.Lock()
	defer peerStatusMap.Unlock()
	so.ownStatusMap.Lock()
	defer so.ownStatusMap.Unlock()
*/

func (asm AtomicStatusMap) Lock() {
	asm.mux.Lock()
}

func (asm AtomicStatusMap) Unlock() {
	asm.mux.Unlock()
}

/*
func (smOld *AtomicStatusMap) IsOutdated(smNew *AtomicStatusMap) bool {
	for kn, vn := range smNew.nextIDmap {
		vo, ok := smOld.nextIDmap[kn]
		if !ok || vn.GetNextId() > vo.GetNextId() {
			return true
		}
	}
	return false
}
*/
/*
func (smOld *AtomicStatusMap) IsOutdatedForOrigin(smNew *AtomicStatusMap, origin string) (wants uint32, isOutdated bool) {
	vNew := smNew.nextIDmap[origin].GetNextId()
	vOldS, ok := smOld.nextIDmap[origin]
	var vOld uint32
	if !ok {
		smOld.nextIDmap[origin] = NewSyncSendId(1)
		return 1, true
	} else if vOld = vOldS.GetNextId(); vNew > vOld {
		return vOld, true
	}
	return vOld, false
}
*/
