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

//DataRequest - subtype of GossipPacket used for file requests between peers
type DataRequest struct {
	Origin      string
	Destination string
	HopLimit    uint32
	HashValue   []byte
}

//DecrementHopCount - decrements hop count of DataRequest by 1
func (dreq *DataRequest) DecrementHopCount() (shouldSend bool) {
	dreq.HopLimit--
	return (dreq.HopLimit > 0)
}

//DataReply - subtype of GossipPacket used for file replies between peers
type DataReply struct {
	Origin      string
	Destination string
	HopLimit    uint32
	HashValue   []byte
	Data        []byte
}

//DecrementHopCount - decrements hop count of DataReply by 1
func (drep *DataReply) DecrementHopCount() (shouldSend bool) {
	drep.HopLimit--
	return (drep.HopLimit > 0)
}

//GossipPacket - protocol structure to be serialized and sent between peers
type GossipPacket struct {
	Simple      *SimpleMessage
	Rumor       *RumorMessage
	Status      *StatusPacket
	Private     *PrivateMessage
	DataRequest *DataRequest
	DataReply   *DataReply
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
	} else if g.DataRequest != nil {
		subtype = "datarequest"
	} else if g.DataReply != nil {
		subtype = "datareply"
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
	case "status":
		err := &GossipPacketError{When: time.Now(), What: "Can't extract origin name from a STATUS message"}
		LogError(err)
	case "private":
		origin = g.Private.Origin
	case "datarequest":
		origin = g.DataRequest.Origin
	case "datareply":
		origin = g.DataReply.Origin
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
	case "private":
		id = g.Private.ID
	case "datarequest":
		err := &GossipPacketError{When: time.Now(), What: "Can't extract ID from a DATA REQUEST message"}
		LogError(err)
	case "datareply":
		err := &GossipPacketError{When: time.Now(), What: "Can't extract ID from a DATA REPLY message"}
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
	case "status":
		err := &GossipPacketError{When: time.Now(), What: "Can't extract contents from a STATUS message"}
		LogError(err)
	case "private":
		contents = g.Private.Text
	case "datarequest":
		err := &GossipPacketError{When: time.Now(), What: "Can't extract contents from a DATA REQUEST message"}
		LogError(err)
	case "datareply":
		err := &GossipPacketError{When: time.Now(), What: "Can't extract contents from a DATA REPLY message"}
		LogError(err)
	default:
		err := &GossipPacketError{When: time.Now(), What: "Gossip packet has no non-nil sub struct"}
		LogError(err)
	}
	return
}

//GetHopLimit - returns the hop-limit of the gossip packet
func (g *GossipPacket) GetHopLimit() (limit uint32) {
	switch subtype := g.GetUnderlyingType(); subtype {
	case "simple":
		err := &GossipPacketError{When: time.Now(), What: "Can't extract hop-limit from a SIMPLE message"}
		LogError(err)
	case "rumor":
		err := &GossipPacketError{When: time.Now(), What: "Can't extract hop-limit from a RUMOR message"}
		LogError(err)
	case "status":
		err := &GossipPacketError{When: time.Now(), What: "Can't extract hop-limit from a STATUS message"}
		LogError(err)
	case "private":
		limit = g.Private.HopLimit
	case "datarequest":
		limit = g.DataRequest.HopLimit
	case "datareply":
		limit = g.DataReply.HopLimit
	default:
		err := &GossipPacketError{When: time.Now(), What: "Gossip packet has no non-nil sub struct"}
		LogError(err)
	}
	return
}

//GetDestination - returns the destination (peer name) of the gossip packet
func (g *GossipPacket) GetDestination() (dest string) {
	switch subtype := g.GetUnderlyingType(); subtype {
	case "simple":
		err := &GossipPacketError{When: time.Now(), What: "Can't extract destination from a SIMPLE message"}
		LogError(err)
	case "rumor":
		err := &GossipPacketError{When: time.Now(), What: "Can't extract destination from a RUMOR message"}
		LogError(err)
	case "status":
		err := &GossipPacketError{When: time.Now(), What: "Can't extract destination from a STATUS message"}
		LogError(err)
	case "private":
		dest = g.Private.Destination
	case "datarequest":
		dest = g.DataRequest.Destination
	case "datareply":
		dest = g.DataReply.Destination
	default:
		err := &GossipPacketError{When: time.Now(), What: "Gossip packet has no non-nil sub struct"}
		LogError(err)
	}
	return
}

//GetHashValue - returns the hash value ([]byte) of the gossip packet
func (g *GossipPacket) GetHashValue() (hash []byte) {
	switch subtype := g.GetUnderlyingType(); subtype {
	case "simple":
		err := &GossipPacketError{When: time.Now(), What: "Can't extract hash value from a SIMPLE message"}
		LogError(err)
	case "rumor":
		err := &GossipPacketError{When: time.Now(), What: "Can't extract hash value from a RUMOR message"}
		LogError(err)
	case "status":
		err := &GossipPacketError{When: time.Now(), What: "Can't extract hash value from a STATUS message"}
		LogError(err)
	case "private":
		err := &GossipPacketError{When: time.Now(), What: "Can't extract hash value from a PRIVATE message"}
		LogError(err)
	case "datarequest":
		hash = g.DataRequest.HashValue
	case "datareply":
		hash = g.DataReply.HashValue
	default:
		err := &GossipPacketError{When: time.Now(), What: "Gossip packet has no non-nil sub struct"}
		LogError(err)
	}
	return
}

