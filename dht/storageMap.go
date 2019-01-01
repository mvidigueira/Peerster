package dht

import (
	"fmt"
	"protobuf"
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
func (sm *StorageMap) Store(key TypeID, data []byte) (isNew bool) {
	_, loaded := sm.ByteMap.LoadOrStore(key, data)
	return !loaded
}

//Delete - deletes data indexed by 'key' from storage.
func (sm *StorageMap) Delete(key TypeID) {
	sm.ByteMap.Delete(key)
}

//Retrieve - returns data indexed by 'key', if present.
func (sm *StorageMap) Retrieve(key TypeID) (data []byte, ok bool) {
	d, ok := sm.ByteMap.Load(key)
	if !ok {
		return
	}
	data = d.([]byte)
	return
}

// Special store operation for keyword -> url mappings
// We need this extra function since we want to support PUT operations. It might not be the cleanest solution to wrap
// the keywords and URLs in struct but I could not think about any other way.
// The definiton of KeywordToURLMap will move to the webcrawler package but it is in a seperate branch at the moment. I will move this when
// I merge the two branches.

type KeywordToURLMap struct {
	Keyword string
	Urls    map[string]int // websiteUrl:NumberOfOccurances
}

func (sm *StorageMap) StoreKeywordToURLMapping(key TypeID, data []byte) (ok bool) {

	newKeywordToUrlMap := &KeywordToURLMap{}
	err := protobuf.Decode(data, newKeywordToUrlMap)
	if err != nil {
		fmt.Println("Error, trying to save non KeyWordToURLMapping data type.")
		return false
	}

	dat, found := sm.ByteMap.Load(key)
	if !found {
		// Append to an empty key -> Store
		sm.ByteMap.Store(key, data)
		return true
	}

	oldKeywordToUrlMap := &KeywordToURLMap{}
	err = protobuf.Decode(dat.([]byte), oldKeywordToUrlMap)
	if err != nil {
		fmt.Println("Error, trying to decode old KeywordToURLMap struct.")
		return false
	}

	// Make sure that that the keywords correspond, they should but just to make sure.
	if oldKeywordToUrlMap.Keyword != newKeywordToUrlMap.Keyword {
		fmt.Println("Error, Keywords does not correspond.")
		return false
	}

	// Append new urls to old struct
	for k, _ := range newKeywordToUrlMap.Urls {
		val, found := oldKeywordToUrlMap.Urls[k]
		if found {
			oldKeywordToUrlMap.Urls[k] = val + 1
			continue
		}
		oldKeywordToUrlMap.Urls[k] = 1
	}

	newData, err := protobuf.Encode(oldKeywordToUrlMap)
	if err != nil {
		fmt.Println("error trying to encode KeywordToURLMapping struct.")
		return false
	}

	sm.ByteMap.Store(key, newData)

	return true
}
