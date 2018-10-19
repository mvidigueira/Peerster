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

var packetSize = 1024

const globalSeed = 2

const rumorMongerTimeout = 1
const antiEntropyTimeout = 1

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

	latestMessages *dto.SafeMessageArray
}

//NewGossiper creates a new gossiper
func NewGossiper(address, name string, UIport int, peers []string, simple bool) *Gossiper {
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
		seqID:         dto.NewSafeCounter(),
		statusChanMap: dto.NewSafeChanMap(),
		quitChanMap:   dto.NewQuitChanMap(),
		msgMap:        dto.NewSafeMessagesMap(),

		latestMessages: dto.NewSafeMessageArray(),
	}
}

//Start starts the gossiper listening routines
func (g *Gossiper) Start() {
	rand.Seed(globalSeed) //time.Now().UnixNano()
	cUI := make(chan *dto.PacketAddressPair)
	go g.clientListenRoutine(cUI)
	go g.receiveClientUDP(cUI)

	cRumor := make(chan *dto.PacketAddressPair)
	cStatus := make(chan *dto.PacketAddressPair)
	go g.statusListenRoutine(cStatus)
	go g.rumorListenRoutine(cRumor)
	go g.receiveExternalUDP(cRumor, cStatus)
	g.antiEntropy()
}

//addToPeers - adds a peer (ip:port) to the list of known peers
func (g *Gossiper) addToPeers(peerAddr string) (isNew bool) {
	return g.peers.AppendUniqueToArray(peerAddr)
}

//pickRandomPeer - picks a random peer from the list of known peers, excluding any in the 'exceptions" list
func (g *Gossiper) pickRandomPeer(exceptions []string) (string, bool) {
	possiblePicks := stringArrayDifference(g.peers.GetArrayCopy(), exceptions)
	if len(possiblePicks) == 0 {
		return "", false
	}
	peer := possiblePicks[rand.Intn(len(possiblePicks))]
	return peer, true
}

//addMessage - adds a message (rumor message) to the map of known messages and latest messages
func (g *Gossiper) addMessage(rm *dto.RumorMessage) bool {
	isNew := g.msgMap.AddMessage(rm)
	if isNew {
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
func (g *Gossiper) receiveClientUDP(c chan *dto.PacketAddressPair) {
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
			packet.Simple.OriginalName = g.name
			c <- &dto.PacketAddressPair{Packet: packet}
		default:
			log.Println("Unrecognized message type. Ignoring...")
		}
	}
}

