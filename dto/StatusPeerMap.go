package dto

import (
	"fmt"
	"sync"
)

type SafeChanMap struct {
	peersMap sync.Map
}

func NewSafeChanMap() *SafeChanMap {
	return &SafeChanMap{peersMap: sync.Map{}}
}

func (scm *SafeChanMap) AddPeer(peerAddress string) {
	scm.peersMap.Store(peerAddress, sync.Map{})
}

func (scm *SafeChanMap) AddListener(peerAddress string, messageId uint32) (chan *StatusPacket, bool) {
	cStatus := make(chan *StatusPacket)
	pmi, _ := scm.peersMap.LoadOrStore(peerAddress, &sync.Map{})
	pm := pmi.(*sync.Map)
	_, loaded := pm.LoadOrStore(messageId, cStatus)

	return cStatus, loaded
}

func (scm *SafeChanMap) RemoveListener(peerAddress string, messageId uint32) {
	pmi, _ := scm.peersMap.LoadOrStore(peerAddress, &sync.Map{})
	pm := pmi.(*sync.Map)
	pm.Delete(messageId)

	return
}

func (scm *SafeChanMap) InformListeners(peerAddress string, sp *StatusPacket) {
	pmi, _ := scm.peersMap.LoadOrStore(peerAddress, &sync.Map{})
	pm := pmi.(*sync.Map)

	statusInserter := func(idI interface{}, cStatusI interface{}) bool {
		cStatus := cStatusI.(chan *StatusPacket)
		cStatus <- sp
		return true
	}

	pm.Range(statusInserter)
}

//func (scm *SafeChanMap)

func (sp *StatusPacket) Print() {
	fmt.Println("-- Printing Status Packet --")
	for _, v := range sp.Want {
		fmt.Printf("Identifier: %s, Next ID: %d\n", v.Identifier, v.NextID)
	}
}
