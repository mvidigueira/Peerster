package dto

import "log"

func LogError(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

type SimpleMessage struct {
	OriginalName  string
	RelayPeerAddr string
	Contents      string
}
type GossipPacket struct {
	Simple *SimpleMessage
}

type Message struct {
	Text string
}
