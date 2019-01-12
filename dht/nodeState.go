package dht

import (
	"fmt"
	"strings"

	. "github.com/mvidigueira/Peerster/dht_util"
)

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

type MessageType int

const (
	StoreT MessageType = iota
	PingT
	PingReplyT
	NodeLookupT
	NodeReplyT
	ValueLookupT
	ValueReplyT
)

func (msg *Message) GetUnderlyingType() MessageType {
	if msg.Store != nil {
		return StoreT
	} else if msg.Ping != nil {
		return PingT
	} else if msg.PingReply != nil {
		return PingReplyT
	} else if msg.NodeLookup != nil {
		return NodeLookupT
	} else if msg.NodeReply != nil {
		return NodeReplyT
	} else if msg.ValueLookup != nil {
		return ValueLookupT
	} else if msg.ValueReply != nil {
		return ValueReplyT
	} else {
		return -1
	}
}

type Store struct {
	Key       TypeID
	Type      string // PUT
	Data      []byte
	Encrypted bool
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
	Key          TypeID
	DbKey        string
	DbBucket     string
	Encrypted    bool
	EncryptedKey []byte
}

type ValueReply struct {
	NodeStates *[]*NodeState
	Data       *[]byte
	Encrypted  bool
}
