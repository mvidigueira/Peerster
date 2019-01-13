package gossiper

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"log"
	"math/big"
	"time"

	"github.com/dedis/protobuf"

	"github.com/mvidigueira/Peerster/dht_util"
	"github.com/mvidigueira/Peerster/diffie_hellman/diffiehellman"
	"github.com/mvidigueira/Peerster/dto"
)

type DiffieHellmanSession struct {
	Key            []byte
	LastTimeUsed   time.Time
	ExpirationDate time.Time
}

func (ds *DiffieHellmanSession) expired() bool {
	return time.Now().After(ds.ExpirationDate)
}

func NewDiffieHellmanSession(key []byte, ExpirationDate time.Time) *DiffieHellmanSession {
	return &DiffieHellmanSession{
		Key:            key,
		ExpirationDate: ExpirationDate,
	}
}

func (g *Gossiper) CleanOldDiffieHellmanSessionsRoutine() {
	for {
		select {
		case <-time.After(time.Second * 5):
			g.activeDiffieHellmanMutex.Lock()
			for key, v := range g.activeDiffieHellmans {
				for _, session := range v {
					if session.expired() && time.Now().After(session.LastTimeUsed.Add(time.Second*10)) {
						delete(g.activeDiffieHellmans, key)
					}
				}
			}
			g.activeDiffieHellmanMutex.Unlock()
		}
	}
}

func (g *Gossiper) diffieListenRoutine(cDiffieHellman chan *dto.PacketAddressPair) {
	for pap := range cDiffieHellman {
		g.addToPeers(pap.GetSenderAddress())
		g.diffieHellmanMapMutex.Lock()
		d, f := g.diffieHellmanMap[pap.Packet.DiffieHellman.NodeID]
		g.diffieHellmanMapMutex.Unlock()
		if !f {
			// Start diffiehellman process
			go g.negotiateDiffieHellman(pap.GetSenderAddress(), pap.Packet.DiffieHellman.NodeID, pap.Packet.DiffieHellman)
		} else {
			d <- pap.Packet.DiffieHellman
		}
	}
}

func (g *Gossiper) negotiateDiffieHellmanInitiator(dest string, nodeID [dht_util.IDByteSize]byte) chan ([]byte) {

	callBackChannel := make(chan []byte)

	go func() {
		g.diffieHellmanMapMutex.Lock()
		rChannel := make(chan *dto.DiffieHellman)
		g.diffieHellmanMap[nodeID] = rChannel
		g.diffieHellmanMapMutex.Unlock()

		// Create a diffie hellman instance and a diffie hellman public key which will be used
		// only for the key exchange
		diffieHellman := diffiehellman.New(diffiehellman.Group(), diffiehellman.P())
		diffiePublicKey := diffieHellman.GeneratePublicKey()

		diffiePacket := &dto.DiffieHellman{
			Origin:          g.name,
			Destination:     dest,
			P:               fmt.Sprintf("%x", diffiehellman.P()),
			G:               fmt.Sprintf("%x", diffiehellman.Group()),
			DiffiePublicKey: diffiePublicKey,
			EcdsaPublicKey:  g.dhtMyID[:],
			NodeID:          g.dhtMyID,
			Init:            true,
		}
		r, s, err := g.signPacket(diffiePacket)
		if err != nil {
			log.Fatal("could not sign request")
		}
		diffiePacket.R = r
		diffiePacket.S = s

		g.sendUDP(&dto.GossipPacket{
			DiffieHellman: diffiePacket,
		}, dest)

		var reply *dto.DiffieHellman
		select {
		case r := <-rChannel:
			reply = r
		case <-time.After(time.Second * 15):
			fmt.Println("Timeout, aborting.")
			g.diffieHellmanMapMutex.Lock()
			delete(g.diffieHellmanMap, nodeID)
			g.diffieHellmanMapMutex.Unlock()
			callBackChannel <- nil
			return
		}

		ok := g.verifyPublicKey(reply.EcdsaPublicKey, dest)
		if !ok {
			fmt.Println("Failed to verify public key.")
			g.diffieHellmanMapMutex.Lock()
			delete(g.diffieHellmanMap, nodeID)
			g.diffieHellmanMapMutex.Unlock()
			callBackChannel <- nil
			return
		}
		ok = g.verifySignature(reply)
		if !ok {
			fmt.Println("Message signature not ok. Aborting diffie-hellman key exchange.3")
			g.diffieHellmanMapMutex.Lock()
			delete(g.diffieHellmanMap, nodeID)
			g.diffieHellmanMapMutex.Unlock()
			callBackChannel <- nil
			return
		}

		// Send ack
		ack := g.diffieAcklowledge(dest)

		// Wait for ack
		ok, _ = g.waitForAcknowledge(dest, nodeID, rChannel)
		if !ok {
			fmt.Println("failed to get ack.")
			g.diffieHellmanMapMutex.Lock()
			delete(g.diffieHellmanMap, nodeID)
			g.diffieHellmanMapMutex.Unlock()
			callBackChannel <- nil
			return
		}

		symmetricKey := diffieHellman.GenerateSymmetricKey(reply.DiffiePublicKey)

		g.activeDiffieHellmanMutex.Lock()
		g.activeDiffieHellmans[nodeID] = append(g.activeDiffieHellmans[nodeID], NewDiffieHellmanSession(symmetricKey, ack.ExpirationDate))
		g.activeDiffieHellmanMutex.Unlock()

		g.diffieHellmanMapMutex.Lock()
		delete(g.diffieHellmanMap, nodeID)
		g.diffieHellmanMapMutex.Unlock()

		fmt.Printf("Key setup with %s\n", dest)

		callBackChannel <- symmetricKey
	}()
	return callBackChannel
}

