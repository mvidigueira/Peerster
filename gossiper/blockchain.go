package gossiper

import (
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/mvidigueira/Peerster/dto"
)

const bitDifficulty = 16
const hashDisplayBytes = 32

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

func checkValid(bc *dto.Block) bool {
	hash32 := bc.Hash()
	first := (bitDifficulty) / 8
	var i int
	for i = 0; i < first; i++ {
		if hash32[i] != 0 {
			return false
		}
	}
	b := byte(0x80)
	for j := 0; j < ((bitDifficulty) % 8); j++ {
		if b&hash32[i] != 0 {
			return false
		}
		b = b >> 1
	}
	return true
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

func (bcl *BlockchainLedger) isPresent(hash [32]byte) (present bool) {
	_, present = bcl.chains[hash]
	return
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
	highestBlock = genesis
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
	if bcl.isPresent(block.Hash()) {
		fmt.Printf("Repeat block\n")
		return false, false, false
	}
	parent, notOrphan := bcl.chains[block.PrevHash]
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
		fmt.Printf("Orphan\n")
		bc = newBlockOfChain(*block)
		bc.appendToBlock(bcl.getGenesis())
		claimedNames, _ = bcl.checkValidTransactionsBackwards(bc)
	}
	bcl.chains[block.Hash()] = bc
	maxLen, highestBlock := bcl.checkOrphansAndReinsert(bc, claimedNames)
	//fmt.Printf("maxlen: %d, highestBlock: %x\n", maxLen, highestBlock.Block.Hash())

	if bc.LengthOfChain > maxLen {
		maxLen = bc.LengthOfChain
		highestBlock = bc
	} else {
		fmt.Printf("Reinserted orphans\n")
	}
	changed, grew := bcl.headChanges(maxLen, highestBlock)
	return true, changed, grew
}

func (bcl *BlockchainLedger) headChanges(lenOfNewFork int, newForkHead *blockOfChain) (changed bool, grew bool) {
	headLongestChain := bcl.chains[bcl.longestChain]
	//blockHash := newForkHead.Block.Hash()
	//fmt.Printf("Head: %x (len: %d), New: %x (len: %d).\n", bcl.longestChain[(32-hashDisplayBytes):], headLongestChain.LengthOfChain, blockHash[(32-hashDisplayBytes):], lenOfNewFork)
	if lenOfNewFork > headLongestChain.LengthOfChain {
		if newForkHead.Block.PrevHash != bcl.longestChain { //fork longer than current chain
			rewind := bcl.getDegreeOfForkage(headLongestChain, newForkHead)
			fmt.Printf("FORK-LONGER rewind %d blocks\n", rewind)
			changed = true
		} else {
			changed = false
			//extending current longest chain
		}
		bcl.mux.Lock()
		bcl.longestChain = newForkHead.Block.Hash()
		bcl.mux.Unlock()
		chainArr := bcl.getBlocksInChain(newForkHead)
		fmt.Printf("CHAIN %s\n", strings.Join(chainArr, " "))
		return changed, true
	} //fork shorter than current chain
	common := bcl.getFirstCommon(headLongestChain, newForkHead)
	fmt.Printf("FORK-SHORTER %x\n", common[(32-hashDisplayBytes):]) //change to Hash()
	return false, false
}

func (bcl *BlockchainLedger) getDegreeOfForkage(first, second *blockOfChain) (rewind int) {
	prevFirst, existsFirst := bcl.chains[first.Block.PrevHash]
	prevSecond, existsSecond := bcl.chains[second.Block.PrevHash]
	if !(existsFirst && existsSecond) {
		return 0
	} else if first.LengthOfChain == 0 || second.LengthOfChain == 0 {
		return 0
	} else if first == prevSecond {
		return 0
	}

	rewind = bcl.getDegreeOfForkage(prevFirst, prevSecond)
	return 1 + rewind
}

func (bcl *BlockchainLedger) getBlocksInChain(bc *blockOfChain) (block []string) {
	names := []string{}
	for _, tx := range bc.Block.Transactions {
		names = append(names, tx.File.Name)
	}
	blockHash := bc.Block.Hash()
	hashS := fmt.Sprintf("%x:%x:%s", blockHash[(32-hashDisplayBytes):], bc.Block.PrevHash[(32-hashDisplayBytes):], strings.Join(names, ","))
	block = []string{hashS}

	prevHash := bc.Block.PrevHash
	prevBc, ok := bcl.chains[prevHash]
	if !ok || prevHash == [32]byte{} {
		return block
	}
	return append(block, bcl.getBlocksInChain(prevBc)...)

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

	first := bitDifficulty / 8
	var i int
	for i = 0; i < first; i++ {
		if hash32[i] != 0 {
			return nil, false
		}
	}
	b := byte(0x80)
	for j := 0; j < (bitDifficulty % 8); j++ {
		if b&hash32[i] != 0 {
			return nil, false
		}
		b = b >> 1
	}

	fmt.Printf("FOUND-BLOCK %x\n", hash32)
	return &dto.BlockPublish{Block: *block, HopLimit: defaultHopLimit}, true
}

