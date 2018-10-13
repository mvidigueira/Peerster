package dto

import (
	"sync"
)

//QuitChanMap - maps the termination channels by peer, origin and messageID
//concurrency safe (insertion and deletion)
type QuitChanMap struct {
	peersMap sync.Map
}

//NewQuitChanMap - for the creation of a new QuitChanMap
func NewQuitChanMap() *QuitChanMap {
	return &QuitChanMap{peersMap: sync.Map{}}
}

//AddListener - creates a unique entry for a specific peer, origin and message id (if not existent)
//and returns the termination channel
func (scm *QuitChanMap) AddListener(peerAddress string, origin string, messageID uint32) (chan int, bool) {
	quit := make(chan int)
	pmi, _ := scm.peersMap.LoadOrStore(peerAddress, &sync.Map{})
	pm := pmi.(*sync.Map)
	omi, _ := pm.LoadOrStore(origin, &sync.Map{})
	om := omi.(*sync.Map)
	v, loaded := om.LoadOrStore(messageID, quit)

	return v.(chan int), loaded
}

func (scm *QuitChanMap) DeleteListener(peerAddress string, origin string, messageID uint32) {
	pmi, _ := scm.peersMap.LoadOrStore(peerAddress, &sync.Map{})
	pm := pmi.(*sync.Map)
	omi, _ := pm.LoadOrStore(origin, &sync.Map{})
	om := omi.(*sync.Map)
	om.Delete(messageID)
}

//InformListeners - informs of termination and eliminates entries of listeners that are no longer
//required to keep stubbornly sending messages (because the status packet NextID is higher than their messageID)
//returns true if there aren't any processes/listeners remaining sending to that peer (false otherwise)
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
				om.Delete(id) //removes listener
			} else {
				allStopped = false
			}

			return true
		}

		om.Range(quitter)
	}

	return allStopped
}
