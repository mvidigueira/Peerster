package main

import (
	"flag"
)

func main() {
	UIPort := flag.String("UIPort", "8080", "Port for the UI client (default \"8080\")")
	msg := flag.String("msg", "", "message to be sent")

	flag.Parse()

	client := NewClient(*UIPort, *msg)
	client.sendUDP()
}
