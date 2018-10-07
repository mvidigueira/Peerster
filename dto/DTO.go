package dto

import (
	"fmt"
	"log"
	"time"
)

//Common error logging

//LogError - common function for fatal errors
func LogError(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

//PacketAddressPair - Packet and Address pair structure
type PacketAddressPair struct {
	Packet        *GossipPacket
	SenderAddress string
}

//GossipPacket - protocol structure to be serialized and sent between peers
type GossipPacket struct {
	Simple *SimpleMessage
	Rumor  *RumorMessage
	Status *StatusPacket
}

//SimpleMessage - subtype of GossipPacket to be used between peers when running in 'simple' mode
type SimpleMessage struct {
	OriginalName  string
	RelayPeerAddr string
	Contents      string
}

//RumorMessage - subtype of GossipPacket to be used between peers normally
type RumorMessage struct {
	Origin string
	ID     uint32
	Text   string
}

//StatusPacket - subtype of GossipPacket used for acknowledgments and Anti-entropy
type StatusPacket struct {
	Want []PeerStatus
}

//PeerStatus - represents the desired 'NextID' from origin specified by 'Identifier'
type PeerStatus struct {
	Identifier string
	NextID     uint32
}

func (sp *StatusPacket) String() string {
	if sp == nil {
		return "nil"
	}
	str := ""
	for _, v := range sp.Want {
		str = str + fmt.Sprintf("peer %s nextID %d ", v.Identifier, v.NextID)
	}
	return str
}

//GetSenderAddress - returns the sender address of the PacketAddresspair
func (pap *PacketAddressPair) GetSenderAddress() (address string) {
	switch subtype := pap.Packet.GetUnderlyingType(); subtype {
	case "simple":
		address = pap.Packet.Simple.RelayPeerAddr
	default:
		address = pap.SenderAddress
	}
	return
}

//GetContents - returns the message (text) contents of the PacketAddresspair
func (pap *PacketAddressPair) GetContents() string {
	return pap.Packet.GetContents()
}

//GetOrigin - returns the origin name of the PacketAddresspair
func (pap *PacketAddressPair) GetOrigin() string {
	return pap.Packet.GetOrigin()
}

//GetContents - returns the message (text) contents of the gossip packet
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

//GetOrigin - returns the origin name of the gossip packet
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

//GetSeqID - returns the sequence id of the gossip packet
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

//GetUnderlyingType - returns the underlying type of the gossip packet, or the empty string in case of no subtype
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

//GossipPacketError - general error type for gossip packets
type GossipPacketError struct {
	When time.Time
	What string
}

func (e *GossipPacketError) Error() string {
	return fmt.Sprintf("at %v, %s",
		e.When, e.What)
}

func (gp GossipPacket) String() string {
	return fmt.Sprintf("GossipPacket printing\nRumor: %s\nStatus: %s\n", gp.Rumor, gp.Status)
}

func (rm *RumorMessage) String() string {
	if rm == nil {
		return "nil"
	}
	str := fmt.Sprintf("id: %d, origin: %s", rm.ID, rm.Origin)
	return str
}
