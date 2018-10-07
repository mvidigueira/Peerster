package dto

import (
	"sync"
)

//AtomicMessageList - RumorMessage Array protected by mutex
type AtomicMessageList struct {
	knownMsgs []RumorMessage
	mux       sync.Mutex
}

//NewAtomicMessageList - for the creation of Atomic Message Lists
func NewAtomicMessageList() *AtomicMessageList {
	return &AtomicMessageList{knownMsgs: make([]RumorMessage, 0), mux: sync.Mutex{}}
}

func (aml *AtomicMessageList) getNextID() uint32 {
	aml.mux.Lock()
	defer aml.mux.Unlock()
	return uint32(len(aml.knownMsgs) + 1)
}

//no bounds checking
func (aml *AtomicMessageList) getMessage(id uint32) RumorMessage {
	aml.mux.Lock()
	defer aml.mux.Unlock()
	return aml.knownMsgs[id-1]
}

func (aml *AtomicMessageList) appendMessage(msg RumorMessage) (length uint32) {
	aml.mux.Lock()
	defer aml.mux.Unlock()
	if msg.ID == uint32(len(aml.knownMsgs)+1) {
		aml.knownMsgs = append(aml.knownMsgs, msg)
	}
	return uint32(len(aml.knownMsgs) + 1)
}
