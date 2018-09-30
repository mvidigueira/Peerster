package main

import (
	"flag"
	"strings"
)

func main() {
	UIPort := flag.String("UIPort", "8080", "Port for the UI client (default \"8080\")")
	gossipAddr := flag.String("gossipAddr", "127.0.0.1:5000", "ip:port for the gossiper (default \"127.0.0.1:5000\")")
	name := flag.String("name", "", "name of the gossiper")
	simple := flag.Bool("simple", false, "run gossiper in simple broadcast mode")
	peersStr := flag.String("peers", "", "comma separated list of peers of the form ip:port")

	flag.Parse()

	peers := make([]string, 0)
	if *peersStr != "" {
		peers = strings.Split(*peersStr, ",")
	}
	gossiper := NewGossiper(*gossipAddr, *name, *UIPort, peers)

	go gossiper.clientListenRoutine()
	gossiper.externalListenRoutine()

	*simple = false //this is just so it compiles, dont forget to use simple flag later on
}
