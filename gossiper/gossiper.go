package gossiper

import (
	"log"
	"math/rand"
	"net"
	"protobuf"
	"strconv"

	"github.com/mvidigueira/Peerster/dto"
	"github.com/mvidigueira/Peerster/routing"
)

var packetSize = 1024

const globalSeed = 2 //time.Now().UnixNano()

//Gossiper server responsible for answering client requests and rumor mongering with other gossipers
type Gossiper struct {
	address   string
	name      string
	UseSimple bool

	conn   *net.UDPConn
	connUI *net.UDPConn

	peers         *dto.SafeStringArray
	seqIDCounter  *dto.SafeCounter
	statusChanMap *dto.SafeChanMap
	quitChanMap   *dto.QuitChanMap
	msgMap        *dto.SafeMessagesMap

	latestMessages *dto.SafeMessageArray

	routingTable *routing.SafeRoutingTable
	rtimeout     int
}

//NewGossiper creates a new gossiper
func NewGossiper(address, name string, UIport int, peers []string, simple bool, rtimeout int) *Gossiper {
	gossipAddr, err := net.ResolveUDPAddr("udp4", address)
	dto.LogError(err)
	clientAddr, err := net.ResolveUDPAddr("udp4", "localhost:"+strconv.Itoa(UIport))
	dto.LogError(err)
	udpConnGossip, err := net.ListenUDP("udp4", gossipAddr)
	dto.LogError(err)
	udpConnClient, err := net.ListenUDP("udp4", clientAddr)
	dto.LogError(err)

	return &Gossiper{
		address:   address,
		name:      name,
		UseSimple: simple,

		conn:   udpConnGossip,
		connUI: udpConnClient,

		peers:         dto.NewSafeStringArray(peers),
		seqIDCounter:  dto.NewSafeCounter(),
		statusChanMap: dto.NewSafeChanMap(),
		quitChanMap:   dto.NewQuitChanMap(),
		msgMap:        dto.NewSafeMessagesMap(),

		routingTable: routing.NewSafeRoutingTable(),

		latestMessages: dto.NewSafeMessageArray(),
		rtimeout:       rtimeout,
	}
}

//Start starts the gossiper listening routines
func (g *Gossiper) Start() {
	rand.Seed(globalSeed)

	cRumor := make(chan *dto.PacketAddressPair)
	go g.rumorListenRoutine(cRumor)
	cStatus := make(chan *dto.PacketAddressPair)
	go g.statusListenRoutine(cStatus)

	cPrivate := make(chan *dto.PacketAddressPair)
	go g.privateMessageListenRoutine(cPrivate)

	go g.receiveExternalUDP(cRumor, cStatus, cPrivate)
	go g.antiEntropy()

	go g.periodicRouteRumor() //DSDV

	cUI := make(chan *dto.PacketAddressPair)
	go g.clientListenRoutine(cUI)
	cUIPM := make(chan *dto.PacketAddressPair)
	go g.clientPMListenRoutine(cUI)

	g.receiveClientUDP(cUI, cUIPM)
}

//addToPeers - adds a peer (ip:port) to the list of known peers
func (g *Gossiper) addToPeers(peerAddr string) (isNew bool) {
	return g.peers.AppendUniqueToArray(peerAddr)
}

//addMessage - adds a message (rumor message) to the map of known messages and latest messages
func (g *Gossiper) addMessage(rm *dto.RumorMessage) bool {
	isNew := g.msgMap.AddMessage(rm)
	if isNew && dto.IsChatRumor(rm) {
		g.latestMessages.AppendToArray(*rm)
	}
	return isNew
}

//sendUDP - sends a gossip packet to peer at 'addr' via UDP, using te gossiper's external connection
func (g *Gossiper) sendUDP(packet *dto.GossipPacket, addr string) {
	udpAddr, err := net.ResolveUDPAddr("udp4", addr)
	dto.LogError(err)
	packetBytes, err := protobuf.Encode(packet)
	dto.LogError(err)
	g.conn.WriteToUDP(packetBytes, udpAddr)
}

//sendAllPeers - sends a gossip packet to all peers except 'exception'
func (g *Gossiper) sendAllPeers(packet *dto.GossipPacket, exception string) {
	for _, v := range stringArrayDifference(g.peers.GetArrayCopy(), []string{exception}) {
		g.sendUDP(packet, v)
	}
}

//sendStatusPacket - sends a status packet to 'peerAdress'.
//The status packet's information is based on the gossiper's currently known messages
func (g *Gossiper) sendStatusPacket(peerAddress string) {
	status := g.msgMap.GetOwnStatusPacket()
	packet := &dto.GossipPacket{Status: status}
	g.sendUDP(packet, peerAddress)
}

