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

//IncrementAndGet - atomically increments and returns the value stored
func (sc *SafeCounter) IncrementAndGet() (v uint32) {
	sc.mux.Lock()
	sc.v++
	v = sc.v
	sc.mux.Unlock()
	return
}
