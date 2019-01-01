package gossiper

import (
	"fmt"
	"time"

	"github.com/mvidigueira/Peerster/dto"
)

const defaultHopLimit = 10

//updateDSDV - given a PacketAddressPair corresponding to a rumor message,
//updates the routingTable aswell as the known origins
func (g *Gossiper) updateDSDV(pap *dto.PacketAddressPair) {
	if pap.GetOrigin() == g.name {
		return
	}

	updated := g.routingTable.UpdateEntry(pap.GetSeqID(), pap.GetOrigin(), pap.GetSenderAddress())
	if updated {
		fmt.Printf("DSDV %s %s\n", pap.GetOrigin(), pap.GetSenderAddress())
	}
	g.origins.AppendUniqueToArray(pap.GetOrigin())
}

//getNextHop - returns the address of the next peer in the path to 'origin'
func (g *Gossiper) getNextHop(origin string) (address string, ok bool) {
	return g.routingTable.GetNextHop(origin)
}

//forward - forwards a packet (Private, DataRequest or DataReply) to the next peer
//in the path to its destination
func (g *Gossiper) forward(packet *dto.GossipPacket) {
	shouldSend := packet.DecrementHopCount()
	if shouldSend {
		nextHop, ok := g.getNextHop(packet.GetDestination())
		if ok {
			if packet.GetUnderlyingType() == "private" {
				fmt.Printf("FORWARDING PRIVATE MESSAGE. Contents: %s, Destination: %s\n", packet.GetContents(), packet.GetDestination())
			}
			g.sendUDP(packet, nextHop)
		}
	}
}

//makeRouteRumorPacket - creates a route rumor packet
func (g *Gossiper) makeRouteRumorPacket() (packet *dto.GossipPacket) {
	rumor := &dto.RumorMessage{
		ID:     g.seqIDCounter.GetAndIncrement(),
		Origin: g.name,
		Text:   "",
	}
	packet = &dto.GossipPacket{Rumor: rumor}
	return
}

//periodicRouteRumor - sends a route rumor packet to a random peer every 'rtimeout' seconds
func (g *Gossiper) periodicRouteRumor() {
	if g.rtimeout == 0 {
		fmt.Printf("Timeout is 0\n")
		return
	}
	t := time.NewTicker(time.Duration(g.rtimeout) * time.Second)

	for {
		peer, ok := g.pickRandomPeer([]string{""})
		if ok {
			packet := g.makeRouteRumorPacket()
			g.addMessage(packet.Rumor)
			fmt.Printf("ATTEMPT to send route rumor message to %s\n", peer)
			g.trySend(packet, peer)
		}
		<-t.C
	}
}

//clientPMListenRoutine - deals with new Private messages from clients
func (g *Gossiper) clientPMListenRoutine(cUIPM chan *dto.PacketAddressPair) {
	for pap := range cUIPM {
		printClientMessage(pap)
		g.printKnownPeers()

		pap.Packet.Private.ID = 0
		pap.Packet.Private.HopLimit = defaultHopLimit
		pap.Packet.Private.Origin = g.name

		if pap.Packet.Private.HopLimit > 0 {
			nextHop, ok := g.getNextHop(pap.GetDestination())
			if ok {
				g.sendUDP(pap.Packet, nextHop)
			}
		}
	}
}

//privateMessageListenRoutine - deals with private packets from other peers
func (g *Gossiper) privateMessageListenRoutine(cPrivate chan *dto.PacketAddressPair) {
	for pap := range cPrivate {
		g.addToPeers(pap.GetSenderAddress())
		if pap.GetDestination() == g.name {
			printGossiperMessage(pap)
			g.printKnownPeers()
			rm := pap.Packet.Private.ToRumorMessage()
			g.latestMessages.AppendToArray(rm)
		} else {
			g.printKnownPeers()
			g.forward(pap.Packet)
		}
	}
}
