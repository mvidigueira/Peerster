package dht

import (
	"protobuf"

	. "github.com/mvidigueira/Peerster/dht_util"
	"github.com/mvidigueira/Peerster/webcrawler"
	bolt "go.etcd.io/bbolt"
)

const (
	KeywordsBucket  = "keywords"
	LinksBucket     = "links"
	CitationsBucket = "citations"
	PageHashBucket  = "pageHash"
	PageRankBucket  = "pageRank"
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
	buckets := []string{KeywordsBucket, LinksBucket, PageHashBucket, CitationsBucket, PageRankBucket}
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
