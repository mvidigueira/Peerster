package gossiper

import (
	"fmt"
	"github.com/axiomhq/hyperloglog"
	"github.com/bluele/gcache"
	"github.com/dedis/protobuf"
	"github.com/mvidigueira/Peerster/dht"
	"github.com/mvidigueira/Peerster/dht_util"
	"github.com/mvidigueira/Peerster/webcrawler"
	"go.etcd.io/bbolt"
	"log"
	"math"
	"sync"
)

/*  As described in
http://citeseerx.ist.psu.edu/viewdoc/download?doi=10.1.1.1.4205&rep=rep1&type=pdf
*/

const (
	cacheSize     = 5000000
	prEpsilon     = 0.03
	dampingFactor = 0.85
	InitialRank   = 1 - dampingFactor
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

type ranker struct {
	cache         gcache.Cache
	hits          int
	totalAccesses int
	hll *hyperloglog.Sketch
	hllMux sync.Mutex
	mux           sync.Mutex
}

func (pRanker *ranker) cacheSet(key interface{}, value interface{}) {
	err := pRanker.cache.Set(key, value)
	if err != nil {
		panic(err)
	}
}

func (g *Gossiper) PageRankCacheHitRate() float64 {
	cache := g.pRanker
	return float64(cache.hits) / float64(cache.totalAccesses)
}

func newRanker() (pRanker *ranker) {
	gc := gcache.New(cacheSize).ARC().Build()
	pRanker = &ranker{cache: gc, hll: hyperloglog.New14()}
	return
}

type UpdateToProcess struct {
	Update  *RankUpdate
	IsLocal bool
	IsNew   bool
}

//RankUpdate is generated when a node has updated a pagerank of a page
//and needs to propagate it to the rest of the network
type RankUpdate struct {
	OutboundLink string    //The link that the sender thinks should be affected
	RankInfo     *RankInfo //New info to consider
}

type RankInfo struct {
	Url                   string
	Rank                  float64
	NumberOfOutboundLinks int
}

//Recalculate the Rank of a page taking into account the new information
func (g *Gossiper) receiveRankUpdate(update *RankUpdate, isLocal bool, isNew bool) {
	g.pRanker.hllMux.Lock()
	g.pRanker.hll.Insert([]byte(update.RankInfo.Url))
	g.pRanker.hllMux.Unlock()
	g.processRankUpdates(&UpdateToProcess{Update: update, IsLocal: isLocal, IsNew: isNew})
}

func (g *Gossiper) GetDocumentEstimate() int {
	numberOfDocuments := 0
	g.dhtDb.Db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(dht.PageHashBucket))
		stats := b.Stats()
		numberOfDocuments = stats.KeyN
		log.Printf("page hashes: %d\n", numberOfDocuments)
		b = tx.Bucket([]byte(dht.PageRankBucket))
		stats = b.Stats()
		log.Printf("page ranks: %d\n", stats.KeyN)
		if stats.KeyN > numberOfDocuments {
			numberOfDocuments = stats.KeyN
		}
		return nil
	})

	hllEstimate := int(g.pRanker.hll.Estimate())
	if hllEstimate > numberOfDocuments {
		numberOfDocuments = hllEstimate
	}
	log.Printf("hll estimate: %d\n",hllEstimate)

	return numberOfDocuments
}

