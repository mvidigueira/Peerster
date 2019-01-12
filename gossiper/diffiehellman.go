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
	"protobuf"
	"time"

	"github.com/mvidigueira/Diffie-Hellman/diffiehellman"
	"github.com/mvidigueira/Peerster/dht_util"
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

func NewDiffieHellmanSession(key []byte) *DiffieHellmanSession {
	return &DiffieHellmanSession{
		Key:            key,
		ExpirationDate: time.Now().Local().Add(time.Second * time.Duration(60)),
	}
}

func (g *Gossiper) CleanOldDiffieHellmanSessionsRoutine() {
	for {
		select {
		case <-time.After(time.Second * 10):
			for key, v := range g.activeDiffieHellmans {
				for _, session := range v {
					if session.expired() && time.Now().After(session.LastTimeUsed.Add(time.Second*180)) {
						delete(g.activeDiffieHellmans, key)
					}
				}
			}
		}
	}
}

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

func (g *Gossiper) negotiateDiffieHellmanInitiator(dest string) chan ([]byte) {

	callBackChannel := make(chan []byte)

	go func() {
		// Create channel to map responses back to this routine.
		rChannel, f := g.diffieHellmanMap[dest]
		if !f {
			log.Fatal("key not found negitate diffie.")
		}

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
		case <-time.After(time.Second * 5):
			fmt.Println("Timeout, aborting.")
			delete(g.diffieHellmanMap, dest)
			return
		}

		ok := g.verifyPublicKey(reply.EcdsaPublicKey, dest)
		if !ok {
			fmt.Println("Failed to verify public key.")
			delete(g.diffieHellmanMap, dest)
			return
		}
		ok = g.verifySignature(reply)
		if !ok {
			fmt.Println("Message signature not ok. Aborting diffie-hellman key exchange.3")
			delete(g.diffieHellmanMap, dest)
			return
		}

		// Send ack
		g.diffieAcklowledge(dest)

		// Wait for ack
		ok = g.waitForAcknowledge(dest, rChannel)
		if !ok {
			fmt.Println("failed to get ack.")
			delete(g.diffieHellmanMap, dest)
			return
		}

		fmt.Printf("KEY LENGTH: %d\n", len(reply.DiffiePublicKey))

		symmetricKey := diffieHellman.GenerateSymmetricKey(reply.DiffiePublicKey)

		g.activeDiffieHellmans[dest] = append(g.activeDiffieHellmans[dest], NewDiffieHellmanSession(symmetricKey))

		delete(g.diffieHellmanMap, dest)

		fmt.Println(symmetricKey)

		fmt.Printf("1Key setup with %s\n", dest)

		callBackChannel <- symmetricKey
	}()
	return callBackChannel
}

func (g *Gossiper) negotiateDiffieHellman(ch chan *dto.DiffieHellman, dest string, packet *dto.DiffieHellman) {

	// Create channel to map responses back to this routine.
	rChannel := make(chan *dto.DiffieHellman)
	g.diffieHellmanMap[dest] = rChannel

	// Get group and p used in diffie hellman
	group, _ := new(big.Int).SetString(packet.G, 16)
	p, _ := new(big.Int).SetString(packet.P, 16)
	diffieHellman := diffiehellman.New(group, p)

	// generate symmetric key
	fmt.Printf("KEY LENGTH: %d\n", len(packet.DiffiePublicKey))

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
	}
	r, s, err := g.signPacket(diffiePacket)
	if err != nil {
		fmt.Errorf("could not sign request: %v", err)
		delete(g.diffieHellmanMap, dest)
		return
	}
	diffiePacket.S = s
	diffiePacket.R = r

	g.sendUDP(&dto.GossipPacket{
		DiffieHellman: diffiePacket,
	}, dest)

	// Wait for ack
	ok := g.waitForAcknowledge(dest, rChannel)
	if !ok {
		fmt.Println("failed to get ack.")
		delete(g.diffieHellmanMap, dest)
		return
	}

	// Send ack
	g.diffieAcklowledge(dest)

	// Save symmetric key
	g.activeDiffieHellmans[dest] = append(g.activeDiffieHellmans[dest], NewDiffieHellmanSession(symmetricKey))

	// Clean up
	delete(g.diffieHellmanMap, dest)

	fmt.Println(symmetricKey)
	fmt.Printf("1Key setup with %s\n", dest)
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
func (g *Gossiper) diffieAcklowledge(dest string) {
	ack := &dto.DiffieHellman{EcdsaPublicKey: g.dhtMyID[:]}
	r, s, err := g.signPacket(ack)
	if err != nil {
		log.Fatal("could not sign request")
	}
	ack.S = s
	ack.R = r
	g.sendUDP(&dto.GossipPacket{
		DiffieHellman: ack,
	}, dest)
}

func (g *Gossiper) waitForAcknowledge(dest string, responseChan chan *dto.DiffieHellman) bool {
	select {
	case res := <-responseChan:
		ok := g.verifyPublicKey(res.EcdsaPublicKey, dest)
		if !ok {
			fmt.Println("Failed to verify public key.")
			return false
		}
		ok = g.verifySignature(res)
		if !ok {
			fmt.Println("Message signature not ok. Aborting diffie-hellman key exchange. 5")
			return false
		}
		return true
	case <-time.After(time.Second * 10):
		fmt.Println("Timeout, aborting.")
		delete(g.diffieHellmanMap, dest)
		return false
	}
}

func (g *Gossiper) extractEcdsaPublicKeyComponents(packet *dto.DiffieHellman) (*big.Int, *big.Int) {
	x := new(big.Int)
	x.SetBytes(packet.EcdsaPublicKey[:32])
	y := new(big.Int)
	y.SetBytes(packet.EcdsaPublicKey[32:])
	return x, y
}
