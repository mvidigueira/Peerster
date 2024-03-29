package gossiper

import (
	"fmt"
	"time"

	"github.com/mvidigueira/Peerster/dto"
	"github.com/mvidigueira/Peerster/fileparsing"
)

const downloadChunkTimeout = 5

//makeDataReply - returns a reference to a DataReply with corresponding 'destination', 'data' and 'checksum'
//Origin is the gossiper's name, and the HopLimit is the dafaultHopLimit (DSDV.go)
func (g *Gossiper) makeDataReply(destination string, data []byte, checksum [32]byte) *dto.DataReply {
	dataReply := dto.DataReply{
		Origin:      g.name,
		Destination: destination,
		HopLimit:    defaultHopLimit,
		HashValue:   checksum[:],
		Data:        data,
	}
	return &dataReply
}

//makeDataRequest - returns a reference to a DataRequest with corresponding 'destination' and 'checksum'
//Origin is the gossiper's name, and the HopLimit is the dafaultHopLimit (DSDV.go)
func (g *Gossiper) makeDataRequest(destination string, checksum [32]byte) *dto.DataRequest {
	dataRequest := dto.DataRequest{
		Origin:      g.name,
		Destination: destination,
		HopLimit:    defaultHopLimit,
		HashValue:   checksum[:],
	}
	return &dataRequest
}

//answerDataRequest - constructs a dataReply to peer 'origin' with the data
//from the requested chunk/metafile with hash value 'hash'.
func (g *Gossiper) answerDataRequest(origin string, hash []byte) (dataReply *dto.DataReply, ok bool) {
	hash32, ok := fileparsing.ConvertToHash32(hash)
	if !ok {
		return nil, false
	}

	chunk, ok := g.chunkMap.GetChunk(hash32)
	if !ok {
		fmt.Printf("No valid metafile/chunk found matching hash value '%x'\n", hash32)
		return nil, false
	}
	dataReply = g.makeDataReply(origin, chunk, hash32)

	return dataReply, true
}

//downloadFile - downloads chunk identified with 'hash' from peer 'origin'
//Returns the data ([]byte) associated with that chunk
func (g *Gossiper) downloadChunk(hash [32]byte, origin string) (data []byte) {
	cDataReply, _ := g.dlChunkListeners.AddListener(hash) //TODO: add collision message if !isNew

	dataRequest := g.makeDataRequest(origin, hash)
	packet := &dto.GossipPacket{DataRequest: dataRequest}
	g.forward(packet)

	t := time.NewTicker(downloadChunkTimeout * time.Second)
	for {
		select {
		case <-t.C:
			g.forward(packet)
		case data := <-cDataReply:
			t.Stop()
			return data
		}
	}
}

//downloadFile - downloads file identified with 'metahash' from peer 'origin', saving it with name 'nameToSave'
//Returns true if the download was successful, false otherwise.
func (g *Gossiper) downloadFile(nameToSave string, metahash [32]byte, origin string) (success bool) {
	alreadyDownloading := !(g.dlFilesSet.AddUnique(metahash))
	if alreadyDownloading {
		fmt.Printf("File with metahash %x is already being downloaded. Please wait...\n", metahash)
		return false
	}

	randomOrigins := (origin == "")

	var chunkOwnersMap map[uint64]*dto.SafeStringArray
	if randomOrigins {
		cm, hasTotalMatch, _ := g.metahashToChunkOwnersMap.GetMapCopy(metahash)
		chunkOwnersMap = cm
		if !hasTotalMatch {
			fmt.Printf("Error: Attempt to download file with no total matches.\n")
			return false
		}
		origin = pickRandom(chunkOwnersMap[1].GetArrayCopy())
	}

	fmt.Printf("DOWNLOADING metafile of %s from %s\n", nameToSave, origin)
	metafile := g.downloadChunk(metahash, origin)
	chunkHashes, ok := fileparsing.ParseMetafile(metafile)
	if !ok {
		fmt.Printf("Invalid Metafile. Download FAILED\n")
		g.dlFilesSet.Delete(metahash)
		return false
	}

	g.chunkMap.AddChunk(metahash, metafile)
	estimatedSize := fileparsing.EstimateFileSize(metafile)
	g.fileMap.AddEntry(nameToSave, estimatedSize, metafile, metahash)
	sfe, _ := g.fileMap.GetEntry(metahash)

	chunks := make([][]byte, len(chunkHashes))
	var actualSize = 0
	for i, hash := range chunkHashes {
		if randomOrigins {
			origin = pickRandom(chunkOwnersMap[uint64(i+1)].GetArrayCopy())
		}
		fmt.Printf("DOWNLOADING %s chunk %d from %s\n", nameToSave, i+1, origin)
		chunks[i] = g.downloadChunk(hash, origin)
		actualSize += len(chunks[i])
		g.chunkMap.AddChunk(hash, chunks[i]) //avoids reparsing whole file
		sfe.SafeAddChunkIndex(uint64(i + 1)) //TODO: check if + 1 is necessary or not
	}

	sfe.SafeSetSize(actualSize)
	if fileparsing.WriteFileFromChunks(nameToSave, chunks, randomOrigins) {
		fmt.Printf("RECONSTRUCTED file %s\n", nameToSave)
	} else {
		fmt.Printf("Error writing file %s to disk\n", nameToSave)
	}

	g.dlFilesSet.Delete(metahash)
	return true
}

