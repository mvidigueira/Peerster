package gossiper

import (
	"fmt"

	"github.com/mvidigueira/Peerster/dto"
)

func (g *Gossiper) updateDSDV(pap *dto.PacketAddressPair) {
	updated := g.routingTable.UpdateEntry(pap.GetSeqID(), pap.GetOrigin(), pap.GetSenderAddress())
	if updated {
		fmt.Printf("DSDV %s %s\n", pap.GetOrigin(), pap.GetSenderAddress())
	}
}

func (g *Gossiper) getNextHop(origin string) (address string, ok bool) {
	return g.routingTable.GetNextHop(origin)
}
