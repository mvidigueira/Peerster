package dto

import (
	"fmt"
	"log"
	"sync"
	"time"
)

//Common error logging

func LogError(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

//Common structures

type PacketAddressPair struct {
	Packet        *GossipPacket
	SenderAddress string
}
type GossipPacket struct {
	Simple *SimpleMessage
	Rumor  *RumorMessage
	Status *StatusPacket
}

type SimpleMessage struct {
	OriginalName  string
	RelayPeerAddr string
	Contents      string
}
type RumorMessage struct {
	Origin string
	ID     uint32
	Text   string
}

type StatusPacket struct {
	Want []PeerStatus
}
type PeerStatus struct {
	Identifier string
	NextID     uint32
}

//Extra Functions

func (pap *PacketAddressPair) GetSenderAddress() (address string) {
	switch subtype := pap.Packet.GetUnderlyingType(); subtype {
	case "simple":
		address = pap.Packet.Simple.RelayPeerAddr
	default:
		address = pap.SenderAddress
	}
	return
}

func (pap *PacketAddressPair) GetContents() string {
	return pap.Packet.GetContents()
}

func (pap *PacketAddressPair) GetOrigin() string {
	return pap.Packet.GetOrigin()
}

func (g *GossipPacket) GetContents() (contents string) {
	switch subtype := g.GetUnderlyingType(); subtype {
	case "simple":
		contents = g.Simple.Contents
	case "rumor":
		contents = g.Rumor.Text
	case "status":
		err := &GossipPacketError{When: time.Now(), What: "Can't extract contents from a STATUS message"}
		LogError(err)
	default:
		err := &GossipPacketError{When: time.Now(), What: "Gossip packet has no non-nil sub struct"}
		LogError(err)
	}
	return
}

func (g *GossipPacket) GetOrigin() (origin string) {
	switch subtype := g.GetUnderlyingType(); subtype {
	case "simple":
		origin = g.Simple.OriginalName
	case "rumor":
		origin = g.Rumor.Origin
	case "status":
		err := &GossipPacketError{When: time.Now(), What: "Can't extract origin name from a STATUS message"}
		LogError(err)
	default:
		err := &GossipPacketError{When: time.Now(), What: "Gossip packet has no non-nil sub struct"}
		LogError(err)
	}
	return
}

func (g *GossipPacket) GetSeqID() (id uint32) {
	switch subtype := g.GetUnderlyingType(); subtype {
	case "simple":
		err := &GossipPacketError{When: time.Now(), What: "Can't extract ID from a SIMPLE message"}
		LogError(err)
	case "rumor":
		id = g.Rumor.ID
	case "status":
		err := &GossipPacketError{When: time.Now(), What: "Can't extract ID from a STATUS message"}
		LogError(err)
	default:
		err := &GossipPacketError{When: time.Now(), What: "Gossip packet has no non-nil sub struct"}
		LogError(err)
	}
	return
}

func (g *GossipPacket) GetUnderlyingType() (subtype string) {
	if g.Simple != nil {
		subtype = "simple"
	} else if g.Rumor != nil {
		subtype = "rumor"
	} else if g.Status != nil {
		subtype = "status"
	} else {
		subtype = ""
	}
	return
}

type GossipPacketError struct {
	When time.Time
	What string
}

func (e *GossipPacketError) Error() string {
	return fmt.Sprintf("at %v, %s",
		e.When, e.What)
}

//STATUS

type MsgMap struct {
	rumors map[string][]RumorMessage
	mux    sync.Mutex
}

type StatusMap struct {
	m       map[string]uint32
	sending bool
	mux     sync.Mutex
}

func (wm *MsgMap) AddRumor(packet *GossipPacket) {
	rumor := packet.Rumor
	if rumor != nil {
		v, ok := wm.rumors[rumor.Origin]
		if !ok {
			v = make([]RumorMessage, 0)
			wm.rumors[rumor.Origin] = v
		}
		if rumor.ID == uint32(len(v)) { //only add next message
			wm.rumors[rumor.Origin] = append(v, *rumor)
		}
	}
}

func (wm *MsgMap) ToStatusPacket() *StatusPacket {
	statusList := make([]PeerStatus, len(wm.rumors))
	i := 0
	wm.mux.Lock()
	for k, v := range wm.rumors {
		statusList[i] = PeerStatus{Identifier: k, NextID: uint32(len(v) + 1)}
		i++
	}
	wm.mux.Unlock()
	return &StatusPacket{Want: statusList}
}

func (sm StatusMap) UpdateStatusMap(sp *StatusPacket) (modified bool) {
	sm.mux.Lock()
	modified = false
	for _, pair := range sp.Want {
		value, ok := sm.m[pair.Identifier]
		if !ok || pair.NextID > value {
			sm.m[pair.Identifier] = value
			modified = true
		}
	}
	if modified {
		sm.sending = true
	}
	sm.mux.Unlock()
	return
}

//just for testing

func Print(wm *MsgMap) {
	fmt.Println("-- Printing Msg Map --")
	wm.mux.Lock()
	for key, value := range wm.rumors {
		fmt.Printf("Identifier: %s\n", key)
		for _, msg := range value {
			fmt.Printf("Message ID: %d, Message: %v\n", msg.ID, msg.Text)
		}
	}
	wm.mux.Unlock()
}

//just for testing

func (sp *StatusPacket) Print() {
	fmt.Println("-- Printing Status Packet --")
	for _, v := range sp.Want {
		fmt.Printf("Identifier: %s, Next ID: %d\n", v.Identifier, v.NextID)
	}
}

func (sp *StatusPacket) String() string {
	str := ""
	for _, v := range sp.Want {
		str = str + fmt.Sprintf("peer %s nextID %d ", v.Identifier, v.NextID)
	}
	return str
}
