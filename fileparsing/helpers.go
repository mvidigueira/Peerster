package fileparsing

import (
	"bufio"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"strings"
)

const shareBaseDir = "./_SharedFiles/"
const dlBaseDir = "./_Downloads/"
const defaultChunkSize = 8192 //8KiB or 8KB? Assumed KiB

//ReadChunks - parses the file with the given name at 'shareBaseDir' into 'defaultChunkSize' sized chunks.
//Returns the array of chunks, the total size (in bytes) of the file, and an error (if any ocurred).
//Does NOT pad the last chunk.
func ReadChunks(fileName string) (chunks [][]byte, size int, err error) {
	f, err := os.Open(shareBaseDir + fileName)
	if err != nil {
		return nil, 0, err
	}

	chunks = make([][]byte, 0)
	size = 0
	r := bufio.NewReader(f)
	var chunkSize = 0
	for err == nil {
		chunk := make([]byte, defaultChunkSize)
		chunkSize, err = r.Read(chunk)
		if chunkSize > 0 {
			size = size + chunkSize
			chunks = append(chunks, chunk[0:chunkSize])
		}
	}
	if err != io.EOF {
		return nil, 0, err
	}

	return chunks, size, nil
}

//WriteFileFromChunks - takes an array of chunks and writes them to file named 'name' at directiory 'dlBaseDir'
//returns true if it successful, false otherwise
func WriteFileFromChunks(name string, chunks [][]byte) (ok bool) {
	f, err := os.Create(dlBaseDir + name)
	if err != nil {
		fmt.Println(err)
		return false
	}
	defer f.Close()

	w := bufio.NewWriter(f)

	for _, chunk := range chunks {
		_, err := w.Write(chunk)
		if err != nil {
			fmt.Println(err)
			return false
		}
	}

	err = w.Flush()
	if err != nil {
		fmt.Println(err)
		return false
	}

	return true
}

//CreateChunksMap - takes an array of chunks and returns a checksum-chunk map.
//Each checksum is created by applying SHA256 to each chunk.
//Also returns an array of bytes with the concatenated checksums (metafile), and a hash (also SHA256) of this array.
func CreateChunksMap(chunks [][]byte) (chunkMap map[[32]byte][]byte, metafile []byte, metahash [32]byte) {
	chunkMap = make(map[[32]byte][]byte)
	metafile = make([]byte, 32*len(chunks))

	for i, chunk := range chunks {
		sum := sha256.Sum256(chunk)
		chunkMap[sum] = chunk

		for j, sumj := range sum {
			metafile[i*32+j] = sumj
		}
	}

	metahash = sha256.Sum256(metafile)

	return chunkMap, metafile, metahash
}

//ParseMetafile - breaks the metafile into 32-byte arrays
func ParseMetafile(metafile []byte) (checksums [][32]byte, ok bool) {
	if len(metafile)%32 != 0 {
		fmt.Printf("Bad metafile: Undivisable by 32\n")
		return nil, false
	}
	checksums = make([][32]byte, len(metafile)/32)
	for i := range checksums {
		copy(checksums[i][:], metafile[i*32:(i+1)*32])
	}
	return checksums, true
}

//ConvertToHash32 - Converts a slice of undetermined size to a 32-byte array version
//Returns the 32-byte array, and true if the conversion was successful (false otherwise)
func ConvertToHash32(hash []byte) (hash32 [32]byte, ok bool) {
	if len(hash) != 32 {
		fmt.Printf("SHA256 hash length mismatch. Is %v but should be 32.\n", len(hash))
		ok = false
		return
	}
	copy(hash32[:], hash)
	return hash32, true
}

//VerifyDataHash - returns true if the provided 'hash' matches the 'data''s SHA256 hash
func VerifyDataHash(hash []byte, data []byte) (ok bool) {
	if len(hash) != 32 {
		fmt.Printf("SHA256 hash length mismatch. Is %v but should be 32.\n", len(hash))
		return false
	}
	var hash32 [32]byte
	copy(hash32[:], hash)
	return hash32 == sha256.Sum256(data)
}

//EstimateFileSize - provides an estimate of the file size (always greater than actual size)
func EstimateFileSize(metafile []byte) (estimatedSize int) {
	if len(metafile)%32 != 0 {
		fmt.Printf("Bad metafile: Undivisable by 32\n")
		return -1
	}
	return len(metafile) / 32 * defaultChunkSize
}

//ContainsKeyword - returns true if 'name' has a substring matching one of the keywords, false otherwise.
func ContainsKeyword(name string, keywords []string) bool {
	for _, keyword := range keywords {
		if strings.Contains(name, keyword) {
			return true
		}
	}
	return false
}