func (g *Gossiper) negotiateDiffieHellman(dest string, nodeID [dht_util.IDByteSize]byte, packet *dto.DiffieHellman) {

	// Create channel to map responses back to this routine.
	rChannel := make(chan *dto.DiffieHellman)
	g.diffieHellmanMapMutex.Lock()
	g.diffieHellmanMap[nodeID] = rChannel
	g.diffieHellmanMapMutex.Unlock()

	// Get group and p used in diffie hellman
	group, b := new(big.Int).SetString(packet.G, 16)
	if !b {
		fmt.Println(packet.G)
		fmt.Println(b)
		log.Fatal("AWDAWD123")

	}
	p, b := new(big.Int).SetString(packet.P, 16)
	if !b {
		fmt.Println(packet.P)
		fmt.Println(b)
		log.Fatal("AWDAWD321")
	}
	diffieHellman := diffiehellman.New(group, p)

	// generate symmetric key
	symmetricKey := diffieHellman.GenerateSymmetricKey(packet.DiffiePublicKey)

	// Generarate one time public key used during diffie hellman
	diffiePublicKey := diffieHellman.GeneratePublicKey()

	// Send your public key to the initiator

	diffiePacket := &dto.DiffieHellman{
		Origin:          g.name,
		Destination:     dest,
		HopLimit:        10,
		P:               packet.P,
		G:               packet.G,
		DiffiePublicKey: diffiePublicKey,
		EcdsaPublicKey:  g.dhtMyID[:],
		NodeID:          g.dhtMyID,
		Init:            false,
	}
	r, s, err := g.signPacket(diffiePacket)
	if err != nil {
		fmt.Errorf("could not sign request: %v", err)
		g.diffieHellmanMapMutex.Lock()
		delete(g.diffieHellmanMap, nodeID)
		g.diffieHellmanMapMutex.Unlock()
		return
	}
	diffiePacket.S = s
	diffiePacket.R = r

	g.sendUDP(&dto.GossipPacket{
		DiffieHellman: diffiePacket,
	}, dest)

	// Wait for ack
	ok, ack := g.waitForAcknowledge(dest, nodeID, rChannel)
	if !ok {
		fmt.Println("failed to get ack.")
		g.diffieHellmanMapMutex.Lock()
		delete(g.diffieHellmanMap, nodeID)
		g.diffieHellmanMapMutex.Unlock()
		return
	}

	// Send ack
	g.diffieAcklowledge(dest)

	// Save symmetric key
	g.activeDiffieHellmanMutex.Lock()
	g.activeDiffieHellmans[nodeID] = append(g.activeDiffieHellmans[nodeID], NewDiffieHellmanSession(symmetricKey, ack.ExpirationDate))
	g.activeDiffieHellmanMutex.Unlock()

	// Clean up
	g.diffieHellmanMapMutex.Lock()
	delete(g.diffieHellmanMap, nodeID)
	g.diffieHellmanMapMutex.Unlock()

	fmt.Println(symmetricKey)
	fmt.Printf("Key setup with %s\n", dest)
}

