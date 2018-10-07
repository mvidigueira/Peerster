package gossiper

import (
	"fmt"
	"log"
	"math/rand"
	"net"
	"protobuf"
	"strconv"
	"strings"
	"time"

	"github.com/mvidigueira/Peerster/dto"
)

const packetSize = 1024
const debug = false

//Gossiper server responsible for answering client requests and rumor mongering with other gossipers
type Gossiper struct {
	address   string
	name      string
	UseSimple bool

	conn   *net.UDPConn
	connUI *net.UDPConn

	peers         []string
	seqID         *dto.SafeCounter
	statusChanMap *dto.SafeChanMap
	msgMap        *dto.SafeMessagesMap
}

//NewGossiper creates a new gossiper
func NewGossiper(address, name string, UIport int, peers []string, simple bool) *Gossiper {
	gossipAddr, err := net.ResolveUDPAddr("udp4", address)
	clientAddr, err := net.ResolveUDPAddr("udp4", "localhost:"+strconv.Itoa(UIport))
	dto.LogError(err)
	udpConnGossip, err := net.ListenUDP("udp4", gossipAddr)
	udpConnClient, err := net.ListenUDP("udp4", clientAddr)
	dto.LogError(err)

	return &Gossiper{
		address:   address,
		name:      name,
		peers:     peers,
		UseSimple: simple,

		conn:   udpConnGossip,
		connUI: udpConnClient,

		seqID:         dto.NewSafeCounter(),
		statusChanMap: dto.NewSafeChanMap(),
		msgMap:        dto.NewSafeMessagesMap(),
	}
}

//Start starts the gossiper listening routines
func (g *Gossiper) Start() {
	cUI := make(chan *dto.PacketAddressPair)
	go g.receiveClientUDP(cUI)
	go g.clientListenRoutine(cUI)

	cRumor := make(chan *dto.PacketAddressPair)
	cStatus := make(chan *dto.PacketAddressPair)
	go g.receiveExternalUDP(cRumor, cStatus)
	go g.rumorListenRoutine(cRumor)
	g.statusListenRoutine(cStatus)
}

func (g *Gossiper) statusListenRoutine(cStatus chan *dto.PacketAddressPair) {
	for pap := range cStatus {
		printStatusMessage(pap)
		g.addToPeers(pap.SenderAddress)
		peer := pap.GetSenderAddress()
		go g.statusPacketReceived(peer, pap.Packet.Status)
	}
}

func (g *Gossiper) statusPacketReceived(peer string, sp *dto.StatusPacket) {
	g.statusChanMap.InformListeners(peer, sp)
	for _, peerStatus := range sp.Want {
		msg, ok := g.msgMap.GetMessage(peerStatus.Identifier, peerStatus.NextID)
		if ok {
			packet := &dto.GossipPacket{Rumor: msg}
			g.stubbornSend(packet, peer)
			return
		}
	}

}

func (g *Gossiper) rumorMonger(packet *dto.GossipPacket, except string) {
	var peer string
	rand.Seed(1) //time.Now().UnixNano()
	for peer = g.peers[rand.Intn(len(g.peers))]; peer == except; {
		if len(g.peers) == 1 {
			fmt.Printf("No other friends to monger with :(")
			return
		}
	}

	g.stubbornSend(packet, peer)
}

func (g *Gossiper) stubbornSend(packet *dto.GossipPacket, peer string) {
	cStatus, alreadySending := g.statusChanMap.AddListener(peer, packet.GetSeqID())
	if alreadySending {
		return
	}

	fmt.Printf("MONGERING WITH %s\n", peer)
	g.sendUDP(packet, peer)
	t := time.NewTicker(1 * time.Second)
	for {
		select {
		case <-t.C:
			fmt.Printf("MONGERING WITH %s\n", peer)
			g.sendUDP(packet, peer)
		case status := <-cStatus:
			for _, v := range status.Want {
				if v.Identifier == packet.GetOrigin() && v.NextID > packet.GetSeqID() {
					t.Stop()
					return
				}
			}
		}
	}
}

//Client Handling
func (g *Gossiper) clientListenRoutine(cUI chan *dto.PacketAddressPair) {
	for pap := range cUI {
		printClientMessage(pap)
		g.printKnownPeers()
		packet := g.makeGossip(pap.Packet, true)
		if !g.UseSimple {
			g.msgMap.AddMessage(packet.Rumor)
			go g.rumorMonger(packet, "")
		} else {
			g.sendAllPeers(packet, "")
		}

		if debug {
			status := g.msgMap.GetOwnStatusPacket()
			status.Print()
		}

	}
}

