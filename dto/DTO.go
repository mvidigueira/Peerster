package dto

import (
	"fmt"
	"log"
	"time"
)

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

//PeerStatus - represents the desired 'NextID' from origin specified by 'Identifier'
type PeerStatus struct {
	Identifier string
	NextID     uint32
}

//StatusPacket - subtype of GossipPacket used for acknowledgments and Anti-entropy
type StatusPacket struct {
	Want []PeerStatus
}

func (sp *StatusPacket) String() string {
	str := ""
	for _, v := range sp.Want {
		str = str + fmt.Sprintf("peer %s nextID %d ", v.Identifier, v.NextID)
	}
	return str
}

//PrivateMessage - subtype of GossipPacket used for private messages between peers
type PrivateMessage struct {
	Origin      string
	ID          uint32
	Text        string
	Destination string
	HopLimit    uint32
}

//DecrementHopCount - decrements hop count of PrivateMessage by 1
func (pm *PrivateMessage) DecrementHopCount() (shouldSend bool) {
	pm.HopLimit--
	return (pm.HopLimit > 0)
}

//ToRumorMessage - type conversion
func (pm PrivateMessage) ToRumorMessage() RumorMessage {
	rm := RumorMessage{
		Origin: pm.Origin,
		ID:     pm.ID,
		Text:   pm.Text,
	}
	return rm
}

//GossipPacket - protocol structure to be serialized and sent between peers
type GossipPacket struct {
	Simple  *SimpleMessage
	Rumor   *RumorMessage
	Status  *StatusPacket
	Private *PrivateMessage
}

//GetUnderlyingType - returns the underlying type of the gossip packet, or the empty string in case of no subtype
func (g *GossipPacket) GetUnderlyingType() (subtype string) {
	if g.Simple != nil {
		subtype = "simple"
	} else if g.Rumor != nil {
		subtype = "rumor"
	} else if g.Status != nil {
		subtype = "status"
	} else if g.Private != nil {
		subtype = "private"
	} else {
		subtype = ""
	}
	return
}

//IsChatPacket - returns true if the rumor message is a chat message, false otherwise
func IsChatPacket(packet *GossipPacket) bool {
	return packet.Rumor != nil && packet.Rumor.Text != ""
}

//IsChatRumor - returns true if the rumor message is a chat message, false otherwise
func IsChatRumor(rm *RumorMessage) bool {
	return rm.Text != ""
}

//GetOrigin - returns the origin name of the gossip packet
func (g *GossipPacket) GetOrigin() (origin string) {
	switch subtype := g.GetUnderlyingType(); subtype {
	case "simple":
		origin = g.Simple.OriginalName
	case "rumor":
		origin = g.Rumor.Origin
	case "private":
		origin = g.Private.Origin
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
	case "private":
		id = g.Private.ID
	case "status":
		err := &GossipPacketError{When: time.Now(), What: "Can't extract ID from a STATUS message"}
		LogError(err)
	default:
		err := &GossipPacketError{When: time.Now(), What: "Gossip packet has no non-nil sub struct"}
		LogError(err)
	}
	return
}

//GetContents - returns the message (text) contents of the gossip packet
func (g *GossipPacket) GetContents() (contents string) {
	switch subtype := g.GetUnderlyingType(); subtype {
	case "simple":
		contents = g.Simple.Contents
	case "rumor":
		contents = g.Rumor.Text
	case "private":
		contents = g.Private.Text
	case "status":
		err := &GossipPacketError{When: time.Now(), What: "Can't extract contents from a STATUS message"}
		LogError(err)
	default:
		err := &GossipPacketError{When: time.Now(), What: "Gossip packet has no non-nil sub struct"}
		LogError(err)
	}
	return
}

//GetHopLimit - returns the hop-limit of the gossip packet
func (g *GossipPacket) GetHopLimit() (id uint32) {
	switch subtype := g.GetUnderlyingType(); subtype {
	case "simple":
		err := &GossipPacketError{When: time.Now(), What: "Can't extract hop-limit from a SIMPLE message"}
		LogError(err)
	case "rumor":
		err := &GossipPacketError{When: time.Now(), What: "Can't extract hop-limit from a RUMOR message"}
		LogError(err)
	case "private":
		id = g.Private.HopLimit
	case "status":
		err := &GossipPacketError{When: time.Now(), What: "Can't extract hop-limit from a STATUS message"}
		LogError(err)
	default:
		err := &GossipPacketError{When: time.Now(), What: "Gossip packet has no non-nil sub struct"}
		LogError(err)
	}
	return
}

//GetDestination - returns the destination (peer name) of the PacketAddresspair
func (g *GossipPacket) GetDestination() (dest string) {
	switch subtype := g.GetUnderlyingType(); subtype {
	case "simple":
		err := &GossipPacketError{When: time.Now(), What: "Can't extract destination from a SIMPLE message"}
		LogError(err)
	case "rumor":
		err := &GossipPacketError{When: time.Now(), What: "Can't extract destination from a RUMOR message"}
		LogError(err)
	case "private":
		dest = g.Private.Destination
	case "status":
		err := &GossipPacketError{When: time.Now(), What: "Can't extract destination from a STATUS message"}
		LogError(err)
	default:
		err := &GossipPacketError{When: time.Now(), What: "Gossip packet has no non-nil sub struct"}
		LogError(err)
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

//PacketAddressPair - Packet and Address pair structure
type PacketAddressPair struct {
	Packet        *GossipPacket
	SenderAddress string
}

//GetOrigin - returns the origin name of the PacketAddresspair
func (pap *PacketAddressPair) GetOrigin() string {
	return pap.Packet.GetOrigin()
}

//GetSeqID - returns the sequence ID of the PacketAddresspair
func (pap *PacketAddressPair) GetSeqID() uint32 {
	return pap.Packet.GetSeqID()
}

//GetContents - returns the message (text) contents of the PacketAddresspair
func (pap *PacketAddressPair) GetContents() string {
	return pap.Packet.GetContents()
}

//GetHopLimit - returns the hop-limit of the PacketAddresspair
func (pap *PacketAddressPair) GetHopLimit() uint32 {
	return pap.Packet.GetHopLimit()
}

//GetDestination - returns the destination (peer name) of the PacketAddresspair
func (pap *PacketAddressPair) GetDestination() string {
	return pap.Packet.GetDestination()
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

//LogError - common function for fatal errors
func LogError(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
