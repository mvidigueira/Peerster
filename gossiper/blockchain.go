package gossiper

import (
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/mvidigueira/Peerster/dto"
)

type blockOfChain struct {
	LengthOfChain int
	Block         dto.Block
	Forks         map[[32]byte]bool
}

func newGenesisBlock() *blockOfChain {
	return &blockOfChain{LengthOfChain: 0, Forks: make(map[[32]byte]bool)}
}

func newBlockOfChain(block dto.Block) *blockOfChain {
	return &blockOfChain{LengthOfChain: 0, Block: block, Forks: make(map[[32]byte]bool)}
}

func (bc *blockOfChain) appendToBlock(prevbc *blockOfChain) {
	prevbc.Forks[bc.Block.Hash()] = true
	bc.LengthOfChain = prevbc.LengthOfChain + 1
}

func (bc *blockOfChain) getForks() map[[32]byte]bool {
	return bc.Forks
}

func (bc *blockOfChain) removeFork(nextbc *blockOfChain) {
	delete(bc.Forks, nextbc.Block.Hash())
}

func (bc *blockOfChain) checkValidTransactions(claimedNames *dto.SafeStringArray) (valid bool) {
	for _, tx := range bc.Block.Transactions {
		valid = claimedNames.AppendUniqueToArray(tx.File.Name)
		if !valid {
			return false
		}
	}
	return true
}

//BlockchainLedger - TODO: write this
type BlockchainLedger struct {
	longestChain [32]byte
	mux          sync.Mutex
	chains       map[[32]byte]*blockOfChain
}

func NewBlockchainLedger() *BlockchainLedger {
	genesis := newGenesisBlock()
	initialMap := make(map[[32]byte]*blockOfChain)
	initialMap[[32]byte{}] = genesis
	return &BlockchainLedger{longestChain: [32]byte{}, mux: sync.Mutex{}, chains: initialMap}
}

func (bcl *BlockchainLedger) getGenesis() (genesis *blockOfChain) {
	return bcl.chains[[32]byte{}]
}

func (bcl *BlockchainLedger) checkValidTransactionsBackwards(bc *blockOfChain) (claimedNames *dto.SafeStringArray, valid bool) {
	prevHash := bc.Block.PrevHash
	claimedNames = dto.NewSafeStringArray([]string{})
	for _, tx := range bc.Block.Transactions {
		valid = claimedNames.AppendUniqueToArray(tx.File.Name)
		if !valid {
			return nil, false
		}
	}
	for prevHash != [32]byte{} {
		prevBc, ok := bcl.chains[prevHash]
		if !ok {
			return claimedNames, true //orphan start, not breaking any transaction rules
		}
		valid = prevBc.checkValidTransactions(claimedNames)
		if !valid {
			return nil, false //name was claimed before
		}
		prevHash = prevBc.Block.PrevHash
	}
	return claimedNames, true
}

func (bcl *BlockchainLedger) deleteForkFromChain(bc *blockOfChain) {
	for nextHash := range bc.Forks {
		next := bcl.chains[nextHash]
		bcl.deleteForkFromChain(next)
	}
	fmt.Printf("REMOVING %x from chain\n", bc.Block.Hash())
	delete(bcl.chains, bc.Block.Hash())
}

func (bcl *BlockchainLedger) pruneInvalidForks(bc *blockOfChain, claimedNames *dto.SafeStringArray) (valid bool) {
	for _, tx := range bc.Block.Transactions {
		valid := claimedNames.AppendUniqueToArray(tx.File.Name)
		if !valid {
			bcl.deleteForkFromChain(bc)
			return false
		}
	}
	for nextHash := range bc.Forks {
		nextBc, ok := bcl.chains[nextHash]
		if !ok {
			fmt.Printf("Impossible\n")
			return
		}
		bcl.pruneInvalidForks(nextBc, claimedNames)
	}
	return true
}

//newBlock is already inserted at this point, claimedNames are all claimed names before it
func (bcl *BlockchainLedger) recursiveUpdateLength(bc *blockOfChain) (maxLen int, highestBlock *blockOfChain) {
	prevHash := bc.Block.PrevHash
	prevBc := bcl.chains[prevHash]
	bc.LengthOfChain = prevBc.LengthOfChain + 1
	maxLen = bc.LengthOfChain
	highestBlock = bc
	for nextHash := range bc.Forks {
		next := bcl.chains[nextHash]
		len, highest := bcl.recursiveUpdateLength(next)
		if len > maxLen {
			maxLen = len
			highestBlock = highest
		}
	}
	return maxLen, highestBlock
}

