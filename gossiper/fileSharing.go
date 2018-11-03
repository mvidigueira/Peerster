package gossiper

import (
	"crypto/sha256"
	"fmt"
	"time"

	"github.com/mvidigueira/Peerster/dto"
	"github.com/mvidigueira/Peerster/fileparsing"
)

const downloadChunkTimeout = 5

func (g *Gossiper) clientFileShareListenRoutine(cFileShareUI chan string) {
	for fileName := range cFileShareUI {
		fmt.Printf("clientFileShareListenRoutine: Share request for file: %s\n", fileName)

		chunks, size, err := fileparsing.ReadChunks(fileName)
		if err != nil {
			fmt.Println(err)
			continue
		}
		chunksMap, metafile, metahash := fileparsing.CreateChunksMap(chunks)
		g.fileMap.AddEntry(fileName, size, metafile, metahash)

		for checksum, chunk := range chunksMap {
			g.chunkMap.AddChunk(checksum, chunk)
		}

		fmt.Printf("Share request COMPLETED. File checksum: %x\n", metahash)
	}
}

func (g *Gossiper) dataRequestListenRoutine(cDataRequest chan *dto.PacketAddressPair) {
	for pap := range cDataRequest {
		g.addToPeers(pap.GetSenderAddress())
		g.printKnownPeers()

		if pap.GetDestination() == g.name {
			drep, ok := g.answerDataRequest(pap.GetOrigin(), pap.GetHashValue())
			if ok {
				replyPacket := &dto.GossipPacket{DataReply: drep}
				g.forward(replyPacket)
			}
		} else {
			g.forward(pap.Packet)
		}

		g.updateDSDV(pap) //routing
	}
}

func (g *Gossiper) dataReplyListenRoutine(cDataReply chan *dto.PacketAddressPair) {
	for pap := range cDataReply {
		g.addToPeers(pap.GetSenderAddress())
		g.printKnownPeers()

		if pap.GetDestination() == g.name {
			//stuff
		} else {
			if verifyDataReply(pap.Packet.DataReply) {
				g.forward(pap.Packet)
			} else {
				fmt.Printf("Error: incorrect hash value found in DataReply\n")
			}
		}

		g.updateDSDV(pap) //routing
	}
}

func (g *Gossiper) clientFileDownloadListenRoutine(cFileDownloadUI chan *dto.FileToDownload) {
	for fileToDL := range cFileDownloadUI {
		fmt.Printf("Download request for file with metahash: %x\n", fileToDL.GetMetahash())

		go g.downloadFile(fileToDL.GetFileName(), fileToDL.GetMetahash(), fileToDL.GetOrigin())
	}
}

func (g *Gossiper) downloadFile(nameToSave string, metahash [32]byte, origin string) {
	alreadyDownloading := !(g.dlFilesSet.AddUnique(metahash))
	if alreadyDownloading {
		fmt.Printf("File with metahash %x is already being downloaded. Please wait...\n", metahash)
		return
	}

	fmt.Printf("DOWNLOADING metafile of %s from %s\n", nameToSave, origin)
	metafile := g.downloadChunk(metahash, origin)

	chunkHashes, ok := fileparsing.ParseMetafile(metafile)
	if !ok {
		fmt.Printf("Download FAILED\n")
		return
	}

	chunks := make([][]byte, len(chunkHashes))
	var size = 0
	for i, hash := range chunkHashes {
		fmt.Printf("DOWNLOADING %s chunk %d from %s\n", nameToSave, i-1, origin)
		chunks[i] = g.downloadChunk(hash, origin)
		size += len(chunks[i])
		g.chunkMap.AddChunk(hash, chunks[i])
	}

	fileparsing.WriteFileFromChunks(nameToSave, chunks)
	g.fileMap.AddEntry(nameToSave, size, metafile, metahash)
	fmt.Printf("RECONSTRUCTED file %s\n", nameToSave)

	g.dlFilesSet.Delete(metahash)
}

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

func (g *Gossiper) makeDataRequest(destination string, checksum [32]byte) *dto.DataRequest {
	dataRequest := dto.DataRequest{
		Origin:      g.name,
		Destination: destination,
		HopLimit:    defaultHopLimit,
		HashValue:   checksum[:],
	}
	return &dataRequest
}

func (g *Gossiper) answerDataRequest(origin string, hash []byte) (dataReply *dto.DataReply, ok bool) {
	if len(hash) != 32 {
		fmt.Printf("SHA256 hash length mismatch. Is %v but should be 32.\n", len(hash))
		return nil, false
	}
	var hash32 [32]byte
	copy(hash32[:], hash)

	sfe, ok := g.fileMap.GetEntry(hash32)
	if ok {
		dataReply = g.makeDataReply(origin, sfe.GetMetafile(), hash32)
	} else {
		chunk, ok := g.chunkMap.GetChunk(hash32)
		if !ok {
			fmt.Printf("No valid metafile/chunk found matching hash value '%x'\n", hash32)
			return nil, false
		}
		dataReply = g.makeDataReply(origin, chunk, hash32)
	}
	return
}

func verifyDataReply(dataReply *dto.DataReply) (ok bool) {
	if len(dataReply.HashValue) != 32 {
		fmt.Printf("SHA256 hash length mismatch. Is %v but should be 32.\n", len(dataReply.HashValue))
		return false
	}
	var hash32 [32]byte
	copy(hash32[:], dataReply.HashValue)
	return hash32 == sha256.Sum256(dataReply.Data)
}
