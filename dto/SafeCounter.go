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
	defer sc.mux.Unlock()
	v = sc.v
	sc.v++
	return
}

//SwapIfLess - atomically set new value if old < new
func (sc *SafeCounter) SwapIfLess(new uint32) {
	sc.mux.Lock()
	defer sc.mux.Unlock()
	if sc.v < new {
		sc.v = new
	}
}
