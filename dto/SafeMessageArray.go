package dto

import "sync"

type SafeMessageArray struct {
	array *[]RumorMessage
	mux   sync.Mutex
}

func NewSafeMessageArray() *SafeMessageArray {
	array := make([]RumorMessage, 0)
	return &SafeMessageArray{array: &array, mux: sync.Mutex{}}
}

func (sma *SafeMessageArray) AppendToArray(item RumorMessage) {
	sma.mux.Lock()
	defer sma.mux.Unlock()
	*sma.array = append(*sma.array, item)
}

func (sma *SafeMessageArray) GetArrayCopyAndDelete() []RumorMessage {
	sma.mux.Lock()
	oldArray := *sma.array
	array := make([]RumorMessage, 0)
	sma.array = &array
	sma.mux.Unlock()
	return oldArray
}