//GetData - returns the data ([]byte) of the gossip packet
func (g *GossipPacket) GetData() (data []byte) {
	switch subtype := g.GetUnderlyingType(); subtype {
	case "simple":
		err := &GossipPacketError{When: time.Now(), What: "Can't extract data from a SIMPLE message"}
		LogError(err)
	case "rumor":
		err := &GossipPacketError{When: time.Now(), What: "Can't extract data from a RUMOR message"}
		LogError(err)
	case "status":
		err := &GossipPacketError{When: time.Now(), What: "Can't extract data from a STATUS message"}
		LogError(err)
	case "private":
		err := &GossipPacketError{When: time.Now(), What: "Can't extract data from a PRIVATE message"}
		LogError(err)
	case "datarequest":
		err := &GossipPacketError{When: time.Now(), What: "Can't extract data from a DATA REQUEST message"}
		LogError(err)
	case "datareply":
		data = g.DataReply.Data
	default:
		err := &GossipPacketError{When: time.Now(), What: "Gossip packet has no non-nil sub struct"}
		LogError(err)
	}
	return
}

//DecrementHopCount - returns true if positive hop count after decrement
func (g *GossipPacket) DecrementHopCount() (shouldSend bool) {
	switch subtype := g.GetUnderlyingType(); subtype {
	case "simple":
		err := &GossipPacketError{When: time.Now(), What: "Can't decrement hop count from a SIMPLE message"}
		LogError(err)
	case "rumor":
		err := &GossipPacketError{When: time.Now(), What: "Can't decrement hop count from a RUMOR message"}
		LogError(err)
	case "status":
		err := &GossipPacketError{When: time.Now(), What: "Can't decrement hop count from a STATUS message"}
		LogError(err)
	case "private":
		shouldSend = g.Private.DecrementHopCount()
	case "datarequest":
		shouldSend = g.DataRequest.DecrementHopCount()
	case "datareply":
		shouldSend = g.DataReply.DecrementHopCount()
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

//GetOrigin - returns the origin name of the PacketAddressPair
func (pap *PacketAddressPair) GetOrigin() string {
	return pap.Packet.GetOrigin()
}

//GetSeqID - returns the sequence ID of the PacketAddressPair
func (pap *PacketAddressPair) GetSeqID() uint32 {
	return pap.Packet.GetSeqID()
}

//GetContents - returns the message (text) contents of the PacketAddressPair
func (pap *PacketAddressPair) GetContents() string {
	return pap.Packet.GetContents()
}

//GetHopLimit - returns the hop-limit of the PacketAddressPair
func (pap *PacketAddressPair) GetHopLimit() uint32 {
	return pap.Packet.GetHopLimit()
}

//GetDestination - returns the destination (peer name) of the PacketAddressPair
func (pap *PacketAddressPair) GetDestination() string {
	return pap.Packet.GetDestination()
}

//GetHashValue - returns the hash value ([]byte) of the PacketAddressPair
func (pap *PacketAddressPair) GetHashValue() []byte {
	return pap.Packet.GetHashValue()
}

//GetData - returns the data ([]byte) of the PacketAddressPair
func (pap *PacketAddressPair) GetData() []byte {
	return pap.Packet.GetData()
}

//DecrementHopCount - returns true if positive hop count after decrement
func (pap *PacketAddressPair) DecrementHopCount() bool {
	return pap.Packet.DecrementHopCount()
}

//GetSenderAddress - returns the sender address of the PacketAddressPair
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

//FileToShare - protocol structure sent from client to gossiper with file name to share
type FileToShare struct {
	FileName string
}

//GetFileName - returns the name of the file
func (fts *FileToShare) GetFileName() string {
	return fts.FileName
}

//FileToDownload - protocol structure sent from client to gossiper.
//Filename is the name the file is saved as.
//Metahash identifies the file to download.
//Origin identifies the peer that has the file.
type FileToDownload struct {
	FileName string
	Origin   string
	Metahash [32]byte
}

//GetFileName - returns the name of the file
func (ftd *FileToDownload) GetFileName() string {
	return ftd.FileName
}

//GetOrigin - returns the name of the peer that has the file
func (ftd *FileToDownload) GetOrigin() string {
	return ftd.Origin
}

//GetMetahash - returns the hash of the file's metafile
func (ftd *FileToDownload) GetMetahash() [32]byte {
	return ftd.Metahash
}

//ClientRequest - protocol structure to be serialized and sent from client to gossiper
type ClientRequest struct {
	Packet       *GossipPacket
	FileShare    *FileToShare
	FileDownload *FileToDownload
}

//GetUnderlyingType - returns the underlying type of the client request, or the empty string in case of no subtype
func (cr *ClientRequest) GetUnderlyingType() (subtype string) {
	if cr.Packet != nil {
		subtype = cr.Packet.GetUnderlyingType()
	} else if cr.FileShare != nil {
		subtype = "fileShare"
	} else if cr.FileDownload != nil {
		subtype = "fileDownload"
	} else {
		subtype = ""
	}
	return
}
