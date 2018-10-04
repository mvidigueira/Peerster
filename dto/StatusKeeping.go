package dto

import "sync"

type SynchronizationObject struct {
	msgs          map[string]*AtomicMessageList
	ownStatusMap  *AtomicStatusMap
	peerStatusMap map[string]*AtomicStatusMap
	mux           sync.Mutex
}

func NewSynchronizationObject() SynchronizationObject {
	return SynchronizationObject{msgs: make(map[string]*AtomicMessageList), ownStatusMap: NewAtomicStatusMap(), peerStatusMap: make(map[string]*AtomicStatusMap), mux: sync.Mutex{}}
}

func (so SynchronizationObject) GetSendingRights(peer string, origin string) (success bool) {
	so.mux.Lock()
	peerStatusMap := so.peerStatusMap[peer]
	so.mux.Unlock()

	return peerStatusMap.AcquireSendRightsIfOutdatedForOrigin(so.ownStatusMap, origin)
}

func (so SynchronizationObject) ReleaseSendingRights(peer string, origin string) (success bool) {
	so.mux.Lock()
	peerStatusMap := so.peerStatusMap[peer]
	so.mux.Unlock()

	return peerStatusMap.ReleaseSendRightsIfNotOutdatedForOrigin(so.ownStatusMap, origin)
}
