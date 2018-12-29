package dht

import (
	"sync"
)

//ChanMap - for synchronizing threads that receive Messages with threads interested in them.
//Approximately follows the observer design pattern.
type ChanMap struct {
	NonceMessageMap sync.Map
}

//NewChanMap - for the creation of NewChanMaps
func NewChanMap() *ChanMap {
	return &ChanMap{NonceMessageMap: sync.Map{}}
}

//AddListener - expresses interest in receiving Messages with nonce equal to 'nonce'
//Returns the communication channel to be used to receive said Messages.
func (cm *ChanMap) AddListener(nonce uint64) (cMessage chan *Message, isNew bool) {
	cMsg := make(chan *Message, 2) // must be non blocking
	c, loaded := cm.NonceMessageMap.LoadOrStore(nonce, cMsg)

	return c.(chan *Message), !loaded
}

//RemoveListener - revokes interest in Messages with nonce equal to 'nonce'
func (cm *ChanMap) RemoveListener(nonce uint64) {
	cm.NonceMessageMap.Delete(nonce)
}

//InformListener - sends Message ('data') of replies with nonce equal to 'nonce' to interested threads
//WARNING: do NOT call this concurrently
func (cm *ChanMap) InformListener(nonce uint64, message *Message) {
	c, loaded := cm.NonceMessageMap.Load(nonce)
	if loaded {
		c.(chan *Message) <- message
		cm.NonceMessageMap.Delete(nonce)
	}
}
