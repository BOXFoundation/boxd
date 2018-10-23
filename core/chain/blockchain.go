// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package chain

import (
	"bytes"
	"errors"

	"github.com/BOXFoundation/boxd/boxd/service"
	"github.com/BOXFoundation/boxd/core"
	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/crypto"
	"github.com/BOXFoundation/boxd/log"
	"github.com/BOXFoundation/boxd/p2p"
	"github.com/BOXFoundation/boxd/storage"
	"github.com/BOXFoundation/boxd/util"
	lru "github.com/hashicorp/golang-lru"
	"github.com/jbenet/goprocess"
)

// const defines constants
const (
	BlockMsgChBufferSize = 1024

	Tail = "tail_block"

	BlockTableName = "core_block"
	TxTableName    = "core_tx"

	MaxTimeOffsetSeconds = 2 * 60 * 60
	MaxBlockSize         = 32000000
	CoinbaseLib          = 100
	maxBlockSigOpCnt     = 80000
	LockTimeThreshold    = 5e8 // Tue Nov 5 00:53:20 1985 UTC
	medianTimeBlocks     = 11
	sequenceLockTimeMask = 0x0000ffff
	PeriodDuration       = 3600 * 24 * 100 / 5

	sequenceLockTimeIsSeconds     = 1 << 22
	sequenceLockTimeGranularity   = 9
	unminedHeight                 = 0x7fffffff
	MaxBlockHeaderCountInSyncTask = 1024
)

var logger = log.NewLogger("chain") // logger

var _ service.ChainReader = (*BlockChain)(nil)

// BlockChain define chain struct
type BlockChain struct {
	notifiee                  p2p.Net
	newblockMsgCh             chan p2p.Message
	dbBlock                   storage.Table
	DbTx                      storage.Table
	genesis                   *types.Block
	tail                      *types.Block
	proc                      goprocess.Process
	LongestChainHeight        int32
	longestChainTip           *types.Block
	cache                     *lru.Cache
	hashToOrphanBlock         map[crypto.HashType]*types.Block
	orphanBlockHashToChildren map[crypto.HashType][]*types.Block
}

// NewBlockChain return a blockchain.
func NewBlockChain(parent goprocess.Process, notifiee p2p.Net, db storage.Storage) (*BlockChain, error) {

	b := &BlockChain{
		notifiee:                  notifiee,
		newblockMsgCh:             make(chan p2p.Message, BlockMsgChBufferSize),
		proc:                      goprocess.WithParent(parent),
		hashToOrphanBlock:         make(map[crypto.HashType]*types.Block),
		orphanBlockHashToChildren: make(map[crypto.HashType][]*types.Block),
	}
	var err error
	b.cache, err = lru.New(512)
	if err != nil {
		return nil, err
	}
	b.dbBlock, err = db.Table(BlockTableName)
	if err != nil {
		return nil, err
	}

	b.DbTx, err = db.Table(TxTableName)
	if err != nil {
		return nil, err
	}

	genesis, err := b.loadGenesis()
	if err != nil {
		logger.Error("Failed to load genesis block ", err)
		return nil, err
	}
	b.genesis = genesis

	tail, err := b.LoadTailBlock()
	if err != nil {
		logger.Error("Failed to load tail block ", err)
		return nil, err
	}
	b.tail = tail
	b.LongestChainHeight = tail.Height

	return b, nil
}

// implement interface service.Server
var _ service.Server = (*BlockChain)(nil)

// Run launch blockchain.
func (chain *BlockChain) Run() error {
	chain.subscribeMessageNotifiee()
	chain.proc.Go(chain.loop)

	return nil
}

// Proc returns the goprocess of the BlockChain
func (chain *BlockChain) Proc() goprocess.Process {
	return chain.proc
}

// Stop the blockchain service
func (chain *BlockChain) Stop() {
	chain.proc.Close()
}

