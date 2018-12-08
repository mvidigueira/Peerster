package gossiper

import "github.com/mvidigueira/Peerster/dto"

//UI (frontend) functionality

//FrontEndMessage - eases frontend message display
type FrontEndMessage struct {
	Origin string
	Text   string
	Type   string
}

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

//ConvertToFEMList - converts an array of RumorMessage to an array of FrontEndMessage
func ConvertToFEMList(rmList []dto.RumorMessage) (femList []FrontEndMessage) {
	femList = make([]FrontEndMessage, len(rmList))
	var msgType string
	for i, rm := range rmList {
		if rm.ID == 0 {
			msgType = "Private"
		} else {
			msgType = "Gossip"
		}
		femList[i] = FrontEndMessage{Origin: rm.Origin, Text: rm.Text, Type: msgType}
	}
	return femList
}

//GetMatchesList - returns a MetahashFilenamePair array with all known matches in order
func (g *Gossiper) GetMatchesList() []dto.MetahashFilenamePair {
	return g.matchesGUImap.GetArrayCopy()
}
