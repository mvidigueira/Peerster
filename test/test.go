package main

import (
	"fmt"
	"os"
	"crypto/dsa"
	//"crypto/ecdsa"
	"crypto/rand"
	//"crypto/elliptic"
)

func main() {
	fmt.Println("Hello, playground")
	params := new(dsa.Parameters)

	if err := dsa.GenerateParameters(params, rand.Reader, dsa.L1024N160); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	privatekey := new(dsa.PrivateKey)
	privatekey.PublicKey.Parameters = *params
	dsa.GenerateKey(privatekey, rand.Reader) // this generates a public & private key pair
	
	fmt.Println(len(privatekey.X.Bytes()))
	fmt.Println(len(privatekey.PublicKey.Y.Bytes()))
}