func (chain *BlockChain) subscribeMessageNotifiee() {
	chain.notifiee.Subscribe(p2p.NewNotifiee(p2p.NewBlockMsg, chain.newblockMsgCh))
}

func (chain *BlockChain) loop(p goprocess.Process) {
	logger.Info("Waitting for new block message...")
	for {
		select {
		case msg := <-chain.newblockMsgCh:
			chain.processBlockMsg(msg)
		case <-p.Closing():
			logger.Info("Quit blockchain loop.")
			return
		}
	}
}

func (chain *BlockChain) processBlockMsg(msg p2p.Message) error {
	block := new(types.Block)
	if err := block.Unmarshal(msg.Body()); err != nil {
		return err
	}

	// process block
	chain.ProcessBlock(block, false)

	return nil
}

// ProcessBlock is used to handle new blocks.
func (chain *BlockChain) ProcessBlock(block *types.Block, broadcast bool) (bool, bool, error) {
	blockHash := block.BlockHash()
	logger.Infof("Processing block hash: %v", *blockHash)

	// The block must not already exist in the main chain or side chains.
	if exists := chain.blockExists(*blockHash); exists {
		logger.Warnf("already have block %v", blockHash)
		return false, false, core.ErrBlockExists
	}

	// The block must not already exist as an orphan.
	if _, exists := chain.hashToOrphanBlock[*blockHash]; exists {
		logger.Warnf("already have block (orphan) %v", blockHash)
		return false, false, core.ErrBlockExists
	}

	if err := validateBlock(block, util.NewMedianTime()); err != nil {
		logger.Error(err)
		return false, false, err
	}

	prevHash := block.Header.PrevBlockHash
	if prevHashExists := chain.blockExists(prevHash); !prevHashExists {
		// Orphan block.
		logger.Infof("Adding orphan block %v with parent %v", *blockHash, prevHash)
		chain.addOrphanBlock(block, *blockHash, prevHash)
		return false, true, nil
	}

	// All context-free checks pass, try to accept the block into the chain.
	isMainChain, err := chain.tryAcceptBlock(block)
	if err != nil {
		logger.Error(err)
		return false, false, err
	}

	if err := chain.processOrphans(block); err != nil {
		logger.Error(err)
		return false, false, err
	}

	logger.Infof("Accepted block hash: %v", blockHash)
	if broadcast {
		chain.notifiee.Broadcast(p2p.NewBlockMsg, block)
	}
	return isMainChain, false, nil
}

func (chain *BlockChain) blockExists(blockHash crypto.HashType) bool {
	if chain.cache.Contains(blockHash) {
		return true
	}
	block, err := chain.LoadBlockByHash(blockHash)
	if err != nil || block == nil {
		return false
	}
	return true
}

// tryAcceptBlock validates block within the chain context and see if it can be accepted.
// Return whether it is on the main chain or not.
func (chain *BlockChain) tryAcceptBlock(block *types.Block) (bool, error) {
	blockHash := block.BlockHash()
	// must not be orphan if reaching here
	parentBlock := chain.getParentBlock(block)

	// The height of this block must be one more than the referenced parent block.
	if block.Height != parentBlock.Height+1 {
		logger.Errorf("Block %v's height is %d, but its parent's height is %d", blockHash, block.Height, parentBlock.Height)
		return false, core.ErrWrongBlockHeight
	}

	if err := chain.StoreBlockToDb(block); err != nil {
		return false, err
	}
	chain.cache.Add(*blockHash, block)

	// Connect the passed block to the main or side chain.
	// There are 3 cases.
	parentHash := &block.Header.PrevBlockHash
	tailHash := chain.TailBlock().BlockHash()
	utxoSet := NewUtxoSet()
	if err := utxoSet.LoadBlockUtxos(block, chain.DbTx); err != nil {
		return false, err
	}
	// Case 1): The new block extends the main chain.
	// We expect this to be the most common case.
	if parentHash.IsEqual(tailHash) {
		if err := chain.tryConnectBlockToMainChain(block, utxoSet); err != nil {
			return false, err
		}
		return true, nil
	}

	// Case 2): The block extends or creats a side chain, which is not longer than the main chain.
	if block.Height <= chain.LongestChainHeight {
		logger.Infof("Block %v extends a side chain to height %d without causing reorg, main chain height %d",
			blockHash, block.Height, chain.LongestChainHeight)
		return false, nil
	}

	// Case 3): Extended side chain is longer than the main chain and becomes the new main chain.
	logger.Infof("REORGANIZE: Block %v is causing a reorganization.", blockHash)
	if err := chain.reorganize(block, utxoSet); err != nil {
		return false, err
	}

	// Notify the caller that the new block was accepted into the block chain.
	// The caller would typically want to react by relaying the inventory to other peers.
	// TODO
	// chain.sendNotification(NTBlockAccepted, block)

	// This block is now the end of the best chain.
	chain.SetTailBlock(block, utxoSet)
	return true, nil
}

