package dto

import (
	"sync"
)

//SafeMessagesMap - safe map for known messages
type SafeMessagesMap struct {
	originsMap sync.Map
}

//NewSafeMessagesMap - for the creation of SafeMessagesMap
func NewSafeMessagesMap() *SafeMessagesMap {
	return &SafeMessagesMap{originsMap: sync.Map{}}
}

//AddMessage - adds a rumor message to the underlying map on the following conditions:
//1) the message is new
//2) if it is not the first message of that origin, the previous message must be present
//returns true if 1) and 2), false otherwise
func (scm *SafeMessagesMap) AddMessage(msg *RumorMessage) (isNew bool) {
	pmi, _ := scm.originsMap.LoadOrStore(msg.Origin, &sync.Map{})
	pm := pmi.(*sync.Map)
	notNew := true
	if msg.ID == 1 {
		_, notNew = pm.LoadOrStore(msg.ID, msg)
	} else if _, isLoad := pm.Load(msg.ID - 1); isLoad {
		_, notNew = pm.LoadOrStore(msg.ID, msg)
	}

	return !notNew
}

//GetMessage - returns the RumorMessage with id 'messageID' from node 'origin'
func (scm *SafeMessagesMap) GetMessage(origin string, messageID uint32) (*RumorMessage, bool) {
	pmi, _ := scm.originsMap.LoadOrStore(origin, &sync.Map{})
	pm := pmi.(*sync.Map)
	msg, ok := pm.Load(messageID)
	if !ok {
		return nil, ok
	}
	return msg.(*RumorMessage), ok
}

//GetOwnStatusPacket - returns a StatusPacket using the information of the known message IDs
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
	scm.originsMap.Range(wantInserter)
	return &StatusPacket{Want: wants}
}
