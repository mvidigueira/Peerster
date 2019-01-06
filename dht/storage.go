package dht

import (
	"crypto/sha1"
	"encoding/binary"
	"log"

	"github.com/dedis/protobuf"

	"github.com/mvidigueira/Peerster/bloomfilter"
	. "github.com/mvidigueira/Peerster/dht_util"
	"github.com/mvidigueira/Peerster/webcrawler"
	bolt "go.etcd.io/bbolt"
)

const queueCountID = "queCount"
const bloomFilterStateID = "bloomFilter"

const (
	KeywordsBucket    = "keywords"
	LinksBucket       = "links"
	CitationsBucket   = "citations"
	PageHashBucket    = "pageHash"
	PageRankBucket    = "pageRank"
	CrawlQueueBucket  = "crawlQueue"
	QueueIndexBucket  = "queueIndex"
	BloomFilterBucket = "bloomFilter"
)

type Storage struct {
	Db *bolt.DB
}

// Special store operation for keyword -> url mappings
// We need this extra function since we want to support PUT operations. It might not be the cleanest solution to wrap
// the keywords and URLs in struct but I could not think about any other way.
// The definiton of KeywordToURLMap will move to the webcrawler package but it is in a seperate branch at the moment. I will move this when
// I merge the two branches.

func NewStorage(gossiperName string) (s *Storage) {
	db, err := bolt.Open(gossiperName+"_index.db", 0666, nil)
	buckets := []string{KeywordsBucket, LinksBucket, PageHashBucket, CrawlQueueBucket, QueueIndexBucket, CitationsBucket, PageRankBucket, BloomFilterBucket}
	if err != nil {
		panic(err)
	}
	db.Update(func(tx *bolt.Tx) error {
		for _, bucket := range buckets {
			_, err := tx.CreateBucketIfNotExists([]byte(bucket))
			if err != nil {
				panic(err)
			}
		}
		return nil
	})

	s = &Storage{db}

	// Initiates the crawl queue head pointer if not already present in the database
	keyHash := GenerateKeyHash(queueCountID)
	_, found := s.Retrieve(keyHash, "queueIndex")
	if !found {
		err := db.Update(func(tx *bolt.Tx) error {
			b := tx.Bucket([]byte("queueIndex"))
			return b.Put(keyHash[:], Itob(0))
		})
		if err != nil {
			log.Fatal("Failed to initiate queue")
		}
	}
	return
}

func (s *Storage) Retrieve(key TypeID, bucket string) (data []byte, ok bool) {
	ok = true
	s.Db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucket))
		dbData := b.Get(key[:])
		if dbData == nil {
			ok = false
			return nil
		}
		data = make([]byte, len(dbData))
		copy(data, dbData)
		return nil
	})
	return
}

func (s *Storage) AddLinksForKeyword(key string, newKeywordToUrlMap webcrawler.KeywordToURLMap) (ok bool) {
	ok = true
	idArray := GenerateKeyHash(key)
	id := idArray[:]
	var err error

	s.Db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(KeywordsBucket))
		var keywordMap webcrawler.KeywordToURLMap
		data := b.Get(id)
		if data == nil {
			keywordMap = webcrawler.KeywordToURLMap{key, make(map[string]int)}
		} else {
			err = protobuf.Decode(data, &keywordMap)
			if err != nil {
				panic(err)
			}
		}
		for k, val := range newKeywordToUrlMap.LinkData {
			keywordMap.LinkData[k] = val
		}
		data, err := protobuf.Encode(&keywordMap)
		if err != nil {
			panic(err)
		}
		err = b.Put(id, data)
		return err
	})

	if err != nil {
		ok = false
	}
	return
}

