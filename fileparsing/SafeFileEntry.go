package fileparsing

//SafeFileEntry - for keeping information per file
type SafeFileEntry struct {
	name     string
	size     int
	metafile []byte
	metahash [32]byte
}

//NewSafeFileEntry - for the creation of SafeFileEntry
func NewSafeFileEntry(name string, size int, metafile []byte, metahash [32]byte) *SafeFileEntry {
	return &SafeFileEntry{name: name, size: size, metafile: metafile, metahash: metahash}
}

//GetName - returns the SafeFileEntry's file name
func (sfe *SafeFileEntry) GetName() string {
	return sfe.name
}

//GetSize - returns the SafeFileEntry's file size
func (sfe *SafeFileEntry) GetSize() int {
	return sfe.size
}

//GetMetafile - returns the SafeFileEntry's metafile
func (sfe *SafeFileEntry) GetMetafile() []byte {
	return sfe.metafile
}

//GetMetahash - returns the SafeFileEntry's metafile's hash (metahash)
func (sfe *SafeFileEntry) GetMetahash() [32]byte {
	return sfe.metahash
}

/*
chunks, size, err := readChunks(name)
	if err != nil {
		return nil, err
	}
	chunksMap, metafile, metahash := createChunksMap(chunks)
*/
