package dht

import (
	"crypto/rand"
	"fmt"
	"math/bits"

	. "github.com/mvidigueira/Peerster/dht_util"
)

func xorDistance(id1, id2 TypeID) (result TypeID) {
	for i := range result {
		result[i] = id1[i] ^ id2[i]
	}
	return
}

// id1 > id2  -->  1
// id1 = id2  -->  0
// id1 < id2  --> -1
func compare(id1, id2 TypeID) int {
	for i, v := range id1 {
		if v < id2[i] {
			return -1
		} else if v > id2[i] {
			return 1
		}
	}
	return 0
}

// returns true if id2 is closer to target than id1, false otherwise
func isCloser(target, id1, id2 TypeID) (id2Closer bool) {
	id1C := xorDistance(target, id1)
	id2C := xorDistance(target, id2)
	return compare(id1C, id2C) > 0
}

// prevClosest must be ordered from closest to furthest away
func InsertOrdered(target TypeID, prevClosest []*NodeState, potential *NodeState) (closest []*NodeState) {
	for i, prev := range prevClosest {
		if isCloser(target, prev.NodeID, potential.NodeID) {
			temp := append([]*NodeState{potential}, prevClosest[i:]...)
			closest = append(prevClosest[:i], temp...)
			return
		}
	}
	closest = append(prevClosest, potential)
	return
}

func RandNodeID(base TypeID, leadingBitsInCommon int) (generated TypeID) {
	rand.Read(generated[:])
	//fmt.Printf("Generated: %s\n", generated)
	copy(generated[:leadingBitsInCommon/8], base[:leadingBitsInCommon/8])
	//fmt.Printf("After Copy: %s\n", generated)

	b1 := byte(0x80) >> uint(leadingBitsInCommon%8) //selects bit
	b2 := base[leadingBitsInCommon/8] & b1          //mask
	b2 = ^b2                                        //reverse
	b2 &= b1                                        //mask

	if leadingBitsInCommon%8 > 0 {
		b := byte(0xFF) >> uint(leadingBitsInCommon%8)
		generated[leadingBitsInCommon/8] &= b
		base[leadingBitsInCommon/8] &= ^b
		generated[leadingBitsInCommon/8] |= base[leadingBitsInCommon/8]
	}

	generated[leadingBitsInCommon/8] &= ^b1 //cleans bit
	generated[leadingBitsInCommon/8] |= b2  //sets bit

	//fmt.Printf("After Second Copy: %s\n", generated)
	return
}

func InitialRandNodeID() (generated TypeID) {
	rand.Read(generated[:])
	fmt.Printf("Generated ID: %x\n", generated)
	return
}

func CommonLeadingBits(id1, id2 TypeID) (inCommon int) {
	for i, v := range id1 {
		xored := v ^ id2[i]
		if xored != 0 {
			inCommon += bits.LeadingZeros8(xored)
			return
		}
		inCommon += 8
	}

	return
}