func (chain *BlockChain) addOrphanBlock(orphan *types.Block, orphanHash crypto.HashType, parentHash crypto.HashType) {
	chain.hashToOrphanBlock[orphanHash] = orphan
	// Add to parent hash map lookup index for faster dependency lookups.
	chain.orphanBlockHashToChildren[parentHash] = append(chain.orphanBlockHashToChildren[parentHash], orphan)
}

func (chain *BlockChain) processOrphans(block *types.Block) error {
	// Start with processing at least the passed block.
	acceptedBlocks := []*types.Block{block}

	// TODO: @XIAOHUI determines whether the length of an array can be changed while traversing an array?
	// Note: use index here instead of range because acceptedBlocks can be extended inside the loop
	for i := 0; i < len(acceptedBlocks); i++ {
		acceptedBlock := acceptedBlocks[i]
		acceptedBlockHash := acceptedBlock.BlockHash()

		// Look up all orphans that are parented by the block we just accepted.
		childOrphans := chain.orphanBlockHashToChildren[*acceptedBlockHash]
		for _, orphan := range childOrphans {
			orphanHash := orphan.BlockHash()
			// Remove the orphan from the orphan pool even if it is not accepted
			// since it will not be accepted later if rejected once.
			delete(chain.hashToOrphanBlock, *orphanHash)
			// Potentially accept the block into the block chain.
			if _, err := chain.tryAcceptBlock(orphan); err != nil {
				return err
			}
			// Add this block to the list of blocks to process so any orphan
			// blocks that depend on this block are handled too.
			acceptedBlocks = append(acceptedBlocks, orphan)
		}
		// Remove the acceptedBlock from the orphan children map.
		delete(chain.orphanBlockHashToChildren, *acceptedBlockHash)
	}
	return nil
}

// Finds the parent of a block. Return nil if nonexistent
func (chain *BlockChain) getParentBlock(block *types.Block) *types.Block {

	// check for genesis.
	if block.BlockHash().IsEqual(chain.genesis.BlockHash()) {
		return chain.genesis
	}
	if target, ok := chain.cache.Get(block.Header.PrevBlockHash); ok {
		return target.(*types.Block)
	}
	target, err := chain.LoadBlockByHash(block.Header.PrevBlockHash)
	if err != nil {
		return nil
	}
	return target
}

func (chain *BlockChain) ancestor(block *types.Block, height int32) *types.Block {
	if height < 0 || height > block.Height {
		return nil
	}

	iterBlock := block
	for iterBlock != nil && iterBlock.Height != height {
		iterBlock = chain.getParentBlock(iterBlock)
	}
	return iterBlock
}

