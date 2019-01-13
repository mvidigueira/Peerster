package dht

import (
	"bytes"
	"crypto/sha512"
	"encoding/binary"
	"encoding/gob"
	"errors"
	"log"

	"github.com/dedis/protobuf"

	. "github.com/mvidigueira/Peerster/dht_util"
	"github.com/mvidigueira/Peerster/webcrawler"
	bolt "go.etcd.io/bbolt"
)

const queueCountID = "queCount"
const pastMapID = "pastMap"

const (
	KeywordsBucket   = "keywords"
	LinksBucket      = "links"
	CitationsBucket  = "citations"
	PageHashBucket   = "pageHash"
	PageRankBucket   = "pageRank"
	CrawlQueueBucket = "crawlQueue"
	QueueIndexBucket = "queueIndex"
	PastMapBucket    = "pastMap"
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
	buckets := []string{KeywordsBucket, LinksBucket, PageHashBucket, CrawlQueueBucket, QueueIndexBucket, CitationsBucket, PageRankBucket, PastMapBucket}
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

// All methods below is used by the web crawler to save it's state in order to be able to continue crawling after shutdown/crash.

func (s Storage) UpdateCrawlQueue(data []byte) (err error) {
	err = s.Db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("crawlQueue"))
		id, _ := b.NextSequence()
		newURLHash := sha512.Sum512(Itob(int(id) - 1))
		err = b.Put(newURLHash[:], data)
		return err
	})
	return err
}

// Returns the crawl queue pointer
func (s Storage) CrawlQueueHeadPointer() (uint64, bool) {
	queueHash := GenerateKeyHash(queueCountID)
	val, found := s.Retrieve(queueHash, "queueIndex")
	if !found {
		log.Fatal("Error, sequence number not found.")
	}
	queue := binary.BigEndian.Uint64(val)
	return queue, found
}

// Returns the url pointed to by the crawl queue pointer.
func (s Storage) CrawlQueueHead() (string, error) {

	// Get sequence number of queue head
	queueHash := GenerateKeyHash(queueCountID)
	val, found := s.Retrieve(queueHash, "queueIndex")
	if !found {
		log.Fatal("Error, queue head sequence number not found")
	}
	headSequenceNumber := binary.BigEndian.Uint64(val)

	// Get head of queue
	newURLHash := sha512.Sum512(Itob(int(headSequenceNumber)))
	head, found := s.Retrieve(newURLHash, "crawlQueue")
	if !found {
		return "", errors.New("crawl queue head not found")
	}

	// Update pointer to head of queue
	err := s.Put(queueHash, Itob(int(headSequenceNumber)+1), "queueIndex")
	if err != nil {
		log.Fatal("Error, updating queue head sequence number")
	}

	return string(head), nil
}

func (s Storage) SavePast(past []byte) {
	key := GenerateKeyHash(pastMapID)
	s.Put(key, past, pastMapID)
}

func (s Storage) GetPast() map[string]bool {
	// Restore bloom filter
	hash := GenerateKeyHash(pastMapID)
	pastBytes, found := s.Retrieve(hash, pastMapID)
	if !found {
		log.Println("Could not find past map.")
	}
	var decodedMap map[string]bool
	d := gob.NewDecoder(bytes.NewReader(pastBytes))
	// Decoding the serialized data
	err := d.Decode(&decodedMap)
	if err != nil {
		panic(err)
	}
	return decodedMap
}

// Deletes the url pointed to by the crawl queue index pointer from the dht.
func (s Storage) DeleteCrawlQueueHead() error {
	index, found := s.CrawlQueueHeadPointer()
	if !found {
		log.Println("queue index not found.")
	}
	newURLHash := sha512.Sum512(Itob(int(index) - 1))
	return s.Delete(newURLHash, "crawlQueue")
}
