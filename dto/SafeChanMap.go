package dto

import (
	"sync"
)

type SafeChanMap struct {
	peersMap sync.Map
}

func NewSafeChanMap() *SafeChanMap {
	return &SafeChanMap{peersMap: sync.Map{}}
}

func (scm *SafeChanMap) AddOrGetPeerStatusListener(peerAddress string) (chan *StatusPacket, bool) {
	cStatus := make(chan *StatusPacket)
	c, loaded := scm.peersMap.LoadOrStore(peerAddress, cStatus)

	return c.(chan *StatusPacket), !loaded
}
