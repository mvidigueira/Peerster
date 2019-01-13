package dht

import (
	"github.com/dedis/protobuf"
	"log"
	"sort"

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

type urlData struct {
	url string
	keywordFrequency int
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

	maxSize := 7500
	if bucket == KeywordsBucket && len(data) > maxSize {
		keywordMap := &webcrawler.KeywordToURLMap{}
		err := protobuf.Decode(data, keywordMap)
		if err != nil {
			panic(err)
		}
		newMap := &webcrawler.KeywordToURLMap{Keyword:keywordMap.Keyword, LinkData: make(map[string]int)}
		i := 0
		defer func(){
			log.Printf("packeted %d\n", i)
		}()
		data = []byte{}
		keywordData := []*urlData{}
		for url, f := range keywordMap.LinkData {
			keywordData = append(keywordData, &urlData{url, f})
		}
		sort.Slice(keywordData, func(i, j int) bool {
			return keywordData[i].keywordFrequency > keywordData[j].keywordFrequency
		})

		for _, d := range keywordData {
			if len(data)+len(d.url)+100 > maxSize {
				return
			}
			i++
			newMap.LinkData[d.url] = d.keywordFrequency
			data, err = protobuf.Encode(newMap)
			if err != nil {
				panic(err)
			}
		}
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
		log.Print(err)
		ok = false
	}
	return
}
