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
	quitChanMap   *dto.QuitChanMap
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
		quitChanMap:   dto.NewQuitChanMap(),
		msgMap:        dto.NewSafeMessagesMap(),
	}
}

//Start starts the gossiper listening routines
func (g *Gossiper) Start() {
	rand.Seed(2) //time.Now().UnixNano()
	cUI := make(chan *dto.PacketAddressPair)
	go g.receiveClientUDP(cUI)
	go g.clientListenRoutine(cUI)

	cRumor := make(chan *dto.PacketAddressPair)
	cStatus := make(chan *dto.PacketAddressPair)
	go g.receiveExternalUDP(cRumor, cStatus)
	go g.rumorListenRoutine(cRumor)
	go g.statusListenRoutine(cStatus)
	g.antiEntropy()
}

func (g *Gossiper) antiEntropy() {
	t := time.NewTicker(1 * time.Second)
	for {
		select {
		case <-t.C:
			if len(g.peers) > 0 {
				//fmt.Printf("Anti entropy kawaii chan....................")
				peer := g.peers[rand.Intn(len(g.peers))]
				g.sendStatusPacket(peer)
			}
		}
	}
}

func (g *Gossiper) statusListenRoutine(cStatus chan *dto.PacketAddressPair) {
	for pap := range cStatus {
		printStatusMessage(pap)
		peer := pap.GetSenderAddress()
		g.addToPeers(peer) //fix concurrency issues here
		g.printKnownPeers()

		c, isNew := g.statusChanMap.AddListener(peer)
		if isNew {
			go g.peerListenRoutine(peer, c)
		}
		c <- pap.Packet.Status
	}
}

func (g *Gossiper) peerListenRoutine(peerAddress string, cStatus chan *dto.StatusPacket) {
	//print("Listen routine started for " + peerAddress)
	for {
		select {
		case sp := <-cStatus:
			allStopped := g.quitChanMap.InformListeners(peerAddress, sp)

			hasNewMessages := false
			mySp := g.msgMap.GetOwnStatusPacket()
			myExtraInfo := make([]*dto.PeerStatus, len(mySp.Want))
			for i, ps1 := range mySp.Want {
				myExtraInfo[i] = &dto.PeerStatus{Identifier: ps1.Identifier, NextID: 1}
				for _, ps2 := range sp.Want { //not enough, what about origins not in the "Want" array?
					if ps1.Identifier == ps2.Identifier {
						if ps1.NextID > ps2.NextID {
							myExtraInfo[i].NextID = ps2.NextID
						} else {
							myExtraInfo[i] = nil
						}
					}
				}
			}
			for _, v := range myExtraInfo {
				if v != nil {
					//fmt.Printf("Updating %s for origin %s. Currently sending id: %d\n", peerAddress, v.Identifier, v.NextID)
					rm, _ := g.msgMap.GetMessage(v.Identifier, v.NextID)
					packet := &dto.GossipPacket{Rumor: rm}
					go g.stubbornSend(packet, peerAddress)
				}
			}

			//fmt.Printf("All stopped: %v\n", allStopped)
			//fmt.Printf("I have new Messages: %v\n", hasNewMessages)
			if !hasNewMessages && allStopped {
				for _, v := range sp.Want {
					wantedID := g.msgMap.GetNewestID(v.Identifier) + 1
					if wantedID < v.NextID {
						//fmt.Printf("Asking for new messages\n")
						g.sendStatusPacket(peerAddress)
						break
					}
				}
				fmt.Printf("IN SYNC WITH %s\n", peerAddress)
			}
		}
	}
}

func (g *Gossiper) rumorMonger(packet *dto.GossipPacket, except string) {
	exceptions := make([]string, 0)
	if except != "" {
		exceptions = append(exceptions, except)
	}
	peer, ok := g.pickRandomPeer(exceptions)
	if !ok {
		return
	}
	exceptions = append(exceptions, peer)
	for {
		g.stubbornSend(packet, peer)

		if rand.Intn(2) == 1 {
			return
		}

		peer, ok = g.pickRandomPeer(exceptions)
		if !ok {
			return
		}
		fmt.Printf("FLIPPED COIN sending rumor to %s\n", peer)
		//exceptions = append(exceptions, peer) 								//TODO: uncomment this later after talking to TAs
	}
}

func (g *Gossiper) stubbornSend(packet *dto.GossipPacket, peer string) {
	quit, alreadySending := g.quitChanMap.AddListener(peer, packet.GetOrigin(), packet.GetSeqID())
	if alreadySending {
		return
	}

	fmt.Printf("MONGERING with %s\n", peer)
	g.sendUDP(packet, peer)
	t := time.NewTicker(1 * time.Second)
	for {
		select {
		case <-t.C:
			fmt.Printf("MONGERING with %s\n", peer)
			g.sendUDP(packet, peer)
		case <-quit:
			t.Stop()
			return
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
				go g.rumorMonger(packet, "") //TODO: change "" this back to sender after talking with TAs
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
	//fmt.Printf("Sending status packet to peer: %s\n", peerAddress)
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
			//log.Print("Protobuf decoding error\n")
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
func (g *Gossiper) addToPeers(peerAddr string) bool {
	for _, v := range g.peers {
		if peerAddr == v {
			return false
		}
	}
	g.peers = append(g.peers, peerAddr)
	return true
}

/*
	g.statusChanMap.InformListeners(peer, sp)
	for _, peerStatus := range sp.Want {
		msg, ok := g.msgMap.GetMessage(peerStatus.Identifier, peerStatus.NextID)
		if ok {
			packet := &dto.GossipPacket{Rumor: msg}
			g.stubbornSend(packet, peer)
			return
		}
	}
*/

/*
for _, v := range sp.Want { //not enough, what about origins not in the "Want" array?
				id := g.msgMap.GetNewestID(v.Identifier)
				if id >= v.NextID {
					fmt.Printf("Updating %s for origin %s. Currently sending id: %d\n", peerAddress, v.Identifier, v.NextID)
					rm, _ := g.msgMap.GetMessage(v.Identifier, v.NextID)
					packet := &dto.GossipPacket{Rumor: rm}
					go g.stubbornSend(packet, peerAddress)
					hasNewMessages = true
				}
			}
*/
func (g *Gossiper) pickRandomPeer(exceptions []string) (string, bool) {
	possiblePicks := g.GetPeersExcept(exceptions)
	if len(possiblePicks) == 0 {
		return "", false
	}
	peer := possiblePicks[rand.Intn(len(possiblePicks))]
	return peer, true
}

func (g *Gossiper) GetPeersExcept(exceptions []string) (dif []string) {
	m := map[string]bool{}
	for _, v := range exceptions {
		m[v] = true
	}
	dif = []string{}
	for _, v := range g.peers {
		if _, ok := m[v]; !ok {
			dif = append(dif, v)
		}
	}
	return
}