// tryConnectBlockToMainChain tries to append the passed block to the main chain.
// It enforces multiple rules such as double spends and script verification.
func (chain *BlockChain) tryConnectBlockToMainChain(block *types.Block, utxoSet *UtxoSet) error {
	// // TODO: needed?
	// // The coinbase for the Genesis block is not spendable, so just return
	// // an error now.
	// if block.BlockHash.IsEqual(genesisHash) {
	// 	str := "the coinbase for the genesis block is not spendable"
	// 	return ErrMissingTxOut
	// }
	transactions := block.Txs
	// Perform several checks on the inputs for each transaction.
	// Also accumulate the total fees.
	var totalFees int64
	for _, tx := range transactions {
		txFee, err := ValidateTxInputs(utxoSet, tx, block.Height)
		if err != nil {
			return err
		}

		// Check for overflow.
		lastTotalFees := totalFees
		totalFees += txFee
		if totalFees < lastTotalFees {
			return core.ErrBadFees
		}

		// Update utxos by applying this tx
		if err := utxoSet.ApplyTx(tx, block.Height); err != nil {
			return err
		}
	}

	// Ensure coinbase does not output more than block reward.
	var totalCoinbaseOutput int64
	for _, txOut := range transactions[0].Vout {
		totalCoinbaseOutput += txOut.Value
	}
	expectedCoinbaseOutput := CalcBlockSubsidy(block.Height) + totalFees
	if totalCoinbaseOutput > expectedCoinbaseOutput {
		logger.Errorf("coinbase transaction for block pays %v which is more than expected value of %v",
			totalCoinbaseOutput, expectedCoinbaseOutput)
		return core.ErrBadCoinbaseValue
	}

	// Enforce the sequence number based relative lock-times.
	medianTime := chain.calcPastMedianTime(chain.getParentBlock(block))
	for _, tx := range transactions {
		// A transaction can only be included in a block
		// if all of its input sequence locks are active.
		lockTime, err := chain.calcLockTime(utxoSet, block, tx)
		if err != nil {
			return err
		}
		if !sequenceLockActive(lockTime, block.Height, medianTime) {
			logger.Errorf("block contains transaction whose input sequence locks are not met")
			return core.ErrUnfinalizedTx
		}
	}

	if err := validateBlockScripts(block); err != nil {
		return err
	}
	chain.SetTailBlock(block, utxoSet)
	// Notify others such as mempool.
	chain.notifyBlockConnectionUpdate(block, true)
	return nil
}

// findFork returns final common block between the passed block and the main chain (i.e., fork point)
// and blocks to be detached and attached
func (chain *BlockChain) findFork(block *types.Block) (*types.Block, []*types.Block, []*types.Block) {
	if block.Height <= chain.LongestChainHeight {
		logger.Panicf("Side chain (height: %d) is not longer than main chain (height: %d) during chain reorg",
			block.Height, chain.LongestChainHeight)
	}
	detachBlocks := make([]*types.Block, 0)
	attachBlocks := make([]*types.Block, 0)

	// Start both chain from same height by moving up side chain
	sideChainBlock := block
	for i := block.Height; i > chain.LongestChainHeight; i-- {
		if sideChainBlock == nil {
			logger.Panicf("Block on side chain shall not be nil before reaching main chain height during reorg")
		}
		attachBlocks = append(attachBlocks, sideChainBlock)
		sideChainBlock = chain.getParentBlock(sideChainBlock)
	}

	// Compare two blocks at the same height till they are identical: the fork point
	mainChainBlock, found := chain.TailBlock(), false
	for mainChainBlock != nil && sideChainBlock != nil {
		if mainChainBlock.Height != sideChainBlock.Height {
			logger.Panicf("Expect to compare main chain and side chain block at same height")
		}
		mainChainHash := mainChainBlock.BlockHash()
		sideChainHash := sideChainBlock.BlockHash()
		if mainChainHash.IsEqual(sideChainHash) {
			found = true
			break
		}
		detachBlocks = append(detachBlocks, mainChainBlock)
		attachBlocks = append(attachBlocks, sideChainBlock)
		mainChainBlock, sideChainBlock = chain.getParentBlock(mainChainBlock), chain.getParentBlock(sideChainBlock)
	}
	if !found {
		logger.Panicf("Fork point not found, but main chain and side chain share at least one common block, i.e., genesis")
	}
	if len(attachBlocks) <= len(detachBlocks) {
		logger.Panicf("Blocks to be attached (%d) should be strictly more than ones to be detached (%d)", len(attachBlocks), len(detachBlocks))
	}
	return mainChainBlock, detachBlocks, attachBlocks
}

