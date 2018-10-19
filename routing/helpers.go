package routing

import "github.com/mvidigueira/Peerster/dto"

//makeRoutingGossip - prepares a gossip packet with no text to be sent
//Meant to be used to spread routing information
func makeRoutingGossip(origin string, seqIDCounter *dto.SafeCounter) *dto.GossipPacket {
	rumor := &dto.RumorMessage{
		ID:     seqIDCounter.GetAndIncrement(),
		Origin: origin,
		Text:   "",
	}
	return &dto.GossipPacket{Rumor: rumor}
}
