package gossiper

import "github.com/mvidigueira/Peerster/dto"

//UI (frontend) functionality

//GetName - returns the gossiper's name
func (g *Gossiper) GetName() string {
	return g.name
}

//GetLatestMessagesList - returns whatever messages have been received
//since this function was last called
func (g *Gossiper) GetLatestMessagesList() []dto.RumorMessage {
	return g.latestMessages.GetArrayCopyAndDelete()
}

//GetPeersList - returns a string array with all known peers (ip:port)
func (g *Gossiper) GetPeersList() []string {
	return g.peers.GetArrayCopy()
}

//AddPeer - adds a peer (ip:port) to the list of known peers
func (g *Gossiper) AddPeer(peerAdress string) {
	g.addToPeers(peerAdress)
}

//GetOriginsList - returns a string array with all known origins
func (g *Gossiper) GetOriginsList() []string {
	return g.origins.GetArrayCopy()
}
