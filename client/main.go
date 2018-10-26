package main

import (
	"flag"
)

func main() {
	UIPort := flag.String("UIPort", "8080", "Port for the UI client (default \"8080\")")
	dest := flag.String("dest", "", "destination for the private message")
	fileName := flag.String("file", "", "file to be indexed by the gossiper")
	msg := flag.String("msg", "", "message to be sent")

	flag.Parse()

	client := NewClient(*UIPort, *dest, *fileName, *msg)
	client.sendUDP()
}
