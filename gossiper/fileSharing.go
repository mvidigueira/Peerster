package gossiper

import (
	"fmt"
)

func (g *Gossiper) clientFileShareListenRoutine(cFileShare chan string) {
	for fileName := range cFileShare {
		fmt.Printf("clientFileShareListenRoutine: Share request for file: %s\n", fileName)
	}
}