func (g *Gossiper) signPacket(packet *dto.DiffieHellman) ([]byte, []byte, error) {
	bytes, err := protobuf.Encode(packet)
	if err != nil {
		log.Fatal("error decoding diffie packet.")
	}

	hash := sha256.Sum256(bytes)
	r, s, err := ecdsa.Sign(rand.Reader, g.privateKey, hash[:])
	if err != nil {
		panic(err)
	}
	return r.Bytes(), s.Bytes(), err
}

func (g *Gossiper) verifySignature(packet *dto.DiffieHellman) bool {

	// R and S is not part of the message which should be hashed.
	// Let's therefore copy the values and delete them from the package.
	r := new(big.Int)
	r.SetBytes(packet.R)
	s := new(big.Int)
	s.SetBytes(packet.S)
	packet.S = nil
	packet.R = nil

	// Hash message
	bytes, err := protobuf.Encode(packet)
	if err != nil {
		log.Fatal("error ver diffie packet.")
	}
	hash := sha256.Sum256(bytes)

	// Extra public key components
	x, y := g.extractEcdsaPublicKeyComponents(packet)

	// Create ECDSA public key
	publicKey := &ecdsa.PublicKey{Curve: elliptic.P256(), X: x, Y: y}

	valid := ecdsa.Verify(publicKey, hash[:], r, s)

	return valid
}

// verify that public key belongs to sender
func (g *Gossiper) verifyPublicKey(key []byte, sender string) bool {
	var k [dht_util.IDByteSize]byte
	copy(k[:], key)
	closest := g.LookupNodes(k)
	if len(closest) == 0 {
		return false
	}
	return bytes.Equal(closest[0].NodeID[:], k[:]) && closest[0].Address == sender
}

// Send acknowledgement package
func (g *Gossiper) diffieAcklowledge(dest string) *dto.DiffieHellman {
	ack := &dto.DiffieHellman{
		EcdsaPublicKey: g.dhtMyID[:],
		NodeID:         g.dhtMyID,
		Init:           false,
		ExpirationDate: time.Now().Local().Add(time.Second * time.Duration(10))}
	r, s, err := g.signPacket(ack)
	if err != nil {
		log.Fatal("could not sign request")
	}
	ack.S = s
	ack.R = r
	g.sendUDP(&dto.GossipPacket{
		DiffieHellman: ack,
	}, dest)

	return ack
}

func (g *Gossiper) waitForAcknowledge(dest string, nodeID [dht_util.IDByteSize]byte, responseChan chan *dto.DiffieHellman) (bool, *dto.DiffieHellman) {
	select {
	case res := <-responseChan:
		ok := g.verifyPublicKey(res.EcdsaPublicKey, dest)
		if !ok {
			fmt.Println("Failed to verify public key.")
			return false, nil
		}
		ok = g.verifySignature(res)
		if !ok {
			fmt.Println("Message signature not ok. Aborting diffie-hellman key exchange. 5")
			return false, nil
		}
		return true, res
	case <-time.After(time.Second * 15):
		fmt.Println("Timeout, aborting.")
		g.diffieHellmanMapMutex.Lock()
		delete(g.diffieHellmanMap, nodeID)
		g.diffieHellmanMapMutex.Unlock()
		return false, nil
	}
}

func (g *Gossiper) extractEcdsaPublicKeyComponents(packet *dto.DiffieHellman) (*big.Int, *big.Int) {
	x := new(big.Int)
	x.SetBytes(packet.EcdsaPublicKey[:32])
	y := new(big.Int)
	y.SetBytes(packet.EcdsaPublicKey[32:])
	return x, y
}
