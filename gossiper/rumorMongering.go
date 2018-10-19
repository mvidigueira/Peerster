package gossiper

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/mvidigueira/Peerster/dto"
)

//pickRandomPeer - picks a random peer from the list of known peers, excluding any in the 'exceptions" list
func (g *Gossiper) pickRandomPeer(exceptions []string) (string, bool) {
	possiblePicks := stringArrayDifference(g.peers.GetArrayCopy(), exceptions)
	if len(possiblePicks) == 0 {
		return "", false
	}
	peer := possiblePicks[rand.Intn(len(possiblePicks))]
	return peer, true
}

//peerStatusListenRoutine - status listening routine for a specific peer. Has 3 major functionalities:
//1) Informs rumormongering routines currently sending (trySend function) if the peers are in sync.
//2) Updates the corresponding peer if it is detected that he is outdated
//3) After 2), sends a status packet if the gossiper is outdated in comparison with the other peer
func (g *Gossiper) peerStatusListenRoutine(peerAddress string, cStatus chan *dto.StatusPacket) {
	for {
		select {
		case sp := <-cStatus:
			mySp := g.msgMap.GetOwnStatusPacket()
			statusUpdated := dto.StatusPacket{Want: peerStatusUpdated(mySp.Want, sp.Want)}
			printStatusMessage(peerAddress, statusUpdated)

			notMongering := g.quitChanMap.InformListeners(peerAddress, sp)

			theirDesiredMsgs := peerStatusDifference(mySp.Want, sp.Want)

			for _, v := range theirDesiredMsgs {
				rm, _ := g.msgMap.GetMessage(v.Identifier, v.NextID)
				packet := &dto.GossipPacket{Rumor: rm}
				fmt.Printf("MONGERING with %s\n", peerAddress)
				g.sendUDP(packet, peerAddress)
			}

			if notMongering && (len(theirDesiredMsgs) == 0) {
				ourDesiredMsgs := peerStatusDifference(sp.Want, mySp.Want)
				if len(ourDesiredMsgs) != 0 {
					g.sendStatusPacket(peerAddress)
				} else {
					fmt.Printf("IN SYNC WITH %s\n", peerAddress)
				}
			}
		}
	}
}

//trySend - sends a gossip message to 'peer'. returns when either:
//1) the peers are in sync (synchronization handled by peerStatusListenRoutine)
//2) a timeout ocurrs
func (g *Gossiper) trySend(packet *dto.GossipPacket, peer string) bool {
	quit, alreadySending := g.quitChanMap.AddListener(peer, packet.GetOrigin(), packet.GetSeqID())
	if alreadySending { //this should be impossible if using method trySend correctly. Keeping it here for future safety
		return false
	}

	fmt.Printf("MONGERING with %s\n", peer)
	g.sendUDP(packet, peer)

	t := time.NewTicker(rumorMongerTimeout * time.Second)
	for {
		select {
		case <-t.C:
			g.quitChanMap.DeleteListener(peer, packet.GetOrigin(), packet.GetSeqID())
			return false
		case <-quit:
			t.Stop()
			return true
		}
	}
}

//rumorMonger - implements rumor mongering as per the project description
//when choosing peers, excludes the sender on the first pick, and the previous choice on every following pick.
//stops mongering if there are no available peers to send to (e.g. only knows peer who sent the message)
func (g *Gossiper) rumorMonger(packet *dto.GossipPacket, except string) {
	exceptions := []string{}
	if except != "" {
		exceptions = []string{except}
	}
	peer, ok := g.pickRandomPeer(exceptions)
	if !ok {
		return
	}
	for {
		g.trySend(packet, peer)

		if rand.Intn(2) == 1 {
			return
		}
		exceptions = []string{peer}
		peer, ok = g.pickRandomPeer(exceptions)
		if !ok {
			return
		}
		fmt.Printf("FLIPPED COIN sending rumor to %s\n", peer)
	}
}
