package gossiper

import (
	"fmt"
	"time"

	"github.com/mvidigueira/Peerster/dto"
)

const defaultHopLimit = 10

func (g *Gossiper) updateDSDV(pap *dto.PacketAddressPair) {
	if pap.GetOrigin() == g.name {
		return
	}
	fmt.Printf("ATTEMPT %s %s\n", pap.GetOrigin(), pap.GetSenderAddress())
	updated := g.routingTable.UpdateEntry(pap.GetSeqID(), pap.GetOrigin(), pap.GetSenderAddress())
	if updated {
		fmt.Printf("DSDV %s %s\n", pap.GetOrigin(), pap.GetSenderAddress())
	}
}

func (g *Gossiper) getNextHop(origin string) (address string, ok bool) {
	return g.routingTable.GetNextHop(origin)
}

func (g *Gossiper) periodicRouteRumor() {
	if g.rtimeout == 0 {
		return
	}
	t := time.NewTicker(time.Duration(g.rtimeout) * time.Second)

	for {
		peer, ok := g.pickRandomPeer([]string{""})
		if ok {
			packet := g.makeRouteRumorPacket()
			g.addMessage(packet.Rumor)
			g.trySend(packet, peer)
		}
		<-t.C
	}
}

func (g *Gossiper) makeRouteRumorPacket() (packet *dto.GossipPacket) {
	rumor := &dto.RumorMessage{
		ID:     g.seqIDCounter.GetAndIncrement(),
		Origin: g.name,
		Text:   "",
	}
	packet = &dto.GossipPacket{Rumor: rumor}
	return
}

//clientPMListenRoutine - deals with new messages (simple packets) from clients
func (g *Gossiper) clientPMListenRoutine(cUIPM chan *dto.PacketAddressPair) {
	for pap := range cUIPM {
		printClientMessage(pap)
		g.printKnownPeers()

		pap.Packet.Private.ID = 0
		pap.Packet.Private.HopLimit = defaultHopLimit
		pap.Packet.Private.Origin = g.name

		nextHop, ok := g.getNextHop(pap.GetDestination())
		if ok {
			g.sendUDP(pap.Packet, nextHop)
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
			shouldSend := pap.Packet.Private.DecrementHopCount()
			if shouldSend {
				nextHop, ok := g.getNextHop(pap.GetDestination())
				if ok {
					g.sendUDP(pap.Packet, nextHop)
				}
			}
		}

		g.updateDSDV(pap) //routing
	}
}
