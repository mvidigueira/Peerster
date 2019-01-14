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

type DiffieHellmanNegotiationSession struct {
	ch chan *dto.DiffieHellman
	ID [dht_util.IDByteSize]byte
}

func (g *Gossiper) CleanOldDiffieHellmanSessionsRoutine() {
	for {
		select {
		case <-time.After(time.Second * 5):
			g.activeOutgoingDiffieHellmanMutex.Lock()
			for key, v := range g.activeOutgoingDiffieHellmans {
				for _, session := range v {
					if session.expired() && time.Now().After(session.LastTimeUsed.Add(time.Second*60)) {
						delete(g.activeOutgoingDiffieHellmans, key)
					}
				}
			}
			g.activeOutgoingDiffieHellmanMutex.Unlock()

			g.activeIngoingDiffieHellmanMutex.Lock()
			for key, v := range g.activeIngoingDiffieHellmans {
				for _, session := range v {
					if session.expired() && time.Now().After(session.LastTimeUsed.Add(time.Second*5)) {
						delete(g.activeOutgoingDiffieHellmans, key)
					}
				}
			}
			g.activeIngoingDiffieHellmanMutex.Unlock()
		}
	}
}

func (g *Gossiper) getSession(sessions []*DiffieHellmanNegotiationSession, id [dht_util.IDByteSize]byte) *DiffieHellmanNegotiationSession {
	for _, session := range sessions {
		if bytes.Equal(session.ID[:], id[:]) {
			return session
		}
	}
	return nil
}

func (g *Gossiper) diffieListenRoutine(cDiffieHellman chan *dto.PacketAddressPair) {
	for pap := range cDiffieHellman {
		g.addToPeers(pap.GetSenderAddress())
		g.diffieHellmanMapMutex.Lock()
		d, f := g.diffieHellmanMap[pap.Packet.DiffieHellman.NodeID]
		g.diffieHellmanMapMutex.Unlock()
		if pap.Packet.DiffieHellman.Init {
			// Start diffiehellman process
			fmt.Println(pap.GetSenderAddress())
			go g.negotiateDiffieHellman(pap.GetSenderAddress(), pap.Packet.DiffieHellman.NodeID, pap.Packet.DiffieHellman)
		} else {
			if f && len(d) > 0 {
				go func() {
					session := g.getSession(d, pap.Packet.DiffieHellman.ID)
					if session != nil {
						session.ch <- pap.Packet.DiffieHellman
					}
				}()
			}
		}
	}
}

func (g *Gossiper) negotiateDiffieHellmanInitiator(dest string, nodeID [dht_util.IDByteSize]byte) chan ([]byte) {

	callBackChannel := make(chan []byte)

	go func() {

		token := make([]byte, dht_util.IDByteSize)
		rand.Read(token)
		var token64 [dht_util.IDByteSize]byte
		copy(token64[:], token)

		g.diffieHellmanMapMutex.Lock()
		rChannel := make(chan *dto.DiffieHellman)
		g.diffieHellmanMap[nodeID] = append(g.diffieHellmanMap[nodeID], &DiffieHellmanNegotiationSession{
			ch: rChannel,
			ID: token64,
		})
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
			ID:              token64,
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
			callBackChannel <- nil
			return
		}

		ok := g.verifyPublicKey(reply.EcdsaPublicKey, dest)
		if !ok {
			fmt.Println("Failed to verify public key.")
			callBackChannel <- nil
			return
		}
		ok = g.verifySignature(reply)
		if !ok {
			fmt.Println("Message signature not ok. Aborting diffie-hellman key exchange.3")
			callBackChannel <- nil
			return
		}

		// Send ack
		ack := g.diffieAcklowledge(dest, token64)

		// Wait for ack
		ok, _ = g.waitForAcknowledge(dest, nodeID, rChannel)
		if !ok {
			fmt.Println("failed to get ack.")
			callBackChannel <- nil
			return
		}

		symmetricKey := diffieHellman.GenerateSymmetricKey(reply.DiffiePublicKey)

		g.activeOutgoingDiffieHellmanMutex.Lock()
		g.activeOutgoingDiffieHellmans[nodeID] = append(g.activeOutgoingDiffieHellmans[nodeID], NewDiffieHellmanSession(symmetricKey, ack.ExpirationDate))
		g.activeOutgoingDiffieHellmanMutex.Unlock()

		fmt.Printf("Key setup with %s\n", dest)

		callBackChannel <- symmetricKey
	}()
	return callBackChannel
}

func (g *Gossiper) negotiateDiffieHellman(dest string, nodeID [dht_util.IDByteSize]byte, packet *dto.DiffieHellman) {

	// Get group and p used in diffie hellman
	group, b := new(big.Int).SetString(packet.G, 16)
	if !b {
		log.Fatal("Could not find group.")
	}
	p, b := new(big.Int).SetString(packet.P, 16)
	if !b {
		log.Fatal("Could not find p.")
	}
	diffieHellman := diffiehellman.New(group, p)

	// generate symmetric key
	symmetricKey := diffieHellman.GenerateSymmetricKey(packet.DiffiePublicKey)

	// Generarate one time public key used during diffie hellman
	diffiePublicKey := diffieHellman.GeneratePublicKey()

	// Create channel to map responses back to this routine.
	rChannel := make(chan *dto.DiffieHellman)
	g.diffieHellmanMapMutex.Lock()
	g.diffieHellmanMap[nodeID] = append(g.diffieHellmanMap[nodeID], &DiffieHellmanNegotiationSession{
		ch: rChannel,
		ID: packet.ID,
	})
	g.diffieHellmanMapMutex.Unlock()

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
		ID:              packet.ID,
	}
	r, s, err := g.signPacket(diffiePacket)
	if err != nil {
		fmt.Errorf("could not sign request: %v", err)
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
		return
	}

	// Send ack
	g.diffieAcklowledge(dest, packet.ID)

	// Save symmetric key
	g.activeIngoingDiffieHellmanMutex.Lock()
	g.activeIngoingDiffieHellmans[nodeID] = append(g.activeIngoingDiffieHellmans[nodeID], NewDiffieHellmanSession(symmetricKey, ack.ExpirationDate))
	g.activeIngoingDiffieHellmanMutex.Unlock()

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
func (g *Gossiper) diffieAcklowledge(dest string, id [dht_util.IDByteSize]byte) *dto.DiffieHellman {
	ack := &dto.DiffieHellman{
		EcdsaPublicKey: g.dhtMyID[:],
		NodeID:         g.dhtMyID,
		Init:           false,
		ID:             id,
		ExpirationDate: time.Now().Local().Add(time.Second * time.Duration(100))}
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
