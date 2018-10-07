package dto

import (
	"sync"
)

type QuitChanMap struct {
	peersMap sync.Map
}

func NewQuitChanMap() *QuitChanMap {
	return &QuitChanMap{peersMap: sync.Map{}}
}

func (scm *QuitChanMap) AddListener(peerAddress string, origin string, messageId uint32) (chan int, bool) {
	quit := make(chan int)
	pmi, _ := scm.peersMap.LoadOrStore(peerAddress, &sync.Map{})
	pm := pmi.(*sync.Map)
	omi, _ := pm.LoadOrStore(origin, &sync.Map{})
	om := omi.(*sync.Map)
	v, loaded := om.LoadOrStore(messageId, quit)

	return v.(chan int), loaded
}

func (scm *QuitChanMap) InformListeners(peerAddress string, sp *StatusPacket) (allStopped bool) {
	pmi, _ := scm.peersMap.LoadOrStore(peerAddress, &sync.Map{})
	pm := pmi.(*sync.Map)

	allStopped = true

	for _, v := range sp.Want {
		omi, _ := pm.LoadOrStore(v.Identifier, &sync.Map{})
		om := omi.(*sync.Map)

		quitter := func(idI interface{}, quitI interface{}) bool {
			id := idI.(uint32)
			quit := quitI.(chan int)
			if v.NextID > id {
				close(quit)
				om.Delete(id)
			} else {
				allStopped = false
			}

			return true
		}

		om.Range(quitter)
	}

	return allStopped
}
