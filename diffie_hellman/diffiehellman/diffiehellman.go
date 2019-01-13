package diffiehellman

import (
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"log"
	"math/big"
)

type DiffieHellmanPacket struct {
	G *big.Int
	P *big.Int
}

type DiffieHellman struct {
	g      *big.Int
	p      *big.Int
	secret *big.Int
}

/*
	The value of p and g has been choosen according to the RFC3526 recommendations
 	https://www.ietf.org/rfc/rfc3526.txt
*/
func New(group, p *big.Int) *DiffieHellman {
	return &DiffieHellman{
		g:      group,
		p:      p,
		secret: secret(),
	}
}

func Group() *big.Int {
	return new(big.Int).SetInt64(2)
}

func P() *big.Int {
	return p()
}

/*
	Public API
*/

func (dh *DiffieHellman) GeneratePublicKey() []byte {
	fmt.Println("BEFORE")
	return new(big.Int).Exp(dh.g, dh.secret, dh.p).Bytes()
}

func (dh *DiffieHellman) GenerateSymmetricKey(publicKey []byte) []byte {
	pk := new(big.Int).SetBytes(publicKey)
	sharedSecret := new(big.Int).Exp(pk, dh.secret, dh.p)
	sharedKey := sha256.Sum256(sharedSecret.Bytes())
	return sharedKey[:]
}

/*
	Private
*/

func secret() *big.Int {
	var secret [256]byte
	_, err := rand.Read(secret[:])
	if err != nil {
		log.Fatal(err)
	}
	return new(big.Int).SetBytes(secret[:])
}

func p() *big.Int {
	p, ok := new(big.Int).SetString(
		"FFFFFFFFFFFFFFFFC90FDAA22168C234C4C6628B80DC1CD1"+
			"29024E088A67CC74020BBEA63B139B22514A08798E3404DD"+
			"EF9519B3CD3A431B302B0A6DF25F14374FE1356D6D51C245"+
			"E485B576625E7EC6F44C42E9A637ED6B0BFF5CB6F406B7ED"+
			"EE386BFB5A899FA5AE9F24117C4B1FE649286651ECE45B3D"+
			"C2007CB8A163BF0598DA48361C55D39A69163FA8FD24CF5F"+
			"83655D23DCA3AD961C62F356208552BB9ED529077096966D"+
			"670C354E4ABC9804F1746C08CA18217C32905E462E36CE3B"+
			"E39E772C180E86039B2783A2EC07A28FB5C55DF06F4C52C9"+
			"DE2BCBF6955817183995497CEA956AE515D2261898FA0510"+
			"15728E5A8AACAA68FFFFFFFFFFFFFFFF", 16)
	if !ok {
		log.Fatal("Failed to generate p.")
	}
	return p
}
