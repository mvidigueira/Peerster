package dto

import "sync"

//SafeMessageArray - safe array for rumor messages
type SafeMessageArray struct {
	array *[]RumorMessage
	mux   sync.Mutex
}

//NewSafeMessageArray - for the creation of an empty SafeMessageArray
func NewSafeMessageArray() *SafeMessageArray {
	array := make([]RumorMessage, 0)
	return &SafeMessageArray{array: &array, mux: sync.Mutex{}}
}

//AppendToArray - appends to the array irrespective of element uniqueness
func (sma *SafeMessageArray) AppendToArray(item RumorMessage) {
	sma.mux.Lock()
	defer sma.mux.Unlock()
	*sma.array = append(*sma.array, item)
}

//GetArrayCopyAndDelete - returns the current elements of the array and resets it
func (sma *SafeMessageArray) GetArrayCopyAndDelete() []RumorMessage {
	sma.mux.Lock()
	oldArray := *sma.array
	array := make([]RumorMessage, 0)
	sma.array = &array
	sma.mux.Unlock()
	return oldArray
}
