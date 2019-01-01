package main

import (
	"flag"
)

func main() {
	UIPort := flag.String("UIPort", "8080", "Port for the UI client (default \"8080\")")
	dest := flag.String("dest", "", "destination for the private message")
	fileName := flag.String("file", "", "file to be indexed by the gossiper")
	msg := flag.String("msg", "", "message to be sent")
	request := flag.String("request", "", "request a chunk or metafile of this hash")
	keywords := flag.String("keywords", "", "keywords to be used for a search")
	budget := flag.Int64("budget", 0, "budget to be used for a search")
	node := flag.String("lookupNode", "", "request a dht node lookup")
	key := flag.String("lookupKey", "", "request a dht key lookup")
	store := flag.String("store", "", "key value pair of the form key:value to be stored in the dht. Value can either be a value or a url to a resource on the local file system.")

	flag.Parse()

	client := NewClient(*UIPort, *dest, *fileName, *msg, *request, *keywords, uint64(*budget), *node, *key, *store)
	client.sendUDP()
}
