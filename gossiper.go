package main

import (
	"fmt"
	"net"
	"protobuf"
	"strings"

	"github.com/mvidigueira/Peerster/dto"
)

const packetSize = 1024

type Gossiper struct {
	address string
	name    string
	peers   []string

	addressUI *net.UDPAddr
	conn      *net.UDPConn
	connUI    *net.UDPConn
}

func NewGossiper(address, name, UIport string, peers []string) *Gossiper {
	gossipAddr, err := net.ResolveUDPAddr("udp4", address)
	clientAddr, err := net.ResolveUDPAddr("udp4", "localhost:"+UIport)
	dto.LogError(err)
	udpConnGossip, err := net.ListenUDP("udp4", gossipAddr)
	udpConnClient, err := net.ListenUDP("udp4", clientAddr)
	dto.LogError(err)

	return &Gossiper{
		address: address,
		conn:    udpConnGossip,
		connUI:  udpConnClient,
		name:    name,
		peers:   peers,
	}
}

//PRINTING
func (g Gossiper) printKnownPeers() {
	fmt.Println("PEERS " + strings.Join(g.peers, ","))
}

func printClientMessage(msg dto.SimpleMessage) {
	fmt.Printf("CLIENT MESSAGE %v\n", msg.Contents)
}

func printGossiperMessage(msg dto.SimpleMessage) {
	fmt.Printf("SIMPLE MESSAGE origin %v from %v contents %v\n", msg.OriginalName, msg.RelayPeerAddr, msg.Contents)
}

//Client Interaction
func (g *Gossiper) clientListenRoutine() {
	c := make(chan dto.SimpleMessage, 10)
	go g.receiveClientUDP(c)
	for i := range c {
		printClientMessage(i)
		g.printKnownPeers()
		i.OriginalName = g.name
		i.RelayPeerAddr = g.address
		g.sendAllPeers(i, "")
	}
}

func (g *Gossiper) receiveClientUDP(c chan dto.SimpleMessage) {
	packet := &dto.GossipPacket{}
	packetBytes := make([]byte, packetSize)
	for {
		g.connUI.ReadFromUDP(packetBytes)
		protobuf.Decode(packetBytes, packet)
		c <- *packet.Simple
	}
}

//External Interaction

//RECEIVING
func (g *Gossiper) externalListenRoutine() {
	c := make(chan dto.SimpleMessage, 10)
	go g.receiveExternalUDP(c)
	for i := range c {
		printGossiperMessage(i)
		g.addToPeers(i.RelayPeerAddr)
		g.printKnownPeers()
		relayer := i.RelayPeerAddr
		i.RelayPeerAddr = g.address
		g.sendAllPeers(i, relayer)
	}
}

func (g *Gossiper) receiveExternalUDP(c chan dto.SimpleMessage) {
	packet := &dto.GossipPacket{}
	packetBytes := make([]byte, packetSize)
	for {
		g.conn.ReadFromUDP(packetBytes)
		protobuf.Decode(packetBytes, packet)
		c <- *packet.Simple
	}
}

//when adding statuses, use the necessary map and trash this array
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

//SENDING
func (g *Gossiper) sendAllPeers(msg dto.SimpleMessage, exception string) {
	for _, v := range g.peers {
		if v != exception {
			g.sendUDP(msg, v)
		}
	}
}

func (g *Gossiper) sendUDP(msg dto.SimpleMessage, addr string) {
	udpAddr, err := net.ResolveUDPAddr("udp4", addr)
	packet := &dto.GossipPacket{Simple: &msg}
	packetBytes, err := protobuf.Encode(packet)
	dto.LogError(err)
	g.conn.WriteToUDP(packetBytes, udpAddr)
}
