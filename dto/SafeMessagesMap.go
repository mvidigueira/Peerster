package dto

import (
	"sync"
)

type SafeMessagesMap struct {
	peersMap sync.Map
}

func NewSafeMessagesMap() *SafeMessagesMap {
	return &SafeMessagesMap{peersMap: sync.Map{}}
}

func (scm *SafeMessagesMap) AddOrigin(origin string) {
	scm.peersMap.Store(origin, sync.Map{})
}

func (scm *SafeMessagesMap) AddMessage(msg *RumorMessage) (isNew bool) {
	pmi, _ := scm.peersMap.LoadOrStore(msg.Origin, &sync.Map{})
	pm := pmi.(*sync.Map)
	var notNew bool
	if msg.ID == 1 {
		_, notNew = pm.LoadOrStore(msg.ID, msg)
	} else if _, isLoad := pm.Load(msg.ID - 1); isLoad {
		_, notNew = pm.LoadOrStore(msg.ID, msg)
	}

	return !notNew
}

func (scm *SafeMessagesMap) GetMessage(origin string, messageId uint32) (*RumorMessage, bool) {
	pmi, _ := scm.peersMap.LoadOrStore(origin, &sync.Map{})
	pm := pmi.(*sync.Map)
	msg, ok := pm.Load(messageId)
	if !ok {
		return nil, ok
	}
	return msg.(*RumorMessage), ok
}

func (scm *SafeMessagesMap) GetNewestID(origin string) uint32 {
	pmi, _ := scm.peersMap.LoadOrStore(origin, &sync.Map{})
	pm := pmi.(*sync.Map)

	maxID := uint32(1)
	idCounter := func(idI interface{}, rmI interface{}) bool {
		id := idI.(uint32)
		if id > maxID {
			maxID = id
		}
		return true
	}
	pm.Range(idCounter)

	return maxID
}

func (scm *SafeMessagesMap) GetOwnStatusPacket() *StatusPacket {
	wants := make([]PeerStatus, 0)
	wantInserter := func(originI interface{}, rmI interface{}) bool {
		origin := originI.(string)
		rm := rmI.(*sync.Map)
		maxID := uint32(0)
		idCounter := func(idI interface{}, rmI interface{}) bool {
			id := idI.(uint32)
			if id > maxID {
				maxID = id
			}
			return true
		}
		rm.Range(idCounter)
		wants = append(wants, PeerStatus{Identifier: origin, NextID: maxID + 1})
		return true
	}
	scm.peersMap.Range(wantInserter)
	return &StatusPacket{Want: wants}
}
