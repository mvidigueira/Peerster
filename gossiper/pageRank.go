package gossiper

import (
	"fmt"
	"github.com/bluele/gcache"
	"github.com/dedis/protobuf"
	"github.com/mvidigueira/Peerster/dht"
	"github.com/mvidigueira/Peerster/dht_util"
	"github.com/mvidigueira/Peerster/webcrawler"
	"go.etcd.io/bbolt"
	"math"
)

type rankerCache struct {
	cache gcache.Cache
	hits int
	totalAccesses int
}

func (g *Gossiper) PageRankCacheHitRate() float64 {
	cache := g.rankerCache
	return float64(cache.hits)/float64(cache.totalAccesses)
}

/*  As described in
http://citeseerx.ist.psu.edu/viewdoc/download?doi=10.1.1.1.4205&rep=rep1&type=pdf
*/

const (
	cacheSize     = 1000 //TODO: these values are for testing
	epsilon       = 0.03
	InitialRank   = 1
	dampingFactor = 0.85
)

/*
	Damping factor:
	The larger the damping factor,
	the more emphasis the final vector places on citations rather than random
	factors. A higher damping factor should result in a higher level of accuracy
	in the PageRank value, but at the cost of requiring more iterations of the
	algorithm to reach convergence.

	Epsilon:
	There is a threshold at which reducing the
	size of the error term to produce a more precise value for the PageRank
	vector results in little to no change in the relative rankings of individual
	pages, or the composition of particular percentiles

	Source: https://pdfs.semanticscholar.org/6274/90c3b5d09e3e4546490bf371f7db7c21d553.pdf
*/

func newRankerCache() (ranker *rankerCache) {
	gc := gcache.New(cacheSize).ARC().Build()
	ranker = &rankerCache{cache: gc}
	return
}

type RankUpdate struct {
	OutboundLink string
	RankInfo     *RankInfo
}

type RankInfo struct {
	Url                   string
	Rank                  float64
	NumberOfOutboundLinks int
}

//Recalculate the Rank of a page taking into account the new information
func (g *Gossiper) receiveRankUpdate(update *RankUpdate) {
	//We have received new information about page A,
	//Now we have to update the ranks of all the pages that
	//page A pointed to
	id := dht_util.GenerateKeyHash(update.OutboundLink)

	//Update cache:
	urlId := dht_util.GenerateKeyHash(update.RankInfo.Url)
	g.rankerCache.cache.Set(urlId, update.RankInfo)


	outboundLinksPackage := g.getOutboundLinksFromDb(id)
	if outboundLinksPackage == nil {
		return
	}

	for _, link := range outboundLinksPackage.OutBoundLinks {
		id := dht_util.GenerateKeyHash(link)
		linkInfo := g.getRankFromDatabase(id)
		if linkInfo != nil {
			relerr := linkInfo.updatePageRankWithInfo(update.RankInfo, g)
			//Check that the page rank has not converged yet
			if relerr > epsilon {
				linkOutbounds := g.getOutboundLinksFromDb(id)
				g.setRank(*linkOutbounds, *linkInfo, relerr)
			}
		}
	}
}

func (g *Gossiper) getOutboundLinksFromDb(id dht_util.TypeID) (outboundLinksPackage *webcrawler.OutBoundLinksPackage) {
	outboundLinksPackage = &webcrawler.OutBoundLinksPackage{}
	g.dhtDb.Db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(dht.LinksBucket))
		data := b.Get(id[:])
		if data != nil {
			err := protobuf.Decode(data, outboundLinksPackage)
			if err != nil {
				panic(err)
			}
		}
		return nil
	})
	return
}

//Called when the node has recalculated its local Rank and wants to store it
//sending updates to the rest of the network as well
func (g *Gossiper) setRank(urlInfo webcrawler.OutBoundLinksPackage, pageInfo RankInfo, relerr float64) {
	if relerr < epsilon {
		return
	}

	id := dht_util.GenerateKeyHash(pageInfo.Url)

	//Update cache:
	g.rankerCache.cache.Set(id, pageInfo)

	data, err := protobuf.Encode(&pageInfo)
	if err != nil {
		panic(err)
	}
	g.dhtDb.Db.Batch(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(dht.PageRankBucket))
		err = b.Put(id[:], data)
		if err != nil {
			panic(err)
		}
		return nil
	})

	g.batchRankUpdates(&urlInfo, &pageInfo)
}

