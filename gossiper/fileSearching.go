package gossiper

import (
	"fmt"
	"strings"
	"time"

	"github.com/mvidigueira/Peerster/fileparsing"
	"github.com/mvidigueira/Peerster/filesearching"

	"github.com/mvidigueira/Peerster/dto"
)

const maximumBudget = 32
const budgetIncreaseTimeout = 1
const searchTimeout = 20

//makeSearchRequest - returns a reference to a SearchReply with corresponding 'budget', 'keywords'
//Origin is the gossiper's name
func (g *Gossiper) makeSearchRequest(budget uint64, keywords []string) *dto.SearchRequest {
	searchRequest := dto.SearchRequest{
		Origin:   g.name,
		Budget:   budget,
		Keywords: keywords,
	}
	return &searchRequest
}

//makeSearchReply - returns a reference to a SearchReply with corresponding 'destination', 'sress'
//Origin is the gossiper's name, and the HopLimit is the dafaultHopLimit (DSDV.go)
func (g *Gossiper) makeSearchReply(destination string, sress []*dto.SearchResult) *dto.SearchReply {
	searchReply := dto.SearchReply{
		Origin:      g.name,
		Destination: destination,
		HopLimit:    defaultHopLimit,
		Results:     sress,
	}
	return &searchReply
}

//answerDataRequest - constructs a dataReply to peer 'origin' with the data
//from the requested chunk/metafile with hash value 'hash'.
func (g *Gossiper) answerSearchRequest(origin string, keywords []string) (searchReply *dto.SearchReply, ok bool) {
	sress := g.fileMap.GetMatches(keywords)
	if len(sress) > 0 {
		searchReply = g.makeSearchReply(origin, sress)
		return searchReply, true
	}
	return nil, false
}

//forwardSearchRequest - forwards search request, with correct budget splitting, to neighbors
func (g *Gossiper) forwardSearchRequest(sreq *dto.SearchRequest, recentSender string) {
	peers := stringArrayDifference(g.peers.GetArrayCopy(), []string{sreq.Origin, recentSender})
	if len(peers) == 0 {
		fmt.Printf("Dead end. Cannot forward search request further.\n")
		return
	}
	eqSplit := int(sreq.Budget) / len(peers)
	remainder := int(sreq.Budget) % len(peers)
	for _, peer := range peers {
		budget := eqSplit
		if remainder > 0 {
			remainder--
			budget++
		}
		if budget > 0 {
			sreq.Budget = uint64(budget)
			packet := &dto.GossipPacket{SearchRequest: sreq}
			fmt.Printf("FORWARDING Search Request. To: %v, Keywords: %v, Budget: %v\n", peer, packet.GetKeywords(), packet.GetBudget())
			g.sendUDP(packet, peer)
		}
	}
}

//SearchRequestTimedID - For keeping track of duplicate recent searches
type SearchRequestTimedID struct {
	Time     time.Time
	Origin   string
	Keywords []string
}

//searchRequestListenRoutine - deals with DataRequest messages from other peers
func (g *Gossiper) searchRequestListenRoutine(cSearchRequest chan *dto.PacketAddressPair) {
	timedIDs := make(map[*SearchRequestTimedID]bool, 0) //used as a set
	for pap := range cSearchRequest {
		g.addToPeers(pap.GetSenderAddress())

		fmt.Printf("RECEIVED SEARCH REQUEST from %v, keywords: %v, budget: %v\n", pap.GetOrigin(), pap.GetKeywords(), pap.GetBudget())

		current := time.Now()
		ignore := false
		for srtID := range timedIDs {
			if current.Sub(srtID.Time).Seconds() > 0.5 {
				delete(timedIDs, srtID)
			} else if srtID.Origin == pap.GetOrigin() &&
				len(stringArrayDifference(srtID.Keywords, pap.GetKeywords())) == 0 &&
				len(stringArrayDifference(pap.GetKeywords(), srtID.Keywords)) == 0 {
				fmt.Printf("Repeat. Ignoring...\n")
				ignore = true
			}
		}

		if !ignore {
			srtID := &SearchRequestTimedID{Time: current, Origin: pap.GetOrigin(), Keywords: pap.GetKeywords()}
			timedIDs[srtID] = true

			srep, ok := g.answerSearchRequest(pap.GetOrigin(), pap.GetKeywords())
			if ok {
				replyPacket := &dto.GossipPacket{SearchReply: srep}
				g.forward(replyPacket)
			}

			sreq := pap.Packet.SearchRequest
			sreq.Budget--
			g.forwardSearchRequest(sreq, pap.GetSenderAddress())
		}

	}
}

func printSearchFound(filename string, peer string, metahash [32]byte, chunksList []uint64) {
	chunks := strings.Trim(strings.Join(strings.Fields(fmt.Sprint(chunksList)), ","), "[]")
	fmt.Printf("FOUND match %s at %s metafile=%x chunks=%s\n", filename, peer, metahash, chunks)
}

