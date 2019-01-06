package dht

import (
	"fmt"
	bolt "go.etcd.io/bbolt"
	"protobuf"
)

const (
	KeywordsBucket = "keywords"
	LinksBucket = "links"
	PageHashBucket = "pageHash"
)

type Storage struct {
	Db *bolt.DB
}

// Special store operation for keyword -> url mappings
// We need this extra function since we want to support PUT operations. It might not be the cleanest solution to wrap
// the keywords and URLs in struct but I could not think about any other way.
// The definiton of KeywordToURLMap will move to the webcrawler package but it is in a seperate branch at the moment. I will move this when
// I merge the two branches.
type KeywordToURLMap struct {
	Keyword string
	LinkData map[string]int //url:keywordOccurrences
}

// You cannot serialize plain lists with protobuf but you have to wrap it in a struct
type BatchMessage struct {
	List []*KeywordToURLMap
}

func NewStorage(gossiperName string) (s *Storage){
	db, err := bolt.Open(gossiperName+"_index.db", 0666, nil)
	buckets := []string{KeywordsBucket, LinksBucket, PageHashBucket}
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
		if dbData == nil{
			ok = false
			return nil
		}
		data = make([]byte, len(dbData))
		copy(data, dbData)
		return nil
	})
	return
}

func (s *Storage) AddLinksForKeyword(key string, data []byte) (ok bool) {
	ok = true
	newKeywordToUrlMap := &KeywordToURLMap{}
	err := protobuf.Decode(data, newKeywordToUrlMap)
	if err != nil {
		fmt.Println("Error, trying to save non KeyWordToURLMapping data type.")
		return false
	}
 	idArray := GenerateKeyHash(key)
 	id := idArray[:]

	s.Db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(KeywordsBucket))
		var keywordMap KeywordToURLMap
		data := b.Get(id)
		if data == nil {
			keywordMap = KeywordToURLMap{key, make(map[string]int)}
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
