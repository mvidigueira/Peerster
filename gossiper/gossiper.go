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
const freq = 1

//Gossiper server responsible for answering client requests and rumor mongering with other gossipers
type Gossiper struct {
	address   string
	name      string
	UseSimple bool

	conn   *net.UDPConn
	connUI *net.UDPConn

	peers         *dto.SafeStringArray
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
		UseSimple: simple,

		conn:   udpConnGossip,
		connUI: udpConnClient,

		peers:         dto.NewSafeStringArray(peers),
		seqID:         dto.NewSafeCounter(),
		statusChanMap: dto.NewSafeChanMap(),
		quitChanMap:   dto.NewQuitChanMap(),
		msgMap:        dto.NewSafeMessagesMap(),
	}
}

//Start starts the gossiper listening routines
func (g *Gossiper) Start() {
	rand.Seed(time.Now().UnixNano())
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
	t := time.NewTicker(freq * time.Second)
	for {
		select {
		case <-t.C:
			pick, ok := g.pickRandomPeer([]string{})
			if ok {
				g.sendStatusPacket(pick)
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

		c, isNew := g.statusChanMap.AddOrGetPeerStatusListener(peer)
		if isNew {
			go g.peerStatusListenRoutine(peer, c)
		}
		c <- pap.Packet.Status
	}
}

func (g *Gossiper) peerStatusListenRoutine(peerAddress string, cStatus chan *dto.StatusPacket) {
	for {
		select {
		case sp := <-cStatus:
			notMongering := g.quitChanMap.InformListeners(peerAddress, sp)

			mySp := g.msgMap.GetOwnStatusPacket()
			theirDesiredMsgs := peerStatusDifference(mySp.Want, sp.Want)
			mySp.Print()
			sp.Print()
			(&dto.StatusPacket{Want: theirDesiredMsgs}).Print()
			for _, v := range theirDesiredMsgs {
				rm, _ := g.msgMap.GetMessage(v.Identifier, v.NextID)
				packet := &dto.GossipPacket{Rumor: rm}
				go g.stubbornSend(packet, peerAddress)
			}

			if notMongering && (len(theirDesiredMsgs) == 0) {
				ourDesiredMsgs := peerStatusDifference(sp.Want, mySp.Want)
				if len(ourDesiredMsgs) != 0 {
					g.sendStatusPacket(peerAddress)
				} else {
					fmt.Printf("IN SYNC WITH %s\n", peerAddress)
				}
			}
		}
	}
}

func (g *Gossiper) rumorMonger(packet *dto.GossipPacket, except string) {
	exceptions := []string{}
	if except != "" {
		exceptions = []string{except}
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
	t := time.NewTicker(freq * time.Second)
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
	}
}

//Peer handling
func (g *Gossiper) rumorListenRoutine(cRumor chan *dto.PacketAddressPair) {
	for pap := range cRumor {
		g.addToPeers(pap.GetSenderAddress())

		printGossiperMessage(pap)
		g.printKnownPeers()

		packet := g.makeGossip(pap.Packet, false)
		if !g.UseSimple {
			isNew := g.msgMap.AddMessage(packet.Rumor)
			g.sendStatusPacket(pap.GetSenderAddress())
			if isNew {
				go g.rumorMonger(packet, "") //TODO: change "" this back to sender after talking with TAs
			}
		} else {
			g.sendAllPeers(packet, pap.GetSenderAddress())
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
		_, udpAddr, err := g.conn.ReadFromUDP(packetBytes)
		if err != nil {
			log.Println(err)
			continue
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
	for _, v := range stringArrayDifference(g.peers.GetArrayCopy(), []string{exception}) {
		g.sendUDP(packet, v)
	}
}

func (g *Gossiper) sendUDP(packet *dto.GossipPacket, addr string) {
	udpAddr, err := net.ResolveUDPAddr("udp4", addr)
	dto.LogError(err)
	packetBytes, err := protobuf.Encode(packet)
	dto.LogError(err)
	g.conn.WriteToUDP(packetBytes, udpAddr)
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

func (g *Gossiper) pickRandomPeer(exceptions []string) (string, bool) {
	possiblePicks := stringArrayDifference(g.peers.GetArrayCopy(), exceptions)
	if len(possiblePicks) == 0 {
		return "", false
	}
	peer := possiblePicks[rand.Intn(len(possiblePicks))]
	return peer, true
}

func (g *Gossiper) addToPeers(peerAddr string) (isNew bool) {
	return g.peers.AppendUniqueToArray(peerAddr)
}

//PRINTING
func (g *Gossiper) printKnownPeers() {
	fmt.Println("PEERS " + strings.Join(g.peers.GetArrayCopy(), ","))
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

//'have': a status packet representing what messages we have
//'want': a status packet representing what messages another peer is interested in
//returns: a status packet representing all the useful messages that can be provided to the peer
//Complexity: Assuming Go maps are ~O(1), complexity is ~O(N)
func peerStatusDifference(have []dto.PeerStatus, want []dto.PeerStatus) []dto.PeerStatus {
	wantMap := make(map[string]uint32)
	for _, ps := range want {
		wantMap[ps.Identifier] = ps.NextID
	}
	wantList := make([]dto.PeerStatus, 0)
	for _, ps := range have {
		want, ok := wantMap[ps.Identifier]
		if ok && (want < ps.NextID) {
			wantList = append(wantList, dto.PeerStatus{Identifier: ps.Identifier, NextID: want})
		} else if !ok {
			wantList = append(wantList, dto.PeerStatus{Identifier: ps.Identifier, NextID: 1})
		}
	}
	return wantList
}

//Complexity: Assuming Go maps are ~O(1), complexity is ~O(N)
func stringArrayDifference(first []string, second []string) (difference []string) {
	mSecond := make(map[string]bool)
	for _, key := range second {
		mSecond[key] = true
	}
	difference = []string{}
	for _, key := range first {
		if _, ok := mSecond[key]; !ok {
			difference = append(difference, key)
		}
	}
	return
}