//searchReplyListenRoutine - deals with DataReply messages from other peers
func (g *Gossiper) searchReplyListenRoutine(cDataReply chan *dto.PacketAddressPair) {
	for pap := range cDataReply {
		g.addToPeers(pap.GetSenderAddress())

		fmt.Printf("RECEIVED SEARCH REPLY to %v, hops: %v\n", pap.GetDestination(), pap.GetHopLimit())

		if pap.GetDestination() == g.name {
			//fmt.Printf("THIS IS THE DESTINATION\n")
			sress := pap.GetSearchResults()
			for _, sres := range sress {
				hash32, ok := fileparsing.ConvertToHash32(sres.MetafileHash)
				if ok {
					g.filenamesMap.AddMapping(hash32, sres.FileName)
				}

				var madeAnyMatch bool
				madeTotalMatch, hasTotalMatch := g.metahashToChunkOwnersMap.UpdateWithSearchResult(pap.GetOrigin(), sres)
				if hasTotalMatch {
					g.matchesGUImap.AppendUniqueToArray(hash32, sres.GetFileName())
				}
				if madeTotalMatch {
					arr, _ := g.filenamesMap.GetMapping(hash32)
					//fmt.Printf("array: %v\n", arr.GetArrayCopy())
					g.searchMap.InformListeners(arr.GetArrayCopy(), hash32)
					madeAnyMatch = true
				} else {
					arr, _ := g.filenamesMap.GetMapping(hash32)
					madeAnyMatch = g.searchMap.CheckIfMatches(arr.GetArrayCopy(), hash32)
				}

				if madeAnyMatch {
					printSearchFound(sres.GetFileName(), pap.GetOrigin(), hash32, sres.GetChunkMap())
				}
			}

		} else {
			//fmt.Printf("FORWARDING PACKET\n")
			g.forward(pap.Packet)
		}
	}
}

func (g *Gossiper) searchFilesMatching(keywords []string, initialBudget uint64) (files map[[32]byte]*dto.SafeStringArray, ok bool) {
	files = g.filenamesMap.GetMatches(keywords)
	for metahash, arr := range files {
		cm, hasTotalMatch, _ := g.metahashToChunkOwnersMap.GetMapCopy(metahash)
		if !hasTotalMatch {
			delete(files, metahash)
		}
		for _, filename := range arr.GetArrayCopy() {
			keys := make([]uint64, 0, len(cm))
			for k := range cm {
				keys = append(keys, k)
			}
			chunks := strings.Trim(strings.Join(strings.Fields(fmt.Sprint(keys)), ","), "[]")
			if hasTotalMatch {
				fmt.Printf("From previous searches FOUND filename %v metahash=%x chunks=%v (TOTAL MATCH)\n", filename, metahash, chunks)
			} else {
				fmt.Printf("From previous searches FOUND filename %v metahash=%x chunks=%v (MISSING CHUNKS)\n", filename, metahash, chunks)
			}
		}
	}
	if len(files) < filesearching.ThresholdTotalMatches {
		cFiles, isNew := g.searchMap.AddListenerWithFiles(keywords, files)
		if !isNew {
			fmt.Printf("Network search for keywords %v already in progress. Please wait...\n", keywords)
			return nil, false
		}
		budget := initialBudget
		if initialBudget == 0 {
			budget = 2
		}
		sreq := g.makeSearchRequest(budget, keywords)
		g.forwardSearchRequest(sreq, "")
		t1 := time.NewTicker(budgetIncreaseTimeout * time.Second)
		t2 := time.NewTicker(searchTimeout * time.Second)
		for {
			select {
			case files = <-cFiles:
				fmt.Printf("SEARCH FINISHED\n")
				return files, true
			case <-t1.C:
				if initialBudget == 0 && budget >= 32 {
					t1.Stop()
				} else if initialBudget == 0 {
					budget *= 2
					sreq.Budget = budget
					g.forwardSearchRequest(sreq, "")
					fmt.Printf("RE FORWARDING SEARCH\n")
				}
			case <-t2.C:
				g.searchMap.RemoveListener(keywords)
				select {
				case <-cFiles:
				default:
				}
				close(cFiles)
				fmt.Printf("SEARCH FINISHED UNSUCCESSFULLY\n")
				return nil, false
			}
		}
	}
	fmt.Printf("Skipping network search as enough matches have been found locally...\n")
	fmt.Printf("SEARCH FINISHED\n")
	return
}

//clientFileShareListenRoutine - deals with new fileDownload messages from clients
func (g *Gossiper) clientFileSearchListenRoutine(cFileSearcUI chan *dto.FileToSearch) {
	for fileSearch := range cFileSearcUI {
		fmt.Printf("Search request. Budget: %v, Keywords: %v.\n", fileSearch.GetBudget(), fileSearch.GetKeywords())

		go g.searchFilesMatching(fileSearch.GetKeywords(), fileSearch.GetBudget())
	}
}
