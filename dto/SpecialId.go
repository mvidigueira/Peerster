package dto

import (
	"sync"
)

//SpecialID - for limiting synchronization by origin to one thread at a time
type SpecialID struct {
	nextID  uint32
	sending bool
	mux     sync.Mutex
}

//NewSpecialID - for the creation of SpecialIDs
func NewSpecialID(startID uint32) *SpecialID {
	return &SpecialID{nextID: startID, sending: false, mux: sync.Mutex{}}
}

func (spID *SpecialID) lock() {
	spID.mux.Lock()
}

func (spID *SpecialID) unlock() {
	spID.mux.Unlock()
}

func (spID *SpecialID) getNextID() uint32 {
	return spID.nextID
}

func (spID *SpecialID) isOutdatedComparedTo(spIDNew *SpecialID) (oldNextID uint32, outdated bool) {
	spID.lock()
	defer spID.unlock()
	spIDNew.lock()
	defer spIDNew.unlock()

	new := spIDNew.getNextID()
	old := spID.getNextID()

	outdated = false
	if new > old {
		outdated = true
		oldNextID = old
	}

	return
}

func (spID *SpecialID) setIfGreater(new uint32) (modified bool) {
	spID.mux.Lock()
	defer spID.mux.Unlock()
	if new > spID.nextID {
		spID.nextID = new
		return true
	}
	return false
}

//Acquire sending rights

func (spID *SpecialID) acquireSendRightsIfOutdatedComparedTo(spIDNew *SpecialID) (success bool) {
	spID.lock()
	defer spID.unlock()
	spIDNew.lock()
	defer spIDNew.unlock()

	if spID.sending == false && spIDNew.getNextID() > spID.getNextID() {
		spID.sending = true
		//fmt.Printf("New: %d, Old: %d, true\n", spIDNew.getNextID(), spID.getNextID())
		return true
	}
	//fmt.Printf("New: %d, Old: %d, false\n", spIDNew.getNextID(), spID.getNextID())
	return false
}

func (spID *SpecialID) releaseSendRightsIfNotOutdatedComparedTo(spIDNew *SpecialID) (success bool) {
	spID.lock()
	defer spID.unlock()
	spIDNew.lock()
	defer spIDNew.unlock()

	if spIDNew.getNextID() <= spID.getNextID() {
		spID.sending = false
		return true
	}
	return false
}
