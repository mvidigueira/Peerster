package fileparsing

import (
	"sync"
)

type DLChanMap struct {
	checksumMap sync.Map
}

func NewDLChanMap() *DLChanMap {
	return &DLChanMap{checksumMap: sync.Map{}}
}

func (dlcm *DLChanMap) AddListener(checksum [32]byte) (cDRdata chan []byte, isNew bool) {
	cData := make(chan []byte)
	c, loaded := dlcm.checksumMap.LoadOrStore(checksum, cData)

	return c.(chan []byte), !loaded
}

//dont call this concurrently
func (dlcm *DLChanMap) InformListener(checksum [32]byte, data []byte) {
	c, loaded := dlcm.checksumMap.Load(checksum)
	if loaded {
		c.(chan []byte) <- data
		dlcm.checksumMap.Delete(checksum)
	}
}