//newBlock is already inserted at this point, claimedNames are all claimed names before it
func (bcl *BlockchainLedger) checkOrphansAndReinsert(newBlock *blockOfChain, claimedNames *dto.SafeStringArray) (maxLen int, highestBlock *blockOfChain) {
	newBlockHash := newBlock.Block.Hash()
	genesis := bcl.getGenesis()
	maxLen = 0
	for orphanHash := range genesis.Forks {
		orphanBc := bcl.chains[orphanHash]
		if orphanBc.Block.PrevHash == newBlockHash {
			genesis.removeFork(orphanBc)
			stillValid := bcl.pruneInvalidForks(orphanBc, claimedNames)
			if stillValid {
				orphanBc.appendToBlock(newBlock)
				len, highest := bcl.recursiveUpdateLength(orphanBc)
				if len > maxLen {
					maxLen = len
					highestBlock = highest
				}
			}
		}
	}
	return
}

func (bcl *BlockchainLedger) insertBlock(block *dto.Block) (valid bool, headChanged bool, headGrew bool) {
	parent, notOrphan := bcl.chains[block.Hash()]
	var bc *blockOfChain
	var claimedNames *dto.SafeStringArray
	if notOrphan {
		bc = newBlockOfChain(*block)
		bc.appendToBlock(parent)
		claimedNames, valid = bcl.checkValidTransactionsBackwards(bc)
		if !valid {
			return false, false, false
		}
	} else {
		bc = newBlockOfChain(*block)
		bc.appendToBlock(bcl.getGenesis())
		claimedNames, _ = bcl.checkValidTransactionsBackwards(bc)
	}
	bcl.chains[block.Hash()] = bc
	maxLen, highestBlock := bcl.checkOrphansAndReinsert(bc, claimedNames)

	if bc.LengthOfChain > maxLen {
		maxLen = bc.LengthOfChain
		highestBlock = bc
	}
	changed, grew := bcl.headChanges(maxLen, highestBlock)
	return true, changed, grew
}

func (bcl *BlockchainLedger) headChanges(lenOfNewFork int, newForkHead *blockOfChain) (changed bool, grew bool) {
	headLongestChain := bcl.chains[bcl.longestChain]
	if lenOfNewFork > headLongestChain.LengthOfChain {
		bcl.mux.Lock()
		bcl.longestChain = newForkHead.Block.Hash()
		bcl.mux.Unlock()
		if newForkHead.Block.PrevHash != bcl.longestChain { //fork longer than current chain
			rewind := bcl.getDegreeOfForkage(headLongestChain, newForkHead)
			fmt.Printf("FORK-LONGER rewind %d blocks\n", rewind)
			changed = true
		} else {
			changed = false
			//extending current longest chain
		}
		chainArr := bcl.getBlocksInChain(newForkHead)
		fmt.Printf("CHAIN %s\n", strings.Join(chainArr, ":"))
		return changed, true
	} else { //fork shorter than current chain
		fmt.Printf("FORK-SHORTER [%x]\n", newForkHead.Block.Hash())
		return false, false
	}
}

func (bcl *BlockchainLedger) getDegreeOfForkage(first, second *blockOfChain) (rewind int) {
	if first == second {
		return 0
	}
	prevFirst, existsFirst := bcl.chains[first.Block.PrevHash]
	prevSecond, existsSecond := bcl.chains[first.Block.PrevHash]
	if !(existsFirst && existsSecond) {
		return 1
	}

	return 1 + bcl.getDegreeOfForkage(prevFirst, prevSecond)
}

func (bcl *BlockchainLedger) getBlocksInChain(bc *blockOfChain) (block []string) {
	names := []string{}
	for _, tx := range bc.Block.Transactions {
		names = append(names, tx.File.Name)
	}
	hashS := fmt.Sprintf("%x:%s", bc.Block.Hash(), strings.Join(names, ","))
	block = []string{hashS}

	prevHash := bc.Block.PrevHash
	prevBc, ok := bcl.chains[prevHash]
	if !ok || prevHash == [32]byte{} {
		return block
	} else {
		return append(block, bcl.getBlocksInChain(prevBc)...)
	}
}

func (bcl *BlockchainLedger) getTakenNames(headBc *blockOfChain) (names map[string]bool) {
	names = make(map[string]bool)
	for _, tx := range headBc.Block.Transactions {
		names[tx.File.Name] = true
	}

	prevHash := headBc.Block.PrevHash
	prevBc, ok := bcl.chains[prevHash]
	if !ok || prevHash == [32]byte{} {
		return names
	} else {
		prevNames := bcl.getTakenNames(prevBc)
		for k := range names {
			prevNames[k] = true
		}
		return prevNames
	}
}

