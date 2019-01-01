package gossiper

import (
	"fmt"
	"log"
	"math/big"
	"math/rand"
	"net"
	"protobuf"
	"strconv"
	"time"

	"github.com/mvidigueira/Diffie-Hellman/aesencryptor"
	"github.com/mvidigueira/Diffie-Hellman/diffiehellman"
	"github.com/mvidigueira/Peerster/dht"
	"github.com/mvidigueira/Peerster/dto"
	"github.com/mvidigueira/Peerster/fileparsing"
	"github.com/mvidigueira/Peerster/filesearching"
	"github.com/mvidigueira/Peerster/routing"
)

var packetSize = 10000

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
	origins        *dto.SafeStringArray

	routingTable *routing.SafeRoutingTable
	rtimeout     int

	chunkMap         *fileparsing.SafeChunkMap
	fileMap          *fileparsing.SafeFileMap
	dlFilesSet       *fileparsing.SafeFileSet
	dlChunkListeners *fileparsing.DLChanMap

	metahashToChunkOwnersMap *filesearching.MetahashToChunkOwnersMap
	searchMap                *filesearching.SafeSearchMap
	filenamesMap             *filesearching.SafeFilenamesMap
	matchesGUImap            *dto.SafeHashFilenamePairArray

	blockchainLedger *BlockchainLedger

	dhtMyID     [dht.IDByteSize]byte
	dhtChanMap  *dht.ChanMap
	bucketTable *bucketTable
	storage     *dht.StorageMap

	privateKey           [dht.IDByteSize]byte
	diffieHellmanMap     map[string](chan *dto.DiffieHellman)
	activeDiffieHellmans map[string]([]byte)
}

//NewGossiper creates a new gossiper
func NewGossiper(address, name string, UIport string, peers []string, simple bool, rtimeout int) *Gossiper {
	gossipAddr, err := net.ResolveUDPAddr("udp4", address)
	dto.LogError(err)
	clientAddr, err := net.ResolveUDPAddr("udp4", "localhost:"+UIport)
	dto.LogError(err)
	udpConnGossip, err := net.ListenUDP("udp4", gossipAddr)
	dto.LogError(err)
	udpConnClient, err := net.ListenUDP("udp4", clientAddr)
	dto.LogError(err)

	g := &Gossiper{
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

		latestMessages: dto.NewSafeMessageArray(),
		origins:        dto.NewSafeStringArray([]string{}),

		routingTable: routing.NewSafeRoutingTable(),
		rtimeout:     rtimeout,

		chunkMap:         fileparsing.NewSafeChunkMap(),
		fileMap:          fileparsing.NewSafeFileMap(),
		dlFilesSet:       fileparsing.NewSafeFileSet(),
		dlChunkListeners: fileparsing.NewDLChanMap(),

		metahashToChunkOwnersMap: filesearching.NewMetahashToChunkOwnersMap(),
		searchMap:                filesearching.NewSafeSearchMap(),
		filenamesMap:             filesearching.NewSafefileNamesMap(),
		matchesGUImap:            dto.NewSafeHashFilenamePairArray(),

		blockchainLedger: NewBlockchainLedger(),

		dhtMyID:    dht.InitialRandNodeID(),
		dhtChanMap: dht.NewChanMap(),
		storage:    dht.NewStorageMap(),

		diffieHellmanMap:     map[string](chan *dto.DiffieHellman){},
		activeDiffieHellmans: map[string]([]byte){},
	}
	g.bucketTable = newBucketTable(g.dhtMyID, g)
	return g
}