func getRandomNonce() (r [32]byte) {
	rand.Read(r[:])
	return r
}

func incrementNonce(nonce [32]byte) (incr [32]byte) {
	incr = nonce
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
	canMine := true
	for {
		select {
		case txPubPap := <-cFileNaming: //different for client
			txPub := txPubPap.Packet.TxPublish
			_, present1 := pendingNames[txPub.File.Name]
			_, present2 := takenNames[txPub.File.Name]
			if !present1 && !present2 {
				pendingNames[txPub.File.Name] = txPub
				lastTimeMined = time.Now()
			}
			_, present3 := everyTxEver[txPub.File.Name]
			if !present3 {
				everyTxEver[txPub.File.Name] = txPub
			}
			g.forwardBlockOrTx(txPubPap.Packet, txPubPap.GetSenderAddress())

		case blockPubPap := <-cBlocks:
			block := &blockPubPap.Packet.BlockPublish.Block
			if !checkValid(block) {
				fmt.Printf("Block with invalid hash received. Ignoring...\n")
				continue
			}
			if g.blockchainLedger.isPresent(block.Hash()) {
				//fmt.Printf("Repeat block\n")
				continue
			}
			_, headChanged, headGrew := g.blockchainLedger.insertBlock(block)
			if headChanged {
				headhash, head = g.blockchainLedger.getHead()
				takenNames := g.blockchainLedger.getTakenNames(head)
				pendingNames = mapDifference(everyTxEver, takenNames)
			} else if headGrew {
				headhash, _ = g.blockchainLedger.getHead()
				pendingNames = mapDifference(pendingNames, getNames(block))
			}
			lastTimeMined = time.Now()
			g.forwardBlockOrTx(blockPubPap.Packet, blockPubPap.GetSenderAddress())
		default:
			//fmt.Printf("Head: %x, Nonce: %x\n", headhash, nonce)
			if canMine {
				blockToMine := createBlock(headhash, nonce, pendingNames)
				blockPub, valid := g.blockchainLedger.mine(blockToMine)
				if valid {
					g.blockchainLedger.insertBlock(blockToMine)
					headhash, _ = g.blockchainLedger.getHead()

					pendingNames = make(map[string]*dto.TxPublish)

					currentTimeMined := time.Now()
					delay := currentTimeMined.Sub(lastTimeMined)
					lastTimeMined = currentTimeMined

					if firstBlock {
						delay = time.Second * 5
						firstBlock = false
					}

					canMine = false
					go g.forwardOwnBlock(&dto.GossipPacket{BlockPublish: blockPub}, 2*delay, &canMine)
				}
				nonce = getRandomNonce() //incrementNonce(nonce)
			} else {
				lastTimeMined = time.Now()
				//fmt.Printf("aiting for publish\n")
			}

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

func (g *Gossiper) forwardOwnBlock(packet *dto.GossipPacket, delay time.Duration, canMine *bool) {
	time.Sleep(delay)
	shouldSend := packet.DecrementHopCount()
	if shouldSend {
		for _, v := range stringArrayDifference(g.peers.GetArrayCopy(), []string{}) {
			g.sendUDP(packet, v)
		}
	}
	*canMine = true
}

func (g *Gossiper) forwardBlockOrTx(packet *dto.GossipPacket, sender string) {
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

func (bcl *BlockchainLedger) getFirstCommon(first, second *blockOfChain) (common [32]byte) {
	firstChain := make([][32]byte, 0)
	secondChain := make([][32]byte, 0)
	var exists bool
	firstChain = append(firstChain, first.Block.Hash())
	for {
		first, exists = bcl.chains[first.Block.PrevHash]
		if !exists || first.Block.PrevHash == [32]byte{0} {
			genesis := [][32]byte{[32]byte{}}
			firstChain = append(genesis[:], firstChain...)
			break
		} else {
			hash := [][32]byte{first.Block.Hash()}
			firstChain = append(hash[:], firstChain...)
		}
	}

	secondChain = append(secondChain, second.Block.Hash())
	for {
		second, exists = bcl.chains[second.Block.PrevHash]
		if !exists || second.Block.PrevHash == [32]byte{0} {
			genesis := [][32]byte{[32]byte{}}
			secondChain = append(genesis[:], secondChain...)
			break
		} else {
			hash := [][32]byte{second.Block.Hash()}
			secondChain = append(hash[:], secondChain...)
		}
	}

	//fmt.Printf("First: %v\nSecond:%v\n", firstChain, secondChain)
	var commonInd int
	for i := 0; i < len(firstChain); i++ {
		if firstChain[i] != secondChain[i] {
			break
		}
		commonInd = i
	}
	return firstChain[commonInd]
}
