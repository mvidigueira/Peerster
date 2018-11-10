package main

import (
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"net/http"
	"protobuf"
	"strings"

	"github.com/mvidigueira/Peerster/dto"
	"github.com/mvidigueira/Peerster/fileparsing"
	"github.com/mvidigueira/Peerster/gossiper"
)

var g *gossiper.Gossiper
var uiport string

func main() {
	UIPort := flag.String("UIPort", "8080", "Port for the UI client (default \"8080\")")
	gossipAddr := flag.String("gossipAddr", "127.0.0.1:5000", "ip:port for the gossiper (default \"127.0.0.1:5000\")")
	name := flag.String("name", "", "name of the gossiper")
	peersStr := flag.String("peers", "", "comma separated list of peers of the form ip:port")
	rtimeout := flag.Int("rtimer", 0, "route rumors sending period in seconds, 0 to disable sending of rout rumors (default 0)")
	simple := flag.Bool("simple", false, "run gossiper in simple broadcast mode")

	flag.Parse()

	uiport = *UIPort
	peers := make([]string, 0)
	if *peersStr != "" {
		peers = strings.Split(*peersStr, ",")
	}
	g = gossiper.NewGossiper(*gossipAddr, *name, *UIPort, peers, *simple, *rtimeout)

	go g.Start()

	//frontend

	http.Handle("/", http.FileServer(http.Dir("./frontend")))
	http.HandleFunc("/message", messageHandler)
	http.HandleFunc("/node", nodeHandler)
	http.HandleFunc("/id", idHandler)

	http.HandleFunc("/privatemessage", privateMessageHandler)
	http.HandleFunc("/origins", originsHandler)

	http.HandleFunc("/sharefile", shareFileHandler)
	http.HandleFunc("/dlfile", downloadFileHandler)
	for {
		err := http.ListenAndServe("localhost:"+*UIPort, nil)
		panic(err)
	}
}

func messageHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		list := gossiper.ConvertToFEMList(g.GetLatestMessagesList())
		testJSON, err := json.Marshal(list)
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
		fmt.Printf("Message: %s\n", message)
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

type jsonOrigin struct {
	Name string
}

func privateMessageHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "POST":
		err := r.ParseForm()
		if err != nil {
			panic(err)
		}
		message := r.PostForm.Get("message")
		dest := r.PostForm.Get("destName")

		if dest != "" {
			sendPrivateUDP(dest, message)
		}
	}
}

func originsHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		origins := g.GetOriginsList()

		jsonOriginsList := make([]jsonOrigin, len(origins))
		for i, v := range origins {
			jsonOriginsList[i] = jsonOrigin{Name: v}
		}
		testJSON, err := json.Marshal(jsonOriginsList)
		if err != nil {
			panic(err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(testJSON)
	}
}

func shareFileHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "POST":
		err := r.ParseForm()
		if err != nil {
			panic(err)
		}
		filePath := r.PostFormValue("file")
		sendFileShareUDP(filePath)
	}
}

func downloadFileHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "POST":
		err := r.ParseForm()
		if err != nil {
			panic(err)
		}
		fileName := r.PostFormValue("file")
		from := r.PostFormValue("origin")
		metahash := r.PostFormValue("metahash")

		if fileName == "" {
			fmt.Printf("DL Request error: File Save As name empty\n")
		} else if from == "" {
			fmt.Printf("DL Request error: origin name empty\n")
		} else {
			sendFileDownloadUDP(fileName, from, metahash)
		}
	}
}

func sendUDP(text string) {
	addr, _ := net.ResolveUDPAddr("udp4", "localhost:5500")
	addrGossiper, _ := net.ResolveUDPAddr("udp4", "localhost:"+uiport)
	conn, _ := net.ListenUDP("udp4", addr)
	msg := &dto.SimpleMessage{Contents: text}
	packet := &dto.GossipPacket{Simple: msg}
	request := &dto.ClientRequest{Packet: packet}
	packetBytes, _ := protobuf.Encode(request)

	conn.WriteToUDP(packetBytes, addrGossiper)
	conn.Close()
}

func sendPrivateUDP(dest string, text string) {
	addr, _ := net.ResolveUDPAddr("udp4", "localhost:5500")
	addrGossiper, _ := net.ResolveUDPAddr("udp4", "localhost:"+uiport)
	conn, _ := net.ListenUDP("udp4", addr)
	msg := &dto.PrivateMessage{Text: text, Destination: dest}
	packet := &dto.GossipPacket{Private: msg}
	request := &dto.ClientRequest{Packet: packet}
	packetBytes, _ := protobuf.Encode(request)

	conn.WriteToUDP(packetBytes, addrGossiper)
	conn.Close()
}

func sendFileShareUDP(fileName string) {
	addr, _ := net.ResolveUDPAddr("udp4", "localhost:5500")
	addrGossiper, _ := net.ResolveUDPAddr("udp4", "localhost:"+uiport)
	conn, _ := net.ListenUDP("udp4", addr)
	fileToShare := &dto.FileToShare{FileName: fileName}
	request := &dto.ClientRequest{FileShare: fileToShare}
	packetBytes, _ := protobuf.Encode(request)

	conn.WriteToUDP(packetBytes, addrGossiper)
	conn.Close()
}

func sendFileDownloadUDP(saveAs string, from string, metahash string) {
	addr, _ := net.ResolveUDPAddr("udp4", "localhost:5500")
	addrGossiper, _ := net.ResolveUDPAddr("udp4", "localhost:"+uiport)
	conn, _ := net.ListenUDP("udp4", addr)
	hash, err := hex.DecodeString(metahash)
	if err != nil {
		fmt.Printf("Hash error: could not convert string to byte format. Is it in hex?\n")
		return
	}
	hash32, ok := fileparsing.ConvertToHash32(hash)
	if !ok {
		return
	}
	fileToDownload := &dto.FileToDownload{FileName: saveAs, Origin: from, Metahash: hash32}
	request := &dto.ClientRequest{FileDownload: fileToDownload}
	packetBytes, _ := protobuf.Encode(request)

	conn.WriteToUDP(packetBytes, addrGossiper)
	conn.Close()
}