//Peer handling
func (g *Gossiper) rumorListenRoutine(cRumor chan *dto.PacketAddressPair) {
	for pap := range cRumor {
		printGossiperMessage(pap)
		g.addToPeers(pap.SenderAddress)
		g.printKnownPeers()
		packet := g.makeGossip(pap.Packet, false)
		if !g.UseSimple {
			isNew := !g.msgMap.AddMessage(packet.Rumor)
			sender := pap.GetSenderAddress()
			g.sendStatusPacket(sender)
			if isNew {
				go g.rumorMonger(packet, sender)
			}
		} else {
			g.sendAllPeers(packet, pap.SenderAddress)
		}

		if debug {
			status := g.msgMap.GetOwnStatusPacket()
			status.Print()
		}
	}
}

func (g *Gossiper) sendStatusPacket(peerAddress string) {
	status := g.msgMap.GetOwnStatusPacket()
	packet := &dto.GossipPacket{Status: status}
	g.sendUDP(packet, peerAddress)
}

//BASIC RECEIVING

func (g *Gossiper) receiveClientUDP(c chan *dto.PacketAddressPair) {
	for {
		packet := &dto.GossipPacket{}
		packetBytes := make([]byte, packetSize)
		g.connUI.ReadFromUDP(packetBytes)
		protobuf.Decode(packetBytes, packet)
		packet.Simple.OriginalName = g.name
		c <- &dto.PacketAddressPair{Packet: packet}
	}
}

func (g *Gossiper) receiveExternalUDP(cRumor, cStatus chan *dto.PacketAddressPair) {
	for {
		packet := &dto.GossipPacket{}
		packetBytes := make([]byte, packetSize)
		n, udpAddr, err := g.conn.ReadFromUDP(packetBytes)
		if err != nil {
			log.Println(err)
			continue
		}
		if n > packetSize {
			log.Panic(fmt.Errorf("Warning: Received packet size (%d) exceeds default value of %d", n, packetSize))
		}
		senderAddress := udpAddr.IP.String() + ":" + strconv.Itoa(udpAddr.Port)
		err = protobuf.Decode(packetBytes, packet)

		pap := &dto.PacketAddressPair{Packet: packet, SenderAddress: senderAddress}
		if err != nil {
			log.Print("Protobuf decoding error\n")
		}
		switch packet.GetUnderlyingType() {
		case "status":
			cStatus <- pap
		default:
			cRumor <- pap
		}

	}
}

//SENDING
func (g *Gossiper) sendAllPeers(packet *dto.GossipPacket, exception string) {
	for _, v := range g.peers {
		if v != exception {
			g.sendUDP(packet, v)
		}
	}
}

func (g *Gossiper) sendUDP(packet *dto.GossipPacket, addr string) {
	udpAddr, err := net.ResolveUDPAddr("udp4", addr)
	dto.LogError(err)
	packetBytes, err := protobuf.Encode(packet)
	dto.LogError(err)
	g.conn.WriteToUDP(packetBytes, udpAddr)
}

//PRINTING
func (g *Gossiper) printKnownPeers() {
	fmt.Println("PEERS " + strings.Join(g.peers, ","))
}

func printStatusMessage(pair *dto.PacketAddressPair) {
	fmt.Printf("STATUS from %v %v\n", pair.GetSenderAddress(), pair.Packet.Status.String())
}

func printClientMessage(pair *dto.PacketAddressPair) {
	fmt.Printf("CLIENT MESSAGE %v\n", pair.GetContents())
}

func printGossiperMessage(pair *dto.PacketAddressPair) {
	switch subtype := pair.Packet.GetUnderlyingType(); subtype {
	case "simple":
		fmt.Printf("SIMPLE MESSAGE origin %v from %v contents %v\n",
			pair.GetOrigin(), pair.GetSenderAddress(), pair.GetContents())
	case "rumor":
		fmt.Printf("RUMOR origin %v from %v ID %v contents %v\n",
			pair.GetOrigin(), pair.GetSenderAddress(), pair.Packet.Rumor.ID, pair.GetContents())
	}
}

//OTHER

func (g *Gossiper) makeGossip(received *dto.GossipPacket, isFromClient bool) (packet *dto.GossipPacket) {
	if g.UseSimple {
		simpleMsg := &dto.SimpleMessage{
			OriginalName:  received.GetOrigin(),
			RelayPeerAddr: g.address,
			Contents:      received.GetContents(),
		}
		packet = &dto.GossipPacket{Simple: simpleMsg, Status: nil}
	} else {
		rumor := &dto.RumorMessage{
			Origin: received.GetOrigin(),
			Text:   received.GetContents(),
		}
		if isFromClient {
			rumor.ID = g.seqID.GetAndIncrement()
		} else {
			rumor.ID = received.GetSeqID()
		}
		packet = &dto.GossipPacket{Rumor: rumor, Status: nil}
	}
	return
}

//PS: HAS CONCURRENCY ISSUES, needs mutex at least if to be kept
//when STATUS messages are implemented, use the necessary map and trash the current "peers" array
//so use the corresponding search method. Current O(N) complexity is very bad:
func (g *Gossiper) addToPeers(peerAddr string) {
	for _, v := range g.peers {
		if peerAddr == v {
			return
		}
	}
	g.peers = append(g.peers, peerAddr)
	return
}
