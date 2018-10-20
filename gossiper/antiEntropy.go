package gossiper

import "time"

const antiEntropyTimeout = 10 //TODO: change to 1 for delivery

//antiEntropy - implements anti entropy as per the project description
func (g *Gossiper) antiEntropy() {
	if g.UseSimple {
		return
	}
	t := time.NewTicker(antiEntropyTimeout * time.Second)
	for {
		select {
		case <-t.C:
			pick, ok := g.pickRandomPeer([]string{})
			if ok {
				g.sendStatusPacket(pick)
			}
		}
	}
}
