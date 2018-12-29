package dht

const IDByteSize = 160 / 8

type typeID [IDByteSize]byte

type NodeState struct {
	NodeID  typeID
	Address string // IP address with the form "host:port"
}

type Message struct {
	Nonce       uint64
	SenderID    typeID
	Store       *Store
	Ping        *Ping
	PingReply   *PingReply
	NodeLookup  *NodeLookup
	NodeReply   *NodeReply
	ValueLookup *ValueLookup
	ValueReply  *ValueReply
}

type Store struct {
	Key  typeID
	Data []byte
}

type Ping struct{}

type PingReply struct{}

type NodeLookup struct {
	NodeID typeID
}

type NodeReply struct {
	NodeStates []*NodeState
}

type ValueLookup struct {
	Key typeID
}

type ValueReply struct {
	NodeStates []*NodeState
	Data       []byte
}
