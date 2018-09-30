package main

import (
	"log"
	"net"
	"protobuf"

	"github.com/mvidigueira/Peerster/dto"
)

const clientPort = "5500"

func logError(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

type Client struct {
	addr            *net.UDPAddr
	conn            *net.UDPConn
	udpAddrGossiper *net.UDPAddr
	msg             string
}

func NewClient(UIport, msg string) *Client {
	addr, err := net.ResolveUDPAddr("udp4", "localhost:"+clientPort)
	addrGossiper, err := net.ResolveUDPAddr("udp4", "localhost:"+UIport)
	logError(err)
	conn, err := net.ListenUDP("udp4", addr)
	logError(err)

	return &Client{
		addr:            addr,
		conn:            conn,
		udpAddrGossiper: addrGossiper,
		msg:             msg,
	}
}

func (c *Client) sendUDP() {
	msg := &dto.SimpleMessage{Contents: c.msg}
	packet := &dto.GossipPacket{Simple: msg}
	packetBytes, err := protobuf.Encode(packet)
	dto.LogError(err)
	c.conn.WriteToUDP(packetBytes, c.udpAddrGossiper)
}