func (g *Gossiper) batchRankUpdates(urlInfo *webcrawler.OutBoundLinksPackage, pageInfo *RankInfo) {
	destinations := make(map[dht.NodeState][]*RankUpdate)
	for _, outboundLink := range urlInfo.OutBoundLinks {
		id := dht_util.GenerateKeyHash(outboundLink)
		kClosest := g.LookupNodes(id)
		if len(kClosest) == 0 {
			fmt.Printf("Could not perform store since no neighbours found.\n")
			break
		}
		closest := kClosest[0]
		val, _ := destinations[*closest]
		destinations[*closest] = append(val, &RankUpdate{OutboundLink: outboundLink, RankInfo: pageInfo})
	}

	for dest, batch := range destinations {
		if dest.Address == g.address {
			// This node is the destination
			packetBytes, err := protobuf.Encode(&BatchMessage{PageRankUpdates: batch})
			if err != nil {
				fmt.Println(err)
				return
			}
			msg := g.newDHTStore(dest.NodeID, packetBytes, dht.CitationsBucket)
			g.replyStore(msg)
			continue
		}

		s := make([]interface{}, len(batch))
		for i, v := range batch {
			s[i] = v
		}
		batches := g.createUDPBatches(&dest, s)
		for _, batch := range batches {
			tmp := make([]*RankUpdate, len(batch.([]interface{})))
			for k, b := range batch.([]interface{}) {
				tmp[k] = b.(*RankUpdate)
			}
			packetBytes, err := protobuf.Encode(&BatchMessage{PageRankUpdates: tmp})
			if err != nil {
				panic(err)
			}
			err = g.sendStore(&dest, [20]byte{}, packetBytes, dht.PageRankBucket)
			if err != nil {
				fmt.Printf("Failed to store key.\n")
			}
		}
	}
}

func (page *RankInfo) updatePageRank(g *Gossiper) float64 {
	return page.updatePageRankWithInfo(nil, g)
}

func (page *RankInfo) updatePageRankWithInfo(newInfo *RankInfo, g *Gossiper) (relerr float64) {
	citations := &webcrawler.Citations{}
	url := page.Url
	id := dht_util.GenerateKeyHash(url)
	oldRank := page.Rank

	g.dhtDb.Db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(dht.CitationsBucket))
		citationsData := b.Get(id[:])
		if citationsData != nil {
			err := protobuf.Decode(citationsData, citations)
			if err != nil {
				panic(err)
			}
		}
		return nil
	})

	if citations.CitedBy == nil {
		return 0
	}

	//PR(A) = (1-d) + d (PR(T1)/C(T1) + ... + PR(Tn)/C(Tn))
	//Where C(T) is defined as the number of links going out of page T

	inlinkSum := 0.0
	for _, citation := range citations.CitedBy {
		rankInfo := newInfo
		if newInfo == nil || citation != newInfo.Url {
			rankInfo = g.getRank(dht_util.GenerateKeyHash(citation))
		}
		if rankInfo != nil {
			inlinkSum += rankInfo.Rank / float64(rankInfo.NumberOfOutboundLinks+1)
		}
	}

	inlinkSum *= dampingFactor
	newRank := (1 - dampingFactor) + inlinkSum
	page.Rank = newRank

	relerr = math.Abs(oldRank-newRank) / newRank
	return
}

//get the Rank of a page
//first looks in the cache, then in the db, then on the network
func (g *Gossiper) getRank(id dht_util.TypeID) (rank *RankInfo) {
	g.rankerCache.totalAccesses++
	rankData, err := g.rankerCache.cache.Get(id)
	if err == nil {
		rank = rankData.(*RankInfo)
		g.rankerCache.hits++
		return
	}

	rank = g.getRankFromDatabase(id)

	if rank == nil {
		data, found := g.LookupValue(id, dht.PageRankBucket) //TODO: batching
		if found {
			protobuf.Decode(data, rank)
			g.rankerCache.cache.Set(id, rank)
		}
	}

	return
}

func (g *Gossiper) getRankFromDatabase(id dht_util.TypeID) (rank *RankInfo) {
	rank = &RankInfo{}
	g.dhtDb.Db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(dht.PageRankBucket))
		rankData := b.Get(id[:])
		if rankData != nil {
			err := protobuf.Decode(rankData, rank)
			if err != nil {
				return err
			}
		}
		return nil
	})
	return
}
