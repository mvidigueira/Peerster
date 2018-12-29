package dht

import (
	"crypto/rand"
	"math/bits"
)

func xorDistance(id1, id2 [IDByteSize]byte) (result [IDByteSize]byte) {
	for i := range result {
		result[i] = id1[i] ^ id2[i]
	}
	return
}

// id1 > id2  -->  1
// id1 = id2  -->  0
// id1 < id2  --> -1
func compare(id1, id2 [IDByteSize]byte) int {
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
func isCloser(target, id1, id2 [IDByteSize]byte) (id2Closer bool) {
	id1C := xorDistance(target, id1)
	id2C := xorDistance(target, id2)
	return compare(id1C, id2C) > 0
}

// prevClosest must be ordered from closest to furthest away
func InsertOrdered(target [IDByteSize]byte, prevClosest []*NodeState, potential *NodeState) (closest []*NodeState) {
	for i, prev := range prevClosest {
		if isCloser(target, prev.NodeID, potential.NodeID) {
			temp := append([]*NodeState{potential}, prevClosest[i:]...)
			closest = append(prevClosest[:i], temp...)
			return
		}
	}
	closest = prevClosest
	return
}

func RandNodeID(base [IDByteSize]byte, leadingBitsInCommon int) (generated [IDByteSize]byte) {
	rand.Read(generated[:])
	copy(generated[:leadingBitsInCommon/8+1], base[:leadingBitsInCommon/8+1])
	if leadingBitsInCommon%8 > 0 {
		b := byte(0xFF) >> uint(leadingBitsInCommon%8)
		generated[leadingBitsInCommon/8+1] &= b
		base[leadingBitsInCommon/8+1] &= ^b
		generated[leadingBitsInCommon/8+1] |= base[leadingBitsInCommon/8+1]
	}
	return
}

func InitialRandNodeID() (generated [IDByteSize]byte) {
	rand.Read(generated[:])
	return
}

func CommonLeadingBits(id1, id2 [IDByteSize]byte) (inCommon int) {
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
