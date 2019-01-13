package main

import (
	"fmt"
	"math/big"

	"github.com/mvidigueira/Diffie-Hellman/aesencryptor"
	"github.com/mvidigueira/Diffie-Hellman/diffiehellman"
)

type DiffieHellman struct {
	g      *big.Int
	p      *big.Int
	secret *big.Int
}

func main() {
	diffieHellmanAlice := diffiehellman.New(diffiehellman.Group(), diffiehellman.P())
	diffieHellmanBob := diffiehellman.New(diffiehellman.Group(), diffiehellman.P())

	pkAlice := diffieHellmanAlice.GeneratePublicKey()
	pkBob := diffieHellmanBob.GeneratePublicKey()

	symmetricKeyAlice := diffieHellmanAlice.GenerateSymmetricKey(pkBob)
	symmetricKeyBob := diffieHellmanBob.GenerateSymmetricKey(pkAlice)

	fmt.Println(len(symmetricKeyAlice))

	aesAlice := aesencryptor.New(symmetricKeyAlice)
	aesBob := aesencryptor.New(symmetricKeyBob)

	message1 := "Winter is comming!"

	cipherText := aesAlice.Encrypt([]byte(message1))
	fmt.Println(cipherText)

	plainText := aesBob.Decrypt(cipherText)
	fmt.Println(string(plainText))
}

/*
func generatePrime(size int) *int {
	var nonce [128]byte
	_, err := rand.Read(nonce[:])
	if err != nil {
		fmt.Println(err)
	}
	fmt.Printf("Nonce: %s\n", hex.EncodeToString(nonce[:]))
	reader := bytes.NewReader(nonce[:])
	p, err := rand.Prime(reader, 64)
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(p)
}
*/
