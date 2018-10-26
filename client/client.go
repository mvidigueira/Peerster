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

//Client - used to send private a simple messages to the respective gossiper
type Client struct {
	addr            *net.UDPAddr
	conn            *net.UDPConn
	udpAddrGossiper *net.UDPAddr
	msg             string
	dest            string
}

//NewClient - for the creation of single use clients
func NewClient(UIport, dest string, msg string) *Client {
	addr, err := net.ResolveUDPAddr("udp4", "localhost:"+clientPort)
	addrGossiper, err := net.ResolveUDPAddr("udp4", "localhost:"+UIport)
	logError(err)
	conn, err := net.ListenUDP("udp4", addr)
	logError(err)

	return &Client{
		addr:            addr,
		conn:            conn,
		udpAddrGossiper: addrGossiper,
		dest:            dest,
		msg:             msg,
	}
}

func (c *Client) sendUDP() {
	var packet *dto.GossipPacket
	if c.dest != "" {
		msg := &dto.PrivateMessage{Text: c.msg, Destination: c.dest}
		packet = &dto.GossipPacket{Private: msg}
	} else {
		msg := &dto.SimpleMessage{Contents: c.msg}
		packet = &dto.GossipPacket{Simple: msg}
	}
	packetBytes, err := protobuf.Encode(packet)
	dto.LogError(err)
	c.conn.WriteToUDP(packetBytes, c.udpAddrGossiper)
}

func (c *Client) close() {
	c.conn.Close()
}
