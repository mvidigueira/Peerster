package dht

const IDByteSize = 160 / 8

type NodeState struct {
	NodeID  [IDByteSize]byte
	Address string // IP address with the form "host:port"
}

type Message struct {
	Nonce       uint64
	SenderID    [IDByteSize]byte
	Store       *Store
	Ping        *Ping
	PingReply   *PingReply
	NodeLookup  *NodeLookup
	NodeReply   *NodeReply
	ValueLookup *ValueLookup
	ValueReply  *ValueReply
}

type Store struct {
	Key  [IDByteSize]byte
	Data []byte
}

type Ping struct{}

type PingReply struct{}

type NodeLookup struct {
	NodeID [IDByteSize]byte
}

type NodeReply struct {
	NodeStates []*NodeState
}

type ValueLookup struct {
	Key [IDByteSize]byte
}

type ValueReply struct {
	NodeStates []*NodeState
	Data       []byte
}