//receiveClientUDP - receives gossip packets from CLIENTS and forwards them to the provided channel,
//setting the origin in the process
func (g *Gossiper) receiveClientUDP(cRumoring, cPMing chan *dto.PacketAddressPair) {
	for {
		packet := &dto.GossipPacket{}
		packetBytes := make([]byte, packetSize)
		n, _, err := g.connUI.ReadFromUDP(packetBytes)
		if n > packetSize {
			packetSize = packetSize + 1024
		}
		if err != nil {
			log.Println(err)
			continue
		}
		err = protobuf.Decode(packetBytes, packet)
		switch packet.GetUnderlyingType() {
		case "simple":
			if packet.GetContents() == "" {
				continue
			}
			packet.Simple.OriginalName = g.name
			cRumoring <- &dto.PacketAddressPair{Packet: packet}
		case "rumor":
			if packet.GetContents() == "" {
				continue
			}
			packet.Rumor.Origin = g.name
			cRumoring <- &dto.PacketAddressPair{Packet: packet}
		case "private":
			if packet.GetContents() == "" {
				continue
			}
			cPMing <- &dto.PacketAddressPair{Packet: packet}
		default:
			log.Println("Unrecognized message type. Ignoring...")
		}
	}
}

//receiveExternalUDP - receives gossip packets from PEERS and forwards them to the appropriate channel
//among those provided, depending on whether they are rumor, simple or status packets
func (g *Gossiper) receiveExternalUDP(cRumor, cStatus, cPrivate chan *dto.PacketAddressPair) {
	for {
		packet := &dto.GossipPacket{}
		packetBytes := make([]byte, packetSize)
		n, udpAddr, err := g.conn.ReadFromUDP(packetBytes)
		if n > packetSize {
			packetSize = packetSize + 1024
		}
		if err != nil {
			log.Println(err)
			continue
		}
		protobuf.Decode(packetBytes, packet)
		senderAddress := udpAddr.IP.String() + ":" + strconv.Itoa(udpAddr.Port)
		pap := &dto.PacketAddressPair{Packet: packet, SenderAddress: senderAddress}

		switch packet.GetUnderlyingType() {
		case "status":
			if g.UseSimple {
				log.Println("Running on 'simple' mode. Ignoring status message from " + senderAddress + "...")
			} else {
				cStatus <- pap
			}
		case "rumor":
			if g.UseSimple {
				log.Println("Running on 'simple' mode. Ignoring rumor message from " + senderAddress + "...")
			} else {
				cRumor <- pap
			}
		case "private":
			if g.UseSimple {
				log.Println("Running on 'simple' mode. Ignoring private message from " + senderAddress + "...")
			} else {
				cPrivate <- pap
			}
		case "simple":
			if g.UseSimple {
				cRumor <- pap
			} else {
				log.Println("Running on normal mode. Ignoring simple message from " + senderAddress + "...")
			}
		default:
			log.Println("Unrecognized message type. Ignoring message from " + senderAddress + "...")
		}
	}
}

//makeGossip - prepares a gossip packet to be sent.
//Subtype (simple vs rumor) depends on mode being used (flag -simple)
//Attributes new seqID if message originated from this gossipe
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
			rumor.ID = g.seqIDCounter.GetAndIncrement()
		} else {
			rumor.ID = received.GetSeqID()
		}
		packet = &dto.GossipPacket{Rumor: rumor}
	}
	return
}

//statusListenRoutine - deals with status packets from all sources, adding unknown sources to the peers list.
//Creates and forwards them to status listening routines for that specific peer (concurrent)
func (g *Gossiper) statusListenRoutine(cStatus chan *dto.PacketAddressPair) {
	for pap := range cStatus {
		peer := pap.GetSenderAddress()
		g.addToPeers(peer) //fix concurrency issues here
		g.printKnownPeers()

		c, isNew := g.statusChanMap.AddOrGetPeerStatusListener(peer)
		if isNew {
			go g.peerStatusListenRoutine(peer, c)
		}
		c <- pap.Packet.Status
	}
}

//clientListenRoutine - deals with new messages (simple packets) from clients
func (g *Gossiper) clientListenRoutine(cUI chan *dto.PacketAddressPair) {
	for pap := range cUI {
		printClientMessage(pap)
		g.printKnownPeers()

		packet := g.makeGossip(pap.Packet, true)
		if !g.UseSimple {
			g.addMessage(packet.Rumor)
			go g.rumorMonger(packet, "")
		} else {
			g.sendAllPeers(packet, "")
		}
	}
}

//rumorListenRoutine - deals with rumor/simple packets from other peers
func (g *Gossiper) rumorListenRoutine(cRumor chan *dto.PacketAddressPair) {
	for pap := range cRumor {
		g.addToPeers(pap.GetSenderAddress())

		if dto.IsChatPacket(pap.Packet) || pap.Packet.GetUnderlyingType() == "simple" {
			printGossiperMessage(pap)
		}
		g.printKnownPeers()

		packet := g.makeGossip(pap.Packet, false)
		if !g.UseSimple {
			g.updateDSDV(pap) //routing

			isNew := g.addMessage(packet.Rumor)
			g.sendStatusPacket(pap.GetSenderAddress())
			if isNew {
				go g.rumorMonger(packet, pap.GetSenderAddress())
			}
		} else {
			g.sendAllPeers(packet, pap.GetSenderAddress())
		}
	}
}