//Start starts the gossiper listening routines
func (g *Gossiper) Start() {
	rand.Seed(time.Now().UnixNano())

	cRumor := make(chan *dto.PacketAddressPair)
	go g.rumorListenRoutine(cRumor)
	cStatus := make(chan *dto.PacketAddressPair)
	go g.statusListenRoutine(cStatus)

	cPrivate := make(chan *dto.PacketAddressPair)
	go g.privateMessageListenRoutine(cPrivate)

	cDataRequest := make(chan *dto.PacketAddressPair)
	go g.dataRequestListenRoutine(cDataRequest)
	cDataReply := make(chan *dto.PacketAddressPair)
	go g.dataReplyListenRoutine(cDataReply)

	cSearcRequest := make(chan *dto.PacketAddressPair)
	go g.searchRequestListenRoutine(cSearcRequest)
	cSearchReply := make(chan *dto.PacketAddressPair)
	go g.searchReplyListenRoutine(cSearchReply)

	cFileNaming := make(chan *dto.PacketAddressPair) //blockchain
	cBlocks := make(chan *dto.PacketAddressPair)     //blockchain
	//go g.blockchainMiningRoutine(cFileNaming, cBlocks)

	cDiffieHellman := make(chan *dto.PacketAddressPair)
	go g.diffieListenRoutine(cDiffieHellman)

	cEncryptedMessage := make(chan *dto.PacketAddressPair)
	go g.encryptedPrivateMessageListenRoutine(cEncryptedMessage)

	go g.receiveExternalUDP(cRumor, cStatus, cPrivate, cDataRequest, cDataReply, cSearcRequest, cSearchReply, cFileNaming, cBlocks, cDiffieHellman, cEncryptedMessage)
	go g.antiEntropy()

	go g.periodicRouteRumor() //DSDV

	cUI := make(chan *dto.PacketAddressPair)
	go g.clientListenRoutine(cUI)
	cUIPM := make(chan *dto.PacketAddressPair)
	go g.clientPMListenRoutine(cUIPM)

	cFileShare := make(chan string)
	go g.clientFileShareListenRoutine(cFileShare, cFileNaming)
	cFileDL := make(chan *dto.FileToDownload)
	go g.clientFileDownloadListenRoutine(cFileDL)

	cFileSearch := make(chan *dto.FileToSearch)
	go g.clientFileSearchListenRoutine(cFileSearch)

	cEncryptedPrivate := make(chan *dto.PacketAddressPair)
	go g.clientEPMListenRoutine(cEncryptedPrivate)

	clientDiffieHellman := make(chan *dto.PacketAddressPair)
	go g.clientDiffieListenRoutine(clientDiffieHellman)

	g.receiveClientUDP(cUI, cUIPM, cFileShare, cFileDL, cFileSearch, clientDiffieHellman, cEncryptedPrivate)
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
	dto.SendGossipPacket(packet, addr, g.conn)
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
func (g *Gossiper) receiveClientUDP(cRumoring, cPMing chan *dto.PacketAddressPair, cFileShare chan string, cFileDL chan *dto.FileToDownload, cFileSearch chan *dto.FileToSearch, cDiffie, cEncryptedPrivate chan *dto.PacketAddressPair) {
	for {
		request := &dto.ClientRequest{}
		packetBytes := make([]byte, packetSize)
		n, _, err := g.connUI.ReadFromUDP(packetBytes)
		if n > packetSize {
			packetSize = packetSize + 1024
		}
		if err != nil {
			log.Println(err)
			continue
		}
		fmt.Println("NEW MSG")
		err = protobuf.Decode(packetBytes, request)
		switch request.GetUnderlyingType() {
		case "simple":
			packet := request.Packet
			if packet.GetContents() == "" {
				continue
			}
			packet.Simple.OriginalName = g.name
			cRumoring <- &dto.PacketAddressPair{Packet: packet}
		case "rumor":
			packet := request.Packet
			if packet.GetContents() == "" {
				continue
			}
			packet.Rumor.Origin = g.name
			cRumoring <- &dto.PacketAddressPair{Packet: packet}
		case "private":
			packet := request.Packet
			if packet.GetContents() == "" {
				continue
			}
			cPMing <- &dto.PacketAddressPair{Packet: packet}
		case "fileShare":
			cFileShare <- request.FileShare.GetFileName()
		case "fileDownload":
			cFileDL <- request.FileDownload
		case "fileSearch":
			cFileSearch <- request.FileSearch
		case "diffiehellman":
			packet := request.Packet
			packet.DiffieHellman.Origin = g.name
			cDiffie <- &dto.PacketAddressPair{Packet: packet}
		case "encryptedmessage":
			fmt.Println("NEW ENC")
			packet := request.Packet
			packet.EncryptedMessage.Origin = g.name
			cEncryptedPrivate <- &dto.PacketAddressPair{Packet: packet}
		default:
			log.Println("Unrecognized message type. Ignoring...")
		}
	}
}

//receiveExternalUDP - receives gossip packets from PEERS and forwards them to the appropriate channel
//among those provided, depending on whether they are rumor, simple or status packets
func (g *Gossiper) receiveExternalUDP(cRumor, cStatus, cPrivate, cDataRequest, cDataReply, cSearcRequest, cSearchReply, cFileNaming, cBlocks, cDiffieHellman, cEncypted chan *dto.PacketAddressPair) {
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
		case "datarequest":
			if g.UseSimple {
				log.Println("Running on 'simple' mode. Ignoring datarequest message from " + senderAddress + "...")
			} else {
				cDataRequest <- pap
			}
		case "datareply":
			if g.UseSimple {
				log.Println("Running on 'simple' mode. Ignoring datareply message from " + senderAddress + "...")
			} else {
				cDataReply <- pap
			}
		case "searchrequest":
			if g.UseSimple {
				log.Println("Running on 'simple' mode. Ignoring searchrequest message from " + senderAddress + "...")
			} else {
				cSearcRequest <- pap
			}
		case "searchreply":
			if g.UseSimple {
				log.Println("Running on 'simple' mode. Ignoring searchreply message from " + senderAddress + "...")
			} else {
				cSearchReply <- pap
			}
		case "txpublish":
			if g.UseSimple {
				log.Println("Running on 'simple' mode. Ignoring txpublish message from " + senderAddress + "...")
			} else {
				cFileNaming <- pap
			}
		case "blockpublish":
			if g.UseSimple {
				log.Println("Running on 'simple' mode. Ignoring blockpublish message from " + senderAddress + "...")
			} else {
				cBlocks <- pap
			}
		case "diffiehellman":
			if g.UseSimple {
				log.Println("Running on 'simple' mode. Ignoring diffiehellman message from " + senderAddress + "...")
			} else {
				cDiffieHellman <- pap
			}
		case "encryptedmessage":
			if g.UseSimple {
				log.Println("Running on 'simple' mode. Ignoring encrypted message from " + senderAddress + "...")
			} else {
				cEncypted <- pap
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

//statusListenRoutine - deals with Status packets from all sources, adding unknown sources to the peers list.
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

//clientListenRoutine - deals with Simple/Rumor messages from clients
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

//rumorListenRoutine - deals with Rumor/Simple packets from other peers
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

//diffieHellman

func (g *Gossiper) diffieListenRoutine(cDiffieHellman chan *dto.PacketAddressPair) {
	for pap := range cDiffieHellman {
		g.addToPeers(pap.GetSenderAddress())
		d, f := g.diffieHellmanMap[pap.GetSenderAddress()]
		if !f {
			// New diffie hellman request
			responseChannel := make(chan *dto.DiffieHellman)
			g.diffieHellmanMap[pap.GetSenderAddress()] = responseChannel
			// Start diffiehellman process
			go g.negotiateDiffieHellman(responseChannel, pap.GetSenderAddress(), pap.Packet.DiffieHellman)
		} else {
			d <- pap.Packet.DiffieHellman
		}
	}
}

func (g *Gossiper) clientDiffieListenRoutine(cDiffieHellman chan *dto.PacketAddressPair) {
	for pap := range cDiffieHellman {
		g.addToPeers(pap.GetSenderAddress())
		_, f := g.activeDiffieHellmans[pap.GetSenderAddress()]
		if f {
			fmt.Println("You already share a key with this peer")
			continue
		}

		_, f = g.diffieHellmanMap[pap.GetSenderAddress()]
		if f {
			fmt.Println("DiffieHellman negotation already active")
			continue
		}

		// New diffie hellman request
		responseChannel := make(chan *dto.DiffieHellman)
		g.diffieHellmanMap[pap.Packet.DiffieHellman.Destination] = responseChannel
		// Start diffiehellman process
		go g.negotiateDiffieHellmanInitiator(responseChannel, pap.Packet.DiffieHellman)

	}
}

func (g *Gossiper) negotiateDiffieHellmanInitiator(ch chan *dto.DiffieHellman, packet *dto.DiffieHellman) {
	diffieHellman := diffiehellman.New(diffiehellman.Group(), diffiehellman.P())
	publicKey := diffieHellman.GeneratePublicKey()

	// TODO Sign package with my private key.

	g.sendUDP(&dto.GossipPacket{
		DiffieHellman: &dto.DiffieHellman{
			Origin:      g.name,
			Destination: packet.Destination,
			HopLimit:    10,
			P:           fmt.Sprintf("%x", diffiehellman.P()),
			G:           fmt.Sprintf("%x", diffiehellman.Group()),
			PublicKey:   publicKey,
		},
	}, packet.Destination)

	reply := <-ch

	symmetricKey := diffieHellman.GenerateSymmetricKey(reply.PublicKey)

	g.activeDiffieHellmans[packet.Destination] = symmetricKey

	delete(g.diffieHellmanMap, packet.Destination)

	// Send ack

	g.sendUDP(&dto.GossipPacket{
		DiffieHellman: &dto.DiffieHellman{},
	}, packet.Destination)

	fmt.Println(symmetricKey)

	fmt.Printf("Key setup with %s\n", packet.Destination)
}

func (g *Gossiper) negotiateDiffieHellman(ch chan *dto.DiffieHellman, saddr string, packet *dto.DiffieHellman) {
	group, _ := new(big.Int).SetString(packet.G, 16)
	p, _ := new(big.Int).SetString(packet.P, 16)
	diffieHellman := diffiehellman.New(group, p)
	publicKey := diffieHellman.GeneratePublicKey()
	symmetricKey := diffieHellman.GenerateSymmetricKey(packet.PublicKey)

	// 1. Send your public key to the initiator

	g.sendUDP(&dto.GossipPacket{
		DiffieHellman: &dto.DiffieHellman{
			Origin:      g.name,
			Destination: saddr,
			HopLimit:    10,
			P:           packet.P,
			G:           packet.G,
			PublicKey:   publicKey,
		},
	}, saddr)

	// 2. Wait for reply

	<-ch

	delete(g.diffieHellmanMap, saddr)

	g.activeDiffieHellmans[saddr] = symmetricKey

	fmt.Println(symmetricKey)

	fmt.Printf("Key setup with %s\n", saddr)

}

func (g *Gossiper) clientEPMListenRoutine(ch chan *dto.PacketAddressPair) {
	for pap := range ch {
		eMsg := pap.Packet.EncryptedMessage
		dest := eMsg.Destination
		d, f := g.activeDiffieHellmans[dest]
		if !f {
			fmt.Printf("No active diffie hellman session with %s\n", d)
		}

		pap.Packet.EncryptedMessage.ID = 0
		pap.Packet.EncryptedMessage.HopLimit = defaultHopLimit
		pap.Packet.EncryptedMessage.Origin = g.name

		aesEncrypter := aesencryptor.New(d)
		eMsg.CipherText = aesEncrypter.Encrypt(eMsg.CipherText)
		fmt.Println(eMsg.CipherText)
		pap.Packet.EncryptedMessage = eMsg
		g.sendUDP(pap.Packet, dest)
		/*if pap.Packet.EncryptedMessage.HopLimit > 0 {
			fmt.Println("HOP OK")
			nextHop, ok := g.getNextHop(pap.GetDestination())
			fmt.Println(g.getNextHop(pap.GetDestination()))
			if ok {
				fmt.Println("SENDING EPM")
				g.sendUDP(pap.Packet, nextHop)
			}
		}*/
	}
}

func (g *Gossiper) encryptedPrivateMessageListenRoutine(cEncryptedPrivate chan *dto.PacketAddressPair) {
	for pap := range cEncryptedPrivate {
		eMsg := pap.Packet.EncryptedMessage
		d, f := g.activeDiffieHellmans[pap.GetSenderAddress()]
		if !f {
			fmt.Printf("No active diffie hellman session with %s\n", d)
		}
		fmt.Printf("Cipher: %s\n", eMsg.CipherText)
		aesEncrypter := aesencryptor.New(d)
		eMsg.CipherText = aesEncrypter.Decrypt(eMsg.CipherText)
		fmt.Println(string(eMsg.CipherText))
	}
}
