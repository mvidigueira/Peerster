package fileparsing

import (
	"sync"
)

//DLChanMap - for synchronizing threads that receive DataReplies with threads interested in them.
//Approximately follows the observer design pattern.
type DLChanMap struct {
	checksumMap sync.Map
}

//NewDLChanMap - for the creation of DLChanMaps
func NewDLChanMap() *DLChanMap {
	return &DLChanMap{checksumMap: sync.Map{}}
}

//AddListener - expresses interest in receiving Data from replies with hash value equal to 'checksum'
//Returns the communication channel to be used to receive said Data ([]byte).
func (dlcm *DLChanMap) AddListener(checksum [32]byte) (cDRdata chan []byte, isNew bool) {
	cData := make(chan []byte)
	c, loaded := dlcm.checksumMap.LoadOrStore(checksum, cData)

	return c.(chan []byte), !loaded
}

//InformListener - sends Data ('data') of replies with hash value equal to 'checksum' to interested threads
//WARNING: do NOT call this concurrently
func (dlcm *DLChanMap) InformListener(checksum [32]byte, data []byte) {
	c, loaded := dlcm.checksumMap.Load(checksum)
	if loaded {
		c.(chan []byte) <- data
		dlcm.checksumMap.Delete(checksum)
	}
}
