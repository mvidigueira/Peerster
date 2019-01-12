package dht_util

import (
	"crypto/sha512"
	"fmt"
)

const IDByteSize = 512 / 8

type TypeID [IDByteSize]byte

//ConvertToTypeID - Converts a slice of undetermined size to TypeID
//Returns the TypeID, and true if the conversion was successful (false otherwise)
func ConvertToTypeID(idB []byte) (id TypeID, ok bool) {
	if len(idB) != IDByteSize {
		fmt.Printf("ID length mismatch. Is %v but should be %v.\n", len(idB), IDByteSize)
		ok = false
		return
	}
	copy(id[:], idB)
	return id, true
}

// Converts an arbitrary long key string to a 20byte hexa hash key.
func GenerateKeyHash(key string) [IDByteSize]byte {
	byteKey := []byte(key)
	hash := sha512.Sum512(byteKey)
	//key = hex.EncodeToString(hash[:])
	return hash
}