func (s *Storage) BulkAddLinksForKeyword(urlMaps []*webcrawler.KeywordToURLMap) (ok bool) {

	ok = true

	var err error

	s.Db.Batch(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(KeywordsBucket))

		for _, newKeywordToUrlMap := range urlMaps {
			key := newKeywordToUrlMap.Keyword
			idArray := GenerateKeyHash(key)
			id := idArray[:]

			var keywordMap webcrawler.KeywordToURLMap
			data := b.Get(id)
			if data == nil {
				keywordMap = *newKeywordToUrlMap
			} else {
				err = protobuf.Decode(data, &keywordMap)
				if err != nil {
					panic(err)
				}
				for k, val := range newKeywordToUrlMap.LinkData {
					keywordMap.LinkData[k] = val
				}
			}

			data, err := protobuf.Encode(&keywordMap)
			if err != nil {
				panic(err)
			}
			err = b.Put(id, data)
		}

		return err
	})

	if err != nil {
		ok = false
	}
	return
}

func (s Storage) Put(key TypeID, data []byte, bucket string) (err error) {
	err = s.Db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucket))
		err = b.Put(key[:], data)
		return err
	})
	return err
}

func (s Storage) Delete(key TypeID, bucket string) (err error) {
	err = s.Db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucket))
		err = b.Delete(key[:])
		return err
	})
	return err
}

func (s Storage) UpdateQueue(data []byte) (err error) {
	err = s.Db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("crawlQueue"))
		id, _ := b.NextSequence()
		newURLHash := sha1.Sum(Itob(int(id) - 1))
		err = b.Put(newURLHash[:], data)
		return err
	})
	return err
}

func (s Storage) GetQueueIndex() (uint64, bool) {
	queueHash := GenerateKeyHash(queueCountID)
	val, found := s.Retrieve(queueHash, "queueIndex")
	if !found {
		log.Fatal("Error, sequence number not found.")
	}
	queue := binary.BigEndian.Uint64(val)
	return queue, found
}

func (s Storage) CrawlQueueHead() string {

	// Get sequence number of queue head
	queueHash := GenerateKeyHash(queueCountID)
	val, found := s.Retrieve(queueHash, "queueIndex")
	if !found {
		log.Fatal("Error, queue head sequence number not found")
	}
	headSequenceNumber := binary.BigEndian.Uint64(val)

	// Get head of queue
	newURLHash := sha1.Sum(Itob(int(headSequenceNumber)))
	head, found := s.Retrieve(newURLHash, "crawlQueue")
	if !found {
		log.Fatal("Error, Could not find of queue.")
	}

	// Update pointer to head of queue
	err := s.Put(queueHash, Itob(int(headSequenceNumber)+1), "queueIndex")
	if err != nil {
		log.Fatal("Error, updating queue head sequence number")
	}

	return string(head)
}

func (s Storage) SaveBloomFilter(bloomFilter *bloomfilter.BloomFilter) {
	bytes, err := protobuf.Encode(bloomFilter)
	if err != nil {
		log.Fatal("Error, decoding bloom filter.")
	}
	key := GenerateKeyHash(bloomFilterStateID)
	s.Put(key, bytes, bloomFilterStateID)
}

func (s Storage) GetBloomFilter() *bloomfilter.BloomFilter {
	// Restore bloom filter
	bloomFilterHash := GenerateKeyHash(bloomFilterStateID)
	bloomFilterBytes, found := s.Retrieve(bloomFilterHash, bloomFilterStateID)
	if !found {
		log.Println("Could not find bloom filter.")
	}
	bloomFilter := &bloomfilter.BloomFilter{}
	err := protobuf.Decode(bloomFilterBytes, bloomFilter)
	if err != nil {
		log.Fatal("error decoign bloom filter")
	}
	return bloomFilter
}

func (s Storage) DeleteCrawlHead() error {
	index, found := s.GetQueueIndex()
	if !found {
		log.Println("queue index not found.")
	}
	newURLHash := sha1.Sum(Itob(int(index) - 1))
	return s.Delete(newURLHash, "crawlQueue")
}

func Itob(v int) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, uint64(v))
	return b
}
