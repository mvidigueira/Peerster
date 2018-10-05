package dto

import (
	"sync"
)

//SynchronizationObject - Object for storing rumors, statuses, and synchronizing messages with peers
type SynchronizationObject struct {
	seqID         *SafeCounter
	msgs          map[string]*AtomicMessageList //key = origin
	ownStatusMap  *AtomicStatusMap
	peerStatusMap map[string]*AtomicStatusMap //key = peerAddress
	mux           sync.Mutex
}

//NewSynchronizationObject - for the creation of SynchronizationObjects
func NewSynchronizationObject() *SynchronizationObject {
	return &SynchronizationObject{msgs: make(map[string]*AtomicMessageList), ownStatusMap: NewAtomicStatusMap(), peerStatusMap: make(map[string]*AtomicStatusMap), mux: sync.Mutex{}}
}

//Atomic Message List basic operations

//GetMessageList - atomically retrieve MessageList corresponding to 'origin'
func (so *SynchronizationObject) GetMessageList(origin string) *AtomicMessageList {
	so.mux.Lock()
	defer so.mux.Unlock()
	aml, ok := so.msgs[origin]
	if !ok {
		aml = NewAtomicMessageList()
		so.msgs[origin] = aml
	}
	return aml
}

//GetPeerStatusMap - atomically retrieve StatusMap corresponding to 'peerAddress'
func (so *SynchronizationObject) GetPeerStatusMap(peerAddress string) *AtomicStatusMap {
	so.mux.Lock()
	defer so.mux.Unlock()
	asm, ok := so.peerStatusMap[peerAddress]
	if !ok {
		asm = NewAtomicStatusMap()
		so.peerStatusMap[peerAddress] = asm
	}
	return asm
}

//AddMessage - adds a RumorMessage for storage
func (so *SynchronizationObject) AddMessage(rm RumorMessage) {
	aml := so.GetMessageList(rm.Origin)
	nextID := aml.appendMessage(rm)
	spID := so.ownStatusMap.getSpecialID(rm.Origin)
	spID.setIfGreater(nextID)
}

//UpdateStatus - updates the status map of 'peerAddress' according to a recently received StatusPacket 'status'
func (so *SynchronizationObject) UpdateStatus(peerAddress string, status StatusPacket) {
	peerStatusMap := so.GetPeerStatusMap(peerAddress)
	peerStatusMap.safeUpdate(status)
}

//GetOutdatedMessageByOrigin - returns first message that 'peerAdress' is missing
//or if peer is up to date, returns outdated == false
func (so *SynchronizationObject) GetOutdatedMessageByOrigin(peerAddress, origin string) (rm RumorMessage, outdated bool) {
	peerStatusMap := so.GetPeerStatusMap(peerAddress)
	id, outdated := peerStatusMap.getIDOutdatedForOrigin(so.ownStatusMap, origin)
	if !outdated {
		return
	}
	aml := so.GetMessageList(origin)
	rm = aml.getMessage(id)
	return
}

//HasNewMessages - returns true if peer 'peerAdress' has a higher NextID for some origin
func (so *SynchronizationObject) HasNewMessages(peerAddress string) (hasNew bool) {
	peerStatusMap := so.GetPeerStatusMap(peerAddress)
	hasNew = so.ownStatusMap.hasNewMessages(peerStatusMap)
	return hasNew
}

//GetSendingRights - called when attempting to start synchronizing with 'peer' for messages from 'origin'
func (so *SynchronizationObject) GetSendingRights(peer string, origin string) (success bool) {
	peerStatusMap := so.GetPeerStatusMap(peer)
	return peerStatusMap.acquireSendRightsIfOutdatedForOrigin(so.ownStatusMap, origin)
}

//ReleaseSendingRights - called when attempting to finish synchronizing with 'peer' for messages from 'origin'
func (so *SynchronizationObject) ReleaseSendingRights(peer string, origin string) (success bool) {
	peerStatusMap := so.GetPeerStatusMap(peer)
	return peerStatusMap.releaseSendRightsIfNotOutdatedForOrigin(so.ownStatusMap, origin)
}

//CreateOwnStatusPacket - for the creation of the StatusPacket structure to be sent over the net to other peers
func (so *SynchronizationObject) CreateOwnStatusPacket() *StatusPacket {
	return so.ownStatusMap.toStatusPacket()
}

//CreatePeerStatusPacket - for the creation of the StatusPacket structure from a status map
func (so *SynchronizationObject) CreatePeerStatusPacket(peerAddress string) *StatusPacket {
	peerStatusMap := so.GetPeerStatusMap(peerAddress)
	return peerStatusMap.toStatusPacket()
}