func (chain *BlockChain) revertBlock(block *types.Block, utxoSet *UtxoSet) error {

	if err := utxoSet.LoadBlockUtxos(block, chain.DbTx); err != nil {
		return err
	}
	if err := utxoSet.RevertBlock(block); err != nil {
		return err
	}

	return chain.notifyBlockConnectionUpdate(block, false)
}

func (chain *BlockChain) applyBlock(block *types.Block, utxoSet *UtxoSet) error {

	if err := utxoSet.LoadBlockUtxos(block, chain.DbTx); err != nil {
		return err
	}
	if err := utxoSet.ApplyBlock(block); err != nil {
		return err
	}

	return chain.notifyBlockConnectionUpdate(block, true)
}

func (chain *BlockChain) notifyBlockConnectionUpdate(block *types.Block, connected bool) error {
	updateMsg := UpdateMsg{
		Connected: connected,
		Block:     block,
	}
	chain.notifiee.Notify(NewLocalMessage(p2p.ChainUpdateMsg, updateMsg))
	return nil
}

func (chain *BlockChain) reorganize(block *types.Block, utxoSet *UtxoSet) error {
	// Find the common ancestor of the main chain and side chain
	_, detachBlocks, attachBlocks := chain.findFork(block)

	// Detach the blocks that form the (now) old fork from the main chain.
	// From tip to fork, not including fork
	for _, detachBlock := range detachBlocks {
		if err := chain.revertBlock(detachBlock, utxoSet); err != nil {
			return err
		}
	}

	// Attach the blocks that form the new chain to the main chain starting at the
	// common ancenstor (the point where the chain forked).
	// From fork to tip, not including fork
	for blockIdx := len(attachBlocks) - 1; blockIdx >= 0; blockIdx-- {
		attachBlock := attachBlocks[blockIdx]
		if err := chain.applyBlock(attachBlock, utxoSet); err != nil {
			return err
		}
	}

	return nil
}

// StoreTailBlock store tail block to db.
func (chain *BlockChain) StoreTailBlock(block *types.Block) error {
	data, err := block.Marshal()
	if err != nil {
		return err
	}
	return chain.dbBlock.Put([]byte(Tail), data)
}

// TailBlock return chain tail block.
func (chain *BlockChain) TailBlock() *types.Block {
	return chain.tail
}

// LoadUtxoByPubKey loads utxos of a public key
func (chain *BlockChain) LoadUtxoByPubKey(pubkey []byte) (map[types.OutPoint]*types.UtxoWrap, error) {
	res := make(map[types.OutPoint]*types.UtxoWrap)
	// for out, entry := range chain.utxoSet.utxoMap {
	// 	if bytes.Equal(pubkey, entry.Output.ScriptPubKey) {
	// 		res[out] = entry
	// 	}
	// }
	return res, nil
}

// ListAllUtxos list all the available utxos for testing purpose
func (chain *BlockChain) ListAllUtxos() map[types.OutPoint]*types.UtxoWrap {
	return nil
}

// LoadUtxoByPubKeyScript list all the available utxos owned by a public key bytes
func (chain *BlockChain) LoadUtxoByPubKeyScript(pubkey []byte) (map[types.OutPoint]*types.UtxoWrap, error) {
	return nil, nil
}

// GetBlockHeight returns current height of main chain
func (chain *BlockChain) GetBlockHeight() int32 {
	return chain.LongestChainHeight
}

// GetChainDB returns chain db
func (chain *BlockChain) GetChainDB() storage.Table {
	return chain.dbBlock
}

