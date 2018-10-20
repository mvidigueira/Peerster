package gossiper

import (
	"fmt"
	"time"

	"github.com/mvidigueira/Peerster/dto"
)

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
