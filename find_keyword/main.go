package main

import (
	"flag"
	"github.com/mvidigueira/Peerster/dht"
	"github.com/mvidigueira/Peerster/dht_util"
	"github.com/reiver/go-porterstemmer"
	bolt "go.etcd.io/bbolt"
	"log"
)

func FindKeyword(name string, keyword string) {
	db, err := bolt.Open(name, 0666, nil)
	if err != nil {
		panic(err)
	}
	keyword = porterstemmer.StemString(keyword)
	id := dht_util.GenerateKeyHash(keyword)
	db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(dht.KeywordsBucket))
		resultData := b.Get(id[:])
		log.Println(len(resultData))
		return nil
	})
}

func main() {
	keyword := flag.String("keyword", "", "keyword to find")
	db := flag.String("name", "", "name of DB")
	flag.Parse()
	FindKeyword(*db, *keyword)
}
