package main

import (
	"encoding/json"
	"flag"
	"net"
	"net/http"
	"protobuf"
	"strconv"
	"strings"

	"github.com/mvidigueira/Peerster/dto"
	"github.com/mvidigueira/Peerster/gossiper"
)

var g *gossiper.Gossiper
var uiport int

func main() {
	UIPort := flag.Int("UIPort", 8080, "Port for the UI client (default \"8080\")")
	gossipAddr := flag.String("gossipAddr", "127.0.0.1:5000", "ip:port for the gossiper (default \"127.0.0.1:5000\")")
	name := flag.String("name", "", "name of the gossiper")
	simple := flag.Bool("simple", false, "run gossiper in simple broadcast mode")
	peersStr := flag.String("peers", "", "comma separated list of peers of the form ip:port")

	flag.Parse()

	uiport = *UIPort
	peers := make([]string, 0)
	if *peersStr != "" {
		peers = strings.Split(*peersStr, ",")
	}
	g = gossiper.NewGossiper(*gossipAddr, *name, *UIPort, peers, *simple)

	go g.Start()

	http.Handle("/", http.FileServer(http.Dir("./frontend")))
	http.HandleFunc("/message", messageHandler)
	http.HandleFunc("/node", nodeHandler)
	http.HandleFunc("/id", idHandler)
	for {
		err := http.ListenAndServe("localhost:"+strconv.Itoa(*UIPort), nil)
		panic(err)
	}
}

func messageHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		testJSON, err := json.Marshal(g.GetLatestMessagesList())
		if err != nil {
			panic(err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(testJSON)
	case "POST":
		err := r.ParseForm()
		if err != nil {
			panic(err)
		}
		message := r.PostForm.Get("message")
		sendUDP(message)
	}
}

type jsonPeer struct {
	Name string
}

func nodeHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		peers := g.GetPeersList()

		jsonPeersList := make([]jsonPeer, len(peers))
		for i, v := range peers {
			jsonPeersList[i] = jsonPeer{Name: v}
		}
		testJSON, err := json.Marshal(jsonPeersList)
		if err != nil {
			panic(err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(testJSON)
	case "POST":
		err := r.ParseForm()
		if err != nil {
			panic(err)
		}
		node := r.PostFormValue("peer")
		g.AddPeer(node)
	}
}

func idHandler(w http.ResponseWriter, r *http.Request) {
	testJSON, err := json.Marshal(g.GetName())
	if err != nil {
		panic(err)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(testJSON)
}

func sendUDP(text string) {
	addr, _ := net.ResolveUDPAddr("udp4", "localhost:5500")
	addrGossiper, _ := net.ResolveUDPAddr("udp4", "localhost:"+strconv.Itoa(uiport))
	conn, _ := net.ListenUDP("udp4", addr)
	msg := &dto.SimpleMessage{Contents: text}
	packet := &dto.GossipPacket{Simple: msg}
	packetBytes, _ := protobuf.Encode(packet)

	conn.WriteToUDP(packetBytes, addrGossiper)
	conn.Close()
}