// GetBlockHash finds the block in target height of main chain and returns it's hash
func (chain *BlockChain) GetBlockHash(blockHeight int32) (*crypto.HashType, error) {
	block, err := chain.LoadBlockByHeight(blockHeight)
	if err != nil {
		return nil, err
	}
	return block.BlockHash(), nil
}

// SetTailBlock sets chain tail block.
func (chain *BlockChain) SetTailBlock(tail *types.Block, utxoSet *UtxoSet) error {

	// save utxoset to database
	if err := utxoSet.WriteUtxoSetToDB(chain.DbTx); err != nil {
		return err
	}

	// save current tail to database
	if err := chain.StoreTailBlock(tail); err != nil {
		return err
	}

	// save tx index
	if err := chain.WirteTxIndex(tail); err != nil {
		return err
	}
	chain.LongestChainHeight = tail.Height
	chain.tail = tail
	logger.Infof("Change New Tail. Hash: %s Height: %d", tail.BlockHash().String(), tail.Height)
	return nil
}

func (chain *BlockChain) loadGenesis() (*types.Block, error) {

	if ok, _ := chain.dbBlock.Has(genesisHash[:]); ok {
		genesisBlockFromDb, err := chain.LoadBlockByHash(genesisHash)
		if err != nil {
			return nil, err
		}
		return genesisBlockFromDb, nil
	}

	genesisBin, err := genesisBlock.Marshal()
	if err != nil {
		return nil, err
	}
	chain.dbBlock.Put(genesisHash[:], genesisBin)

	return &genesisBlock, nil

}

// LoadTailBlock load tail block
func (chain *BlockChain) LoadTailBlock() (*types.Block, error) {
	if chain.tail != nil {
		return chain.tail, nil
	}
	if ok, _ := chain.dbBlock.Has([]byte(Tail)); ok {
		tailBin, err := chain.dbBlock.Get([]byte(Tail))
		if err != nil {
			return nil, err
		}

		tailBlock := new(types.Block)
		if err := tailBlock.Unmarshal(tailBin); err != nil {
			return nil, err
		}

		return tailBlock, nil

	}

	tailBin, err := genesisBlock.Marshal()
	if err != nil {
		return nil, err
	}
	chain.dbBlock.Put([]byte(Tail), tailBin)

	return &genesisBlock, nil
}

// LoadBlockByHash load block by hash from db.
func (chain *BlockChain) LoadBlockByHash(hash crypto.HashType) (*types.Block, error) {

	blockBin, err := chain.dbBlock.Get(hash[:])
	if err != nil {
		return nil, err
	}

	block := new(types.Block)
	if err := block.Unmarshal(blockBin); err != nil {
		return nil, err
	}

	return block, nil
}

// LoadBlockByHeight load block by height from db.
func (chain *BlockChain) LoadBlockByHeight(height int32) (*types.Block, error) {

	blockBin, err := chain.dbBlock.Get(util.FromInt32(height))
	if err != nil {
		return nil, err
	}

	block := new(types.Block)
	if err := block.Unmarshal(blockBin); err != nil {
		return nil, err
	}

	return block, nil
}

// StoreBlockToDb store block to db.
func (chain *BlockChain) StoreBlockToDb(block *types.Block) error {
	data, err := block.Marshal()
	if err != nil {
		return err
	}
	hash := block.BlockHash()
	if err := chain.dbBlock.Put(util.FromInt32(block.Height), data); err != nil {
		return err
	}
	return chain.dbBlock.Put((*hash)[:], data)
}

