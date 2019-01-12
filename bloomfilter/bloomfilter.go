package bloomfilter

import (
	"hash"
	"hash/fnv"
	"math"

	"github.com/spaolacci/murmur3"
)

const uint64Size = 64

type BloomFilter struct {
	bitset        []uint64
	k             uint64
	n             uint64
	m             uint64
	hashFunctions []hash.Hash64
}

// PUBLIC API

func New(k, m uint64) *BloomFilter {
	bitset := make([]uint64, int(math.Ceil(float64(m)/uint64Size)))
	return &BloomFilter{
		bitset:        bitset,
		k:             k,
		n:             0,
		m:             m,
		hashFunctions: []hash.Hash64{fnv.New64(), fnv.New64a(), murmur3.New64()},
		//hashFunctions: []hash.Hash64{fnv.New64(), fnv.New64a(), murmur3.New64()},
	}
}

func (bf *BloomFilter) Set(elem []byte) {
	if bf.IsSet(elem) {
		return
	}
	for _, hf := range bf.hashFunctions {
		hf.Reset()
		bitsetIndex, bitIndex := bf.position(elem, hf)
		bf.bitset[bitsetIndex] |= (uint64(1) << bitIndex)
	}
	for _, hf := range bf.hashFunctions {
		hf.Reset()
		hf.Write([]byte{1})
		bitsetIndex, bitIndex := bf.position(elem, hf)
		bf.bitset[bitsetIndex] |= (uint64(1) << bitIndex)
	}
	bf.n++
}

// False postive probablity = (1 - e^(-k*m/n))^k
func (bf *BloomFilter) IsSet(elem []byte) bool {
	for _, hf := range bf.hashFunctions {
		hf.Reset()
		bitsetIndex, bitIndex := bf.position(elem, hf)
		if (bf.bitset[bitsetIndex] & (uint64(1) << bitIndex)) == 0 {
			return false
		}
	}
	for _, hf := range bf.hashFunctions {
		hf.Reset()
		hf.Write([]byte{1})
		bitsetIndex, bitIndex := bf.position(elem, hf)
		if (bf.bitset[bitsetIndex] & (uint64(1) << bitIndex)) == 0 {
			return false
		}
	}
	return true
}

func (bf *BloomFilter) Size() int {
	return int(bf.n)
}

// PRIVATE METHODS

func (bf *BloomFilter) mask(bits int) uint64 {
	var mask uint64
	for i := 0; i < bits; i++ {
		mask |= (1 << uint64(i))
	}
	return mask
}

func (bf *BloomFilter) mInBits() int {
	return int(math.Log(float64(bf.m)))
}

func (bf *BloomFilter) position(elem []byte, hf hash.Hash64) (uint64, uint64) {
	hf.Write(elem)
	hashSum := hf.Sum64()
	mask := bf.mask(bf.mInBits())
	val := hashSum & mask

	bitIndex := uint64(val % uint64Size)
	bitsetIndex := uint64(math.Floor(float64(val) / uint64Size))

	return bitsetIndex, bitIndex
}
