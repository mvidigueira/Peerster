package main

import (
	"encoding/hex"
	"log"
	"net"
	"protobuf"
	"strings"

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
	keywords        string
	budget          uint64
	diffieHellman   bool
}

//NewClient - for the creation of single use clients
func NewClient(UIport, dest string, file string, msg string, request string, keywords string, budget uint64, diffieHellman bool) *Client {
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
		keywords:        keywords,
		budget:          budget,
		diffieHellman:   diffieHellman,
	}
}

func (c *Client) sendUDP() {
	var request *dto.ClientRequest
	if c.keywords != "" {

		file2search := &dto.FileToSearch{Budget: c.budget, Keywords: strings.Split(c.keywords, ",")}
		request = &dto.ClientRequest{FileSearch: file2search}
	} else if c.request != "" {
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
	} else if c.dest != "" && c.diffieHellman && c.msg != "" {
		encryptedMessage := &dto.EncryptedPrivateMessage{
			Destination: c.dest,
			CipherText:  []byte(c.msg),
			HopLimit:    10,
		}
		packet := &dto.GossipPacket{EncryptedMessage: encryptedMessage}
		request = &dto.ClientRequest{Packet: packet}
	} else if c.dest != "" && c.diffieHellman {
		diffieRequest := &dto.DiffieHellman{
			Destination: c.dest,
			HopLimit:    10,
		}
		packet := &dto.GossipPacket{DiffieHellman: diffieRequest}
		request = &dto.ClientRequest{Packet: packet}
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
