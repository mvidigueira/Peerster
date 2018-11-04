package main

import (
	"encoding/hex"
	"log"
	"net"
	"protobuf"

	"github.com/mvidigueira/Peerster/dto"
	"github.com/mvidigueira/Peerster/fileparsing"
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
	file            string
	request         string
}

//NewClient - for the creation of single use clients
func NewClient(UIport, dest string, file string, msg string, request string) *Client {
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
		file:            file,
		msg:             msg,
		request:         request,
	}
}

func (c *Client) sendUDP() {
	var request *dto.ClientRequest
	if c.request != "" {
		hash, err := hex.DecodeString(c.request)
		if err != nil {
			return
		}
		hash32, ok := fileparsing.ConvertToHash32(hash)
		if !ok {
			return
		}
		file2d := &dto.FileToDownload{FileName: c.file, Origin: c.dest, Metahash: hash32}
		request = &dto.ClientRequest{FileDownload: file2d}
	} else if c.file != "" {
		file2s := &dto.FileToShare{FileName: c.file}
		request = &dto.ClientRequest{FileShare: file2s}
	} else if c.dest != "" {
		msg := &dto.PrivateMessage{Text: c.msg, Destination: c.dest}
		packet := &dto.GossipPacket{Private: msg}
		request = &dto.ClientRequest{Packet: packet}
	} else {
		msg := &dto.SimpleMessage{Contents: c.msg}
		packet := &dto.GossipPacket{Simple: msg}
		request = &dto.ClientRequest{Packet: packet}
	}
	packetBytes, err := protobuf.Encode(request)
	dto.LogError(err)
	c.conn.WriteToUDP(packetBytes, c.udpAddrGossiper)
}

func (c *Client) close() {
	c.conn.Close()
}
