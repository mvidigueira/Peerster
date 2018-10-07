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

func (scm *SafeMessagesMap) AddMessage(msg *RumorMessage) bool {
	pmi, _ := scm.peersMap.LoadOrStore(msg.Origin, &sync.Map{})
	pm := pmi.(*sync.Map)
	_, wasLoaded := pm.LoadOrStore(msg.ID, msg)

	return wasLoaded
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

func (scm *SafeMessagesMap) GetOwnStatusPacket() *StatusPacket {
	wants := make([]PeerStatus, 0)
	wantInserter := func(originI interface{}, rmI interface{}) bool {
		origin := originI.(string)
		rm := rmI.(*sync.Map)
		maxID := uint32(1)
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