//receiveExternalUDP - receives gossip packets from PEERS and forwards them to the appropriate channel
//among those provided, depending on whether they are rumor, simple or status packets
func (g *Gossiper) receiveExternalUDP(cRumor, cStatus chan *dto.PacketAddressPair) {
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

//peerStatusListenRoutine - status listening routine for a specific peer. Has 3 major functionalities:
//1) Informs rumormongering routines currently sending (trySend function) if the peers are in sync.
//2) Updates the corresponding peer if it is detected that he is outdated
//3) After 2), sends a status packet if the gossiper is outdated in comparison with the other peer
func (g *Gossiper) peerStatusListenRoutine(peerAddress string, cStatus chan *dto.StatusPacket) {
	for {
		select {
		case sp := <-cStatus:
			mySp := g.msgMap.GetOwnStatusPacket()
			statusUpdated := dto.StatusPacket{Want: peerStatusUpdated(mySp.Want, sp.Want)}
			printStatusMessage(peerAddress, statusUpdated)

			notMongering := g.quitChanMap.InformListeners(peerAddress, sp)

			theirDesiredMsgs := peerStatusDifference(mySp.Want, sp.Want)

			for _, v := range theirDesiredMsgs {
				rm, _ := g.msgMap.GetMessage(v.Identifier, v.NextID)
				packet := &dto.GossipPacket{Rumor: rm}
				fmt.Printf("MONGERING with %s\n", peerAddress)
				g.sendUDP(packet, peerAddress)
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

//trySend - sends a gossip message to 'peer'. returns when either:
//1) the peers are in sync (synchronization handled by peerStatusListenRoutine)
//2) a timeout ocurrs
func (g *Gossiper) trySend(packet *dto.GossipPacket, peer string) bool {
	quit, alreadySending := g.quitChanMap.AddListener(peer, packet.GetOrigin(), packet.GetSeqID())
	if alreadySending { //this should be impossible if using method trySend correctly. Keeping it here for future safety
		return false
	}

	fmt.Printf("MONGERING with %s\n", peer)
	g.sendUDP(packet, peer)

	t := time.NewTicker(rumorMongerTimeout * time.Second)
	for {
		select {
		case <-t.C:
			g.quitChanMap.DeleteListener(peer, packet.GetOrigin(), packet.GetSeqID())
			return false
		case <-quit:
			t.Stop()
			return true
		}
	}
}

//rumorMonger - implements rumor mongering as per the project description
//when choosing peers, excludes the sender on the first pick, and the previous choice on every following pick.
//stops mongering if there are no available peers to send to (e.g. only knows peer who sent the message)
func (g *Gossiper) rumorMonger(packet *dto.GossipPacket, except string) {
	exceptions := []string{}
	if except != "" {
		exceptions = []string{except}
	}
	peer, ok := g.pickRandomPeer(exceptions)
	if !ok {
		return
	}
	for {
		g.trySend(packet, peer)

		if rand.Intn(2) == 1 {
			return
		}
		exceptions = []string{peer}
		peer, ok = g.pickRandomPeer(exceptions)
		if !ok {
			return
		}
		fmt.Printf("FLIPPED COIN sending rumor to %s\n", peer)
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

		printGossiperMessage(pap)
		g.printKnownPeers()

		packet := g.makeGossip(pap.Packet, false)
		if !g.UseSimple {
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

//antiEntropy - implements anti entropy as per the project description
func (g *Gossiper) antiEntropy() {
	if g.UseSimple {
		return
	}
	t := time.NewTicker(antiEntropyTimeout * time.Second)
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

//PRINTING
//printKnownPeers - prints all known peers, format according to project description
func (g *Gossiper) printKnownPeers() {
	fmt.Println("PEERS " + strings.Join(g.peers.GetArrayCopy(), ","))
}

//printStatusMessage - prints a status message, format according to project description
func printStatusMessage(senderAddress string, status dto.StatusPacket) {
	fmt.Printf("STATUS from %v %v\n", senderAddress, status.String())
}

//printClientMessage - prints a client message, format according to project description
func printClientMessage(pair *dto.PacketAddressPair) {
	fmt.Printf("CLIENT MESSAGE %v\n", pair.GetContents())
}

//printGossiperMessage - prints a peer's message (rumor or simple), format according to project description
func printGossiperMessage(pair *dto.PacketAddressPair) {
	switch subtype := pair.Packet.GetUnderlyingType(); subtype {
	case "simple":
		fmt.Printf("SIMPLE MESSAGE origin %v from %v contents %v\n",
			pair.GetOrigin(), pair.GetSenderAddress(), pair.GetContents())
	case "rumor":
		fmt.Printf("RUMOR origin %v from %v ID %v contents %v\n",
			pair.GetOrigin(), pair.GetSenderAddress(), pair.GetSeqID(), pair.GetContents())
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

//peerStatusUpdated - same as above, but includes messages the peer wants but we can't give them
//only used for printing according to project description example, otherwise pointless
func peerStatusUpdated(have []dto.PeerStatus, want []dto.PeerStatus) []dto.PeerStatus {
	wantMap := make(map[string]uint32)
	for _, ps := range want {
		wantMap[ps.Identifier] = ps.NextID
	}
	for _, ps := range have {
		_, ok := wantMap[ps.Identifier]
		if !ok {
			want = append(want, dto.PeerStatus{Identifier: ps.Identifier, NextID: 1})
		}
	}
	return want
}

//Complexity: ~O(N)
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

//UI functionality

//GetName - returns the gossiper's name
func (g *Gossiper) GetName() string {
	return g.name
}

//GetLatestMessagesList - returns whatever messages have been received
//since this function was last called
func (g *Gossiper) GetLatestMessagesList() []dto.RumorMessage {
	return g.latestMessages.GetArrayCopyAndDelete()
}

//GetPeersList - returns a string array with all known peers (ip:port)
func (g *Gossiper) GetPeersList() []string {
	return g.peers.GetArrayCopy()
}

//AddPeer - adds a peer (ip:port) to the list of known peers
func (g *Gossiper) AddPeer(peerAdress string) {
	g.addToPeers(peerAdress)
}
