package dto

import "sync"

type AtomicMessageList struct {
	knownMsgs []RumorMessage
	mux       sync.Mutex
}

func NewAtomicMessageList() *AtomicMessageList {
	return &AtomicMessageList{knownMsgs: make([]RumorMessage, 0), mux: sync.Mutex{}}
}

func (aml AtomicMessageList) getNextID() uint32 {
	aml.mux.Lock()
	defer aml.mux.Unlock()
	return uint32(len(aml.knownMsgs) + 1)
}

//no bounds checking
func (aml AtomicMessageList) getMessage(id uint32) RumorMessage {
	aml.mux.Lock()
	defer aml.mux.Unlock()
	return aml.knownMsgs[id-1]
}

func (aml AtomicMessageList) appendMessage(msg RumorMessage) {
	aml.mux.Lock()
	defer aml.mux.Unlock()
	aml.knownMsgs = append(aml.knownMsgs, msg)
}

func (aml AtomicMessageList) Lock() {
	aml.mux.Lock()
}

func (aml AtomicMessageList) Unlock() {
	aml.mux.Unlock()
}
