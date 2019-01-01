package dht

import (
	"fmt"
	"strings"
)

const IDByteSize = 160 / 8

type TypeID [IDByteSize]byte

func String(ns []*NodeState) string {
	nodes := make([]string, 0)
	for _, node := range ns {
		nodes = append(nodes, fmt.Sprintf("%x", node.NodeID))
	}
	return strings.Join(nodes, ",")
}

type NodeState struct {
	NodeID  TypeID
	Address string // IP address with the form "host:port"
}

type Message struct {
	Nonce       uint64
	SenderID    TypeID
	Store       *Store
	Ping        *Ping
	PingReply   *PingReply
	NodeLookup  *NodeLookup
	NodeReply   *NodeReply
	ValueLookup *ValueLookup
	ValueReply  *ValueReply
}

func (msg *Message) GetUnderlyingType() string {
	if msg.Store != nil {
		return "store"
	} else if msg.Ping != nil {
		return "ping"
	} else if msg.PingReply != nil {
		return "pingreply"
	} else if msg.NodeLookup != nil {
		return "nodelookup"
	} else if msg.NodeReply != nil {
		return "nodereply"
	} else if msg.ValueLookup != nil {
		return "valuelookup"
	} else if msg.ValueReply != nil {
		return "valuereply"
	} else {
		return ""
	}
}

type Store struct {
	Key  TypeID
	Type string // PUT or POST
	Data []byte
}

type Ping struct{}

type PingReply struct{}

type NodeLookup struct {
	NodeID TypeID
}

type NodeReply struct {
	NodeStates []*NodeState
}

type ValueLookup struct {
	Key TypeID
}

type ValueReply struct {
	NodeStates *[]*NodeState
	Data       *[]byte
}