//dataRequestListenRoutine - deals with DataRequest messages from other peers
func (g *Gossiper) dataRequestListenRoutine(cDataRequest chan *dto.PacketAddressPair) {
	for pap := range cDataRequest {
		g.addToPeers(pap.GetSenderAddress())
		g.printKnownPeers()

		fmt.Printf("DATA REQUEST from %s for hashvalue %x\n", pap.GetOrigin(), pap.GetHashValue())

		if pap.GetDestination() == g.name {
			drep, ok := g.answerDataRequest(pap.GetOrigin(), pap.GetHashValue())
			if ok {
				replyPacket := &dto.GossipPacket{DataReply: drep}
				g.forward(replyPacket)
			}
		} else {
			g.forward(pap.Packet)
		}
	}
}

//dataReplyListenRoutine - deals with DataReply messages from other peers
func (g *Gossiper) dataReplyListenRoutine(cDataReply chan *dto.PacketAddressPair) {
	for pap := range cDataReply {
		g.addToPeers(pap.GetSenderAddress())
		g.printKnownPeers()

		fmt.Printf("DATA REPLY from %s for hashvalue %x\n", pap.GetOrigin(), pap.GetHashValue())

		if !fileparsing.VerifyDataHash(pap.Packet.DataReply.HashValue, pap.Packet.DataReply.Data) {
			fmt.Printf("Error: incorrect hash value found in DataReply\n")
			continue
		}

		if pap.GetDestination() == g.name {
			hash32, ok := fileparsing.ConvertToHash32(pap.GetHashValue())
			if ok {
				g.dlChunkListeners.InformListener(hash32, pap.GetData())
			}
		} else {
			g.forward(pap.Packet)
		}
	}
}

//clientFileShareListenRoutine - deals with new fileShare messages from clients
func (g *Gossiper) clientFileShareListenRoutine(cFileShareUI chan string, cFileNaming chan *dto.PacketAddressPair) {
	for fileName := range cFileShareUI {
		fmt.Printf("Share request for file: %s\n", fileName)

		chunks, size, err := fileparsing.ReadChunks(fileName)
		if err != nil {
			fmt.Println(err)
			continue
		}
		chunksMap, metafile, metahash := fileparsing.CreateChunksMap(chunks)
		g.fileMap.AddEntry(fileName, size, metafile, metahash)

		g.chunkMap.AddChunk(metahash, metafile)
		for checksum, chunk := range chunksMap {
			g.chunkMap.AddChunk(checksum, chunk)
		}

		fmt.Printf("Share request COMPLETED. File checksum: %x\n", metahash)

		//blockchain
		file := dto.File{Name: fileName, Size: int64(size), MetafileHash: metahash[:]}
		txPub := &dto.TxPublish{File: file, HopLimit: defaultHopLimit}
		packet := &dto.GossipPacket{TxPublish: txPub}
		pap := &dto.PacketAddressPair{Packet: packet, SenderAddress: ""}
		cFileNaming <- pap
	}
}

//clientFileShareListenRoutine - deals with new fileDownload messages from clients
func (g *Gossiper) clientFileDownloadListenRoutine(cFileDownloadUI chan *dto.FileToDownload) {
	for fileToDL := range cFileDownloadUI {
		fmt.Printf("Download request for file with metahash: %x\n", fileToDL.GetMetahash())

		go g.downloadFile(fileToDL.GetFileName(), fileToDL.GetMetahash(), fileToDL.GetOrigin())
	}
}
