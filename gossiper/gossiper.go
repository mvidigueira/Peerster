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
const debug = true

//Gossiper server responsible for answering client requests and rumor mongering with other gossipers
type Gossiper struct {
	address   string
	name      string
	peers     []string
	UseSimple bool
	seqID     *dto.SafeCounter
	syncObj   *dto.SynchronizationObject

	conn   *net.UDPConn
	connUI *net.UDPConn
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
		seqID:     dto.NewSafeCounter(),
		syncObj:   dto.NewSynchronizationObject(),

		conn:   udpConnGossip,
		connUI: udpConnClient,
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

//Client Handling
func (g *Gossiper) clientListenRoutine(cUI chan *dto.PacketAddressPair) {
	for pap := range cUI {
		printClientMessage(pap)
		g.printKnownPeers()
		packet := g.makeGossip(pap.Packet, true)
		if !g.UseSimple {
			g.syncObj.AddMessage(*packet.Rumor)
			pick := rand.Intn(len(g.peers))
			fmt.Printf("Peer chosen: %s\n", g.peers[pick])
			go g.synchronize(g.peers[pick], packet.Rumor.Origin)
		} else {
			g.sendAllPeers(packet, "")
		}
		/*
			if debug {
				dto.Print(&g.msgMap) //for debugging
				status := g.msgMap.ToStatusPacket()
				status.Print()
			}
		*/
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
			g.syncObj.AddMessage(*packet.Rumor)
			g.sendStatusPacket(pap.GetSenderAddress())
			pick := rand.Intn(len(g.peers))
			fmt.Printf("Peer chosen: %s\n", g.peers[pick])
			go g.synchronize(g.peers[pick], packet.Rumor.Origin)
		} else {
			g.sendAllPeers(packet, pap.SenderAddress)
		}

		/*
			if debug {
				dto.Print(&g.msgMap) //for debugging
				status := g.msgMap.ToStatusPacket()
				status.Print()
			}
		*/
	}
}

func (g *Gossiper) statusListenRoutine(cStatus chan *dto.PacketAddressPair) {
	for pap := range cStatus {
		printStatusMessage(pap)
		g.addToPeers(pap.SenderAddress)
		peer := pap.GetSenderAddress()
		g.syncObj.UpdateStatus(peer, *pap.Packet.Status)
		println("Status Updated")
		go g.synchronize(peer, pap.GetOrigin())
	}
}

func (g *Gossiper) synchronize(peerAddress, origin string) {
	if g.syncObj.GetSendingRights(peerAddress, origin) {
		fmt.Printf("MONGERING with %v\n", peerAddress)
		for !g.syncObj.ReleaseSendingRights(peerAddress, origin) {
			fmt.Printf("SENDING RUMOR to %v\n", peerAddress)
			rm, _ := g.syncObj.GetOutdatedMessageByOrigin(peerAddress, origin)
			packet := &dto.GossipPacket{Rumor: &rm}
			g.sendUDP(packet, peerAddress)
			time.Sleep(15 * time.Second)
		}
		fmt.Printf("STOPPED MONGERING with %v\n", peerAddress)
	}
	if g.syncObj.HasNewMessages(peerAddress) {
		fmt.Printf("Current status: %s\n", g.syncObj.CreateOwnStatusPacket().String())
		fmt.Printf("Their status: %s\n", g.syncObj.CreatePeerStatusPacket(peerAddress).String())
		g.sendStatusPacket(peerAddress)
	}

}

func (g *Gossiper) sendStatusPacket(peerAddress string) {
	status := g.syncObj.CreateOwnStatusPacket()
	fmt.Printf("Current status: %s\n", status.String())
	packet := &dto.GossipPacket{Status: status}
	g.sendUDP(packet, peerAddress)
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
		if err != nil {
			log.Println(err)
			continue
		}
		if n > packetSize {
			log.Panic(fmt.Errorf("Warning: Received packet size (%d) exceeds default value of %d", n, packetSize))
		}
		senderAddress := udpAddr.IP.String() + ":" + strconv.Itoa(udpAddr.Port)
		protobuf.Decode(packetBytes, packet)
		pap := &dto.PacketAddressPair{Packet: packet, SenderAddress: senderAddress}
		fmt.Printf("RECEIVING BARE METAL TYPE: %s; FROM: %s\n", packet.GetUnderlyingType(), senderAddress)

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
	fmt.Printf("SENDING BARE METAL TYPE: %s; TO: %s\n", packet.GetUnderlyingType(), addr)
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
		packet = &dto.GossipPacket{Simple: simpleMsg}
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
		packet = &dto.GossipPacket{Rumor: rumor}
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
