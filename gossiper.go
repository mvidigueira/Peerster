package main

import (
	"fmt"
	"net"
	"protobuf"
	"strconv"
	"strings"
	"sync"

	"github.com/mvidigueira/Peerster/dto"
)

const packetSize = 1024
const debug = false

type SafeCounter struct {
	v   uint32
	mux sync.Mutex
}

func (sc *SafeCounter) IncrementAndGet() (v uint32) {
	sc.mux.Lock()
	sc.v++
	v = sc.v
	sc.mux.Unlock()
	return
}

type Gossiper struct {
	address   string
	name      string
	peers     []string
	UseSimple bool
	seqID     SafeCounter
	wants     dto.WantsMap

	conn   *net.UDPConn
	connUI *net.UDPConn
}

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
		seqID:     SafeCounter{},
		wants:     dto.WantsMap{},

		conn:   udpConnGossip,
		connUI: udpConnClient,
	}
}

//Start
func (g *Gossiper) start() {
	cUI := make(chan *dto.PacketAddressPair)
	go g.receiveClientUDP(cUI)
	go g.clientListenRoutine(cUI)

	cRumor := make(chan *dto.PacketAddressPair)
	cStatus := make(chan *dto.PacketAddressPair)
	go g.receiveExternalUDP(cRumor, cStatus)
	g.rumorListenRoutine(cRumor)
	//g.statusUpdateRoutine(cStatus)
}

//Client Handling
func (g *Gossiper) clientListenRoutine(cUI chan *dto.PacketAddressPair) {
	for pap := range cUI {
		printClientMessage(pap)
		g.printKnownPeers()
		packet := g.makeGossip(pap.Packet, true)

		dto.AddRumor(&g.wants, packet)
		g.sendAllPeers(packet, "")

		if debug {
			dto.Print(g.wants) //for debugging
			status := dto.ToStatusPacket(&g.wants)
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

		dto.AddRumor(&g.wants, packet)
		g.sendAllPeers(packet, pap.SenderAddress)

		if debug {
			dto.Print(g.wants) //for debugging
			status := dto.ToStatusPacket(&g.wants)
			status.Print()
		}
	}
}

//BASIC RECEIVING

func (g *Gossiper) receiveClientUDP(c chan *dto.PacketAddressPair) {
	packet := &dto.GossipPacket{}
	packetBytes := make([]byte, packetSize)
	for {
		g.connUI.ReadFromUDP(packetBytes)
		protobuf.Decode(packetBytes, packet)
		packet.Simple.OriginalName = g.name
		c <- &dto.PacketAddressPair{Packet: packet}
	}
}

func (g *Gossiper) receiveExternalUDP(cRumor, cStatus chan *dto.PacketAddressPair) {
	packet := &dto.GossipPacket{}
	packetBytes := make([]byte, packetSize)
	for {
		n, udpAddr, err := g.conn.ReadFromUDP(packetBytes)
		dto.LogError(err)
		if n > packetSize {
			dto.LogError(fmt.Errorf("Warning: Received packet size (%d) exceeds default value of %d", n, packetSize))
		}
		senderAddress := udpAddr.IP.String() + ":" + strconv.Itoa(udpAddr.Port)
		protobuf.Decode(packetBytes, packet)
		pap := &dto.PacketAddressPair{Packet: packet, SenderAddress: senderAddress}

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
	fmt.Printf("MONGERING with %v\n", addr)
	g.conn.WriteToUDP(packetBytes, udpAddr)
}

//PRINTING
func (g Gossiper) printKnownPeers() {
	fmt.Println("PEERS " + strings.Join(g.peers, ","))
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
		return &dto.GossipPacket{Simple: simpleMsg}
	} else {
		rumor := &dto.RumorMessage{
			Origin: received.GetOrigin(),
			Text:   received.GetContents(),
		}
		if isFromClient {
			rumor.ID = g.seqID.IncrementAndGet()
		} else {
			rumor.ID = received.GetSeqID()
		}
		return &dto.GossipPacket{Rumor: rumor}
	}
}

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
