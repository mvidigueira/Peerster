package dht

import (
	"sync"
)

//StorageMap - for storing the DHT data.
type StorageMap struct {
	ByteMap sync.Map
}

//NewStorageMap - for the creation of StorageMap.
func NewStorageMap() *StorageMap {
	return &StorageMap{ByteMap: sync.Map{}}
}

//Store - stores data in map, indexed by its key.
func (sm *StorageMap) Store(key [IDByteSize]byte, data []byte) (isNew bool) {
	_, loaded := sm.ByteMap.LoadOrStore(key, data)
	return !loaded
}

//Delete - deletes data indexed by 'key' from storage.
func (sm *StorageMap) Delete(key [IDByteSize]byte) {
	sm.ByteMap.Delete(key)
}

//Retrieve - returns data indexed by 'key', if present.
func (sm *StorageMap) Retrieve(key [IDByteSize]byte) (data []byte, ok bool) {
	d, ok := sm.ByteMap.Load(key)
	data = d.([]byte)
	return
}
