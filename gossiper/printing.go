package gossiper

import (
	"fmt"
	"strings"

	"github.com/mvidigueira/Peerster/dto"
)

//PRINTING
//printKnownPeers - prints all known peers, format according to project description
func (g *Gossiper) printKnownPeers() {
	fmt.Println("PEERS " + strings.Join(g.peers.GetArrayCopy(), ","))
}

//printStatusMessage - prints a status message, format according to project description
func printStatusMessage(senderAddress string, status dto.StatusPacket) {
	fmt.Printf("STATUS from %v %v\n", senderAddress, status.String())
}

//printClientMessage - prints a client message, format according to project description
func printClientMessage(pair *dto.PacketAddressPair) {
	fmt.Printf("CLIENT MESSAGE %v\n", pair.GetContents())
}

//printGossiperMessage - prints a peer's message (rumor or simple), format according to project description
func printGossiperMessage(pair *dto.PacketAddressPair) {
	switch subtype := pair.Packet.GetUnderlyingType(); subtype {
	case "simple":
		fmt.Printf("SIMPLE MESSAGE origin %s from %s contents %s\n",
			pair.GetOrigin(), pair.GetSenderAddress(), pair.GetContents())
	case "rumor":
		fmt.Printf("RUMOR origin %s from %s ID %v contents %s\n",
			pair.GetOrigin(), pair.GetSenderAddress(), pair.GetSeqID(), pair.GetContents())
	case "private":
		fmt.Printf("PRIVATE origin %s hop-limit %v contents %s\n",
			pair.GetOrigin(), pair.GetHopLimit(), pair.GetContents())
	}
}