func (bcl *BlockchainLedger) getHead() ([32]byte, *blockOfChain) {
	bcl.mux.Lock()
	defer bcl.mux.Unlock()
	return bcl.longestChain, bcl.chains[bcl.longestChain]
}

func (bcl *BlockchainLedger) mine(block *dto.Block) (bPub *dto.BlockPublish, success bool) {
	hash32 := block.Hash()
	for i := 0; i < 5; i++ {
		if hash32[i] != 0 {
			return nil, false
		}
	}
	fmt.Printf("FOUND-BLOCK %x\n", hash32)
	return &dto.BlockPublish{Block: *block, HopLimit: defaultHopLimit}, true
}

func getRandomNonce() (r [32]byte) {
	temp := make([]byte, 32)
	rand.Read(temp)
	copy(r[:], temp)
	return r
}

func incrementNonce(nonce [32]byte) (incr [32]byte) {
	for i := len(nonce) - 1; i >= 0; i-- {
		incr[i] = nonce[i] + 1
		if incr[i] != 0 {
			return incr
		}
	}
	return
}

func (g *Gossiper) blockchainMiningRoutine(cFileNaming chan *dto.PacketAddressPair, cBlocks chan *dto.PacketAddressPair) {
	everyTxEver := make(map[string]*dto.TxPublish)
	pendingNames := make(map[string]*dto.TxPublish)
	headhash, head := g.blockchainLedger.getHead()
	takenNames := g.blockchainLedger.getTakenNames(head)
	nonce := getRandomNonce()
	lastTimeMined := time.Now()
	firstBlock := true
	for {
		select {
		case txPubPap := <-cFileNaming: //different for client
			txPub := txPubPap.Packet.TxPublish
			_, present1 := pendingNames[txPub.File.Name]
			_, present2 := takenNames[txPub.File.Name]
			if !present1 && !present2 {
				pendingNames[txPub.File.Name] = txPub
			}
			_, present3 := everyTxEver[txPub.File.Name]
			if !present3 {
				everyTxEver[txPub.File.Name] = txPub
			}
			g.forwardBlockOrTx(txPubPap.Packet, txPubPap.GetSenderAddress(), 0)
		case blockPubPap := <-cBlocks:
			block := &blockPubPap.Packet.BlockPublish.Block
			_, headChanged, headGrew := g.blockchainLedger.insertBlock(block)
			if headChanged {
				headhash, head = g.blockchainLedger.getHead()
				takenNames := g.blockchainLedger.getTakenNames(head)
				pendingNames = mapDifference(everyTxEver, takenNames)
			} else if headGrew {
				pendingNames = mapDifference(pendingNames, getNames(block))
			}
			g.forwardBlockOrTx(blockPubPap.Packet, blockPubPap.GetSenderAddress(), 0)
		default:
			blockToMine := createBlock(headhash, nonce, pendingNames)
			blockPub, valid := g.blockchainLedger.mine(blockToMine)
			if valid {
				g.blockchainLedger.insertBlock(blockToMine)
				pendingNames = make(map[string]*dto.TxPublish)

				currentTimeMined := time.Now()
				delay := currentTimeMined.Sub(lastTimeMined)
				lastTimeMined = currentTimeMined

				if firstBlock {
					delay = time.Second * 5
					firstBlock = false
				}

				go g.forwardBlockOrTx(&dto.GossipPacket{BlockPublish: blockPub}, "", 2*delay)
			}
			nonce = incrementNonce(nonce)
		}
	}
}

func createBlock(prevHash [32]byte, nonce [32]byte, txMap map[string]*dto.TxPublish) *dto.Block {
	txs := make([]dto.TxPublish, 0, len(txMap))
	for _, v := range txMap {
		txs = append(txs, *v)
	}
	return &dto.Block{PrevHash: prevHash, Nonce: nonce, Transactions: txs}
}

func (g *Gossiper) forwardBlockOrTx(packet *dto.GossipPacket, sender string, delay time.Duration) {
	time.Sleep(delay)
	shouldSend := packet.DecrementHopCount()
	if shouldSend {
		for _, v := range stringArrayDifference(g.peers.GetArrayCopy(), []string{sender}) {
			g.sendUDP(packet, v)
		}
	}
}

func mapDifference(first map[string]*dto.TxPublish, second map[string]bool) (result map[string]*dto.TxPublish) {
	result = make(map[string]*dto.TxPublish)
	for k, v := range first {
		_, present := second[k]
		if !present {
			result[k] = v
		}
	}
	return
}

func getNames(b *dto.Block) (names map[string]bool) {
	names = make(map[string]bool)
	for _, tx := range b.Transactions {
		names[tx.File.Name] = true
	}
	return names
}
