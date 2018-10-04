package dto

import "sync"

type SyncSendId struct {
	nextID  uint32
	sending bool
	mux     sync.Mutex
}

func NewSyncSendId(startID uint32) *SyncSendId {
	return &SyncSendId{nextID: startID, sending: false, mux: sync.Mutex{}}
}

func (ssId *SyncSendId) GetNextId() uint32 {
	return ssId.nextID
}

func (ssId *SyncSendId) SafeGetNextId() uint32 {
	ssId.mux.Lock()
	defer ssId.mux.Unlock()
	return ssId.GetNextId()
}

func (ssId *SyncSendId) SetIfGreater(new uint32) (modified bool) {
	ssId.mux.Lock()
	defer ssId.mux.Unlock()
	if new > ssId.nextID {
		ssId.nextID = new
		return true
	}
	return false
}

func (ssId *SyncSendId) Lock() {
	ssId.mux.Lock()
}

func (ssId *SyncSendId) Unlock() {
	ssId.mux.Unlock()
}

func (syncOld *SyncSendId) AcquireSendRightsIfOutdatedComparedTo(syncNew *SyncSendId) (success bool) {
	syncOld.Lock()
	defer syncOld.Unlock()
	syncNew.Lock()
	defer syncNew.Unlock()

	if syncOld.sending == false && syncNew.GetNextId() > syncOld.GetNextId() {
		syncOld.sending = true
		return true
	}
	return false
}

func (syncOld *SyncSendId) ReleaseSendRightsIfNotOutdatedComparedTo(syncNew *SyncSendId) (success bool) {
	syncOld.Lock()
	defer syncOld.Unlock()
	syncNew.Unlock()
	defer syncNew.Unlock()

	if syncNew.GetNextId() <= syncOld.GetNextId() {
		syncOld.sending = false
		return true
	}
	return false
}
