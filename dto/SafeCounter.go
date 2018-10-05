package dto

import "sync"

//SafeCounter - simple int counter protected by mutex
type SafeCounter struct {
	v   uint32
	mux sync.Mutex
}

//NewSafeCounter - for the creation of SafeCounters
func NewSafeCounter() *SafeCounter {
	return &SafeCounter{v: 1, mux: sync.Mutex{}}
}

//GetAndIncrement - atomically increments and returns the value stored
func (sc *SafeCounter) GetAndIncrement() (v uint32) {
	sc.mux.Lock()
	v = sc.v
	sc.v++
	sc.mux.Unlock()
	return
}