// LoadTxByHash load transaction with hash.
func (chain *BlockChain) LoadTxByHash(hash crypto.HashType) (*types.Transaction, error) {

	txIndex, err := chain.dbBlock.Get(hash[:])
	if err != nil {
		return nil, err
	}
	buf := bytes.NewBuffer(txIndex)
	height, err := util.ReadInt32(buf)
	if err != nil {
		return nil, err
	}
	idx, err := util.ReadInt32(buf)
	if err != nil {
		return nil, err
	}
	block, err := chain.LoadBlockByHeight(height)
	if err != nil {
		return nil, err
	}
	if block != nil {
		tx := block.Txs[idx]
		target, err := tx.TxHash()
		if err != nil {
			return nil, err
		}
		if *target == hash {
			return tx, nil
		}
	}
	return nil, errors.New("Failed to load tx with hash")
}

// WirteTxIndex build tx index in block
func (chain *BlockChain) WirteTxIndex(block *types.Block) error {

	height := block.Height
	batch := chain.dbBlock.NewBatch()
	defer batch.Close()
	for idx, v := range block.Txs {
		var buf bytes.Buffer
		if err := util.WriteInt32(&buf, height); err != nil {
			return err
		}
		if err := util.WriteInt32(&buf, int32(idx)); err != nil {
			return err
		}
		hash, err := v.TxHash()
		if err != nil {
			return err
		}
		batch.Put(hash[:], buf.Bytes())
	}

	return batch.Write()
}

// LocateForkPointAndFetchHeaders return block headers when get locate fork point request for sync service.
func (chain *BlockChain) LocateForkPointAndFetchHeaders(hashes []*crypto.HashType) ([]*crypto.HashType, error) {
	tailHeight := chain.tail.Height
	for index := range hashes {
		block, err := chain.LoadBlockByHash(*hashes[index])
		if err != nil {
			return nil, err
		}
		if block != nil {
			result := []*crypto.HashType{}
			currentHeight := block.Height + 1
			if tailHeight-block.Height < MaxBlockHeaderCountInSyncTask {
				for currentHeight <= tailHeight {
					block, err := chain.LoadBlockByHeight(currentHeight)
					if err != nil {
						return nil, err
					}
					result = append(result, block.BlockHash())
					currentHeight++
				}
				return result, nil
			}

			var idx int32
			for idx < MaxBlockHeaderCountInSyncTask {
				block, err := chain.LoadBlockByHeight(currentHeight + idx)
				if err != nil {
					return nil, err
				}
				result = append(result, block.BlockHash())
				idx++
			}
			return result, nil
		}

	}
	return nil, nil
}

// CalcRootHashForNBlocks return root hash for N blocks.
func (chain *BlockChain) CalcRootHashForNBlocks(hash crypto.HashType, num int32) (*crypto.HashType, error) {

	block, err := chain.LoadBlockByHash(hash)
	if err != nil {
		return nil, err
	}
	tailHeight := chain.tail.Height
	currentHeight := block.Height + 1
	if tailHeight-currentHeight < num {
		return nil, errors.New("Invalid params num")
	}

	var idx int32
	hashes := make([]*crypto.HashType, num)
	for idx < num {
		block, err := chain.LoadBlockByHeight(currentHeight + idx)
		if err != nil {
			return nil, err
		}
		hashes[idx] = block.BlockHash()
		idx++
	}
	merkleRoot := util.BuildMerkleRoot(hashes)
	rootHash := merkleRoot[len(merkleRoot)-1]
	return rootHash, nil
}

// FetchNBlockAfterSpecificHash get N block after specific hash.
func (chain *BlockChain) FetchNBlockAfterSpecificHash(hash crypto.HashType, num int32) ([]*types.Block, error) {
	block, err := chain.LoadBlockByHash(hash)
	if err != nil {
		return nil, err
	}
	tailHeight := chain.tail.Height
	currentHeight := block.Height + 1
	if tailHeight-currentHeight < num {
		return nil, errors.New("Invalid params num")
	}
	var idx int32
	blocks := make([]*types.Block, num)
	for idx < num {
		block, err := chain.LoadBlockByHeight(currentHeight + idx)
		if err != nil {
			return nil, err
		}
		blocks[idx] = block
		idx++
	}
	return blocks, nil
}