func (g *Gossiper) processRankUpdates(toProcess *UpdateToProcess) {
	update := toProcess.Update
	urlId := dht_util.GenerateKeyHash(update.RankInfo.Url)

	if toProcess.IsNew {
		update.RankInfo.updatePageRank(g)
	}

	if toProcess.IsLocal {
		//If we are the one responsible for storing this PageRank
		data, err := protobuf.Encode(update.RankInfo)
		if err != nil {
			panic(err)
		}
		g.dhtDb.Db.Batch(func(tx *bbolt.Tx) error {
			b := tx.Bucket([]byte(dht.PageRankBucket))
			err := b.Put(urlId[:], data)
			if err != nil {
				panic(err)
			}
			return nil
		})
	}

	//Update cache:
	g.pRanker.cacheSet(urlId, update.RankInfo)

	//We have received new information about page A,
	//Now we have to update the ranks of all the pages that
	//page A pointed to
	id := dht_util.GenerateKeyHash(update.OutboundLink)
	outboundLinksPackage := g.getOutboundLinksFromDb(id)
	if outboundLinksPackage == nil {
		return
	}

	for _, link := range outboundLinksPackage.OutBoundLinks {
		linkId := dht_util.GenerateKeyHash(link)
		linkInfo := g.getRankFromDatabase(linkId)
		if linkInfo != nil {
			relerr := linkInfo.updatePageRankWithInfo(update, g)
			//Check that the pagerank has not converged yet
			if relerr > prEpsilon {
				linkOutbounds := g.getOutboundLinksFromDb(id)
				//Store and send updates
				g.storeRankLocally(linkInfo)
				update := &RankUpdate{"", linkInfo}
				g.batchRankUpdates(linkOutbounds, update)
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

func (g *Gossiper) storeRankLocally(pageInfo *RankInfo) {
	id := dht_util.GenerateKeyHash(pageInfo.Url)

	data, err := protobuf.Encode(pageInfo)
	if err != nil {
		panic(err)
	}

	g.pRanker.cacheSet(id, data)

	g.dhtDb.Db.Batch(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(dht.PageRankBucket))
		err = b.Put(id[:], data)
		if err != nil {
			panic(err)
		}
		return nil
	})

}

func (g *Gossiper) batchRankUpdates(urlInfo *webcrawler.OutBoundLinksPackage, pageUpdate *RankUpdate) {
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
		pageUpdate.OutboundLink = outboundLink
		destinations[*closest] = append(val, pageUpdate)
	}

	for dest, batch := range destinations {
		if dest.Address == g.address {
			// This node is the destination
			for _, update := range batch {
				g.receiveRankUpdate(update, true, false)
			}
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
			//TODO: this should probably be changed when Jakob changes the size of the key?
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

func (page *RankInfo) updatePageRankWithInfo(update *RankUpdate, g *Gossiper) (relerr float64) {
	var newInfo *RankInfo = nil
	if update != nil {
		newInfo = update.RankInfo
	}

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
		return 0 //TODO: this should probably be a different value?
	}

	//PR(A) = (1-d) + d*(PR(T1)/deg(T1) + ... + PR(Tn)/deg(Tn))
	//Where deg(T) is defined as the number of links going out of page T

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
	newRank := (1 - dampingFactor) + dampingFactor*inlinkSum

	page.Rank = newRank
	relerr = math.Abs(oldRank-newRank) / oldRank
	return
}

//Get the Rank of a page
//first looks in the cache, then in the db, then on the network
func (g *Gossiper) getRank(id dht_util.TypeID) (rank *RankInfo) {
	g.pRanker.mux.Lock()
	defer g.pRanker.mux.Unlock()
	rank, exists := g.getRankLocally(id)

	if !exists {
		data, found := g.LookupValue(id, dht.PageRankBucket) //TODO: batching
		if found {
			rank := &RankInfo{}
			err := protobuf.Decode(data, rank)
			if err != nil {
				panic(err)
			}
			g.pRanker.cacheSet(id, rank)
			if err != nil {
				panic(err)
			}
			return rank
		} else {
			g.pRanker.cacheSet(id, nil)
			return nil
		}
	}

	return
}

func (g *Gossiper) getRankLocally(id dht_util.TypeID) (rank *RankInfo, exists bool) {
	g.pRanker.totalAccesses++
	rankData, err := g.pRanker.cache.Get(id)
	if err == nil {
		if rankData == nil {
			return nil, true
		}
		rank = rankData.(*RankInfo)
		g.pRanker.hits++
		exists = true
		return
	}

	if err != gcache.KeyNotFoundError {
		panic(err)
	}

	rank = g.getRankFromDatabase(id)
	if rank != nil {
		exists = true
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
		} else {
			rank = nil
		}
		return nil
	})
	return
}
