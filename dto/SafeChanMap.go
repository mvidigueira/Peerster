package dto

import (
	"sync"
)

//SafeChanMap - simple map of peerAdress to channel
type SafeChanMap struct {
	peersMap sync.Map
}

//NewSafeChanMap - for the creation of SafeChanMap
func NewSafeChanMap() *SafeChanMap {
	return &SafeChanMap{peersMap: sync.Map{}}
}

//AddOrGetPeerStatusListener - pretty much a LoadOrStore encapsulating the channel creation
//returns the channel being used, and true if it is a new channel (false otherwise)
func (scm *SafeChanMap) AddOrGetPeerStatusListener(peerAddress string) (chan *StatusPacket, bool) {
	cStatus := make(chan *StatusPacket)
	c, loaded := scm.peersMap.LoadOrStore(peerAddress, cStatus)

	return c.(chan *StatusPacket), !loaded
}
