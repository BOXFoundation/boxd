// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package chain

import (
	"bytes"
	"errors"
	"fmt"
	"time"

	"github.com/BOXFoundation/boxd/boxd/eventbus"
	"github.com/BOXFoundation/boxd/boxd/service"
	"github.com/BOXFoundation/boxd/core"
	"github.com/BOXFoundation/boxd/core/metrics"
	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/crypto"
	"github.com/BOXFoundation/boxd/log"
	"github.com/BOXFoundation/boxd/p2p"
	"github.com/BOXFoundation/boxd/script"
	"github.com/BOXFoundation/boxd/storage"
	"github.com/BOXFoundation/boxd/util"
	"github.com/BOXFoundation/boxd/util/bloom"
	lru "github.com/hashicorp/golang-lru"
	"github.com/jbenet/goprocess"
)

// const defines constants
const (
	BlockMsgChBufferSize        = 1024
	EternalBlockMsgChBufferSize = 65536

	MaxTimeOffsetSeconds = 2 * 60 * 60
	MaxBlockSize         = 32000000
	CoinbaseLib          = 100
	maxBlockSigOpCnt     = 80000
	LockTimeThreshold    = 5e8 // Tue Nov 5 00:53:20 1985 UTC
	medianTimeBlocks     = 11
	sequenceLockTimeMask = 0x0000ffff
	PeriodDuration       = 3600 * 24 * 100 / 5

	sequenceLockTimeIsSeconds   = 1 << 22
	sequenceLockTimeGranularity = 9
	unminedHeight               = 0x7fffffff
	MaxBlocksPerSync            = 1024

	metricsLoopInterval = 2 * time.Second
	BlockFilterCapacity = 100000
)

var logger = log.NewLogger("chain") // logger

var _ service.ChainReader = (*BlockChain)(nil)

// BlockChain define chain struct
type BlockChain struct {
	notifiee                  p2p.Net
	newblockMsgCh             chan p2p.Message
	consensus                 types.Consensus
	db                        storage.Table
	genesis                   *types.Block
	tail                      *types.Block
	eternal                   *types.Block
	proc                      goprocess.Process
	LongestChainHeight        uint32
	longestChainTip           *types.Block
	cache                     *lru.Cache
	bus                       eventbus.Bus
	hashToOrphanBlock         map[crypto.HashType]*types.Block
	orphanBlockHashToChildren map[crypto.HashType][]*types.Block
	syncManager               types.SyncManager
	filterHolder              BloomFilterHolder
}

// UpdateMsg sent from blockchain to, e.g., mempool
type UpdateMsg struct {
	// block connected/disconnected from main chain
	Connected bool
	Block     *types.Block
}

// NewBlockChain return a blockchain.
func NewBlockChain(parent goprocess.Process, notifiee p2p.Net, db storage.Storage, bus eventbus.Bus) (*BlockChain, error) {

	b := &BlockChain{
		notifiee:                  notifiee,
		newblockMsgCh:             make(chan p2p.Message, BlockMsgChBufferSize),
		proc:                      goprocess.WithParent(parent),
		hashToOrphanBlock:         make(map[crypto.HashType]*types.Block),
		orphanBlockHashToChildren: make(map[crypto.HashType][]*types.Block),
		filterHolder:              NewFilterHolder(),
		bus:                       eventbus.Default(),
	}

	var err error
	b.cache, _ = lru.New(512)

	if b.db, err = db.Table(BlockTableName); err != nil {
		return nil, err
	}

	if b.genesis, err = b.loadGenesis(); err != nil {
		logger.Error("Failed to load genesis block ", err)
		return nil, err
	}

	if b.eternal, err = b.loadEternalBlock(); err != nil {
		logger.Error("Failed to load eternal block ", err)
		return nil, err
	}

	if b.tail, err = b.LoadTailBlock(); err != nil {
		logger.Error("Failed to load tail block ", err)
		return nil, err
	}
	b.LongestChainHeight = b.tail.Height

	if err = b.loadFilters(); err != nil {
		logger.Error("Fail to load filters", err)
		return nil, err
	}

	return b, nil
}

// Setup prepare blockchain.
func (chain *BlockChain) Setup(consensus types.Consensus, syncManager types.SyncManager) {
	chain.consensus = consensus
	chain.syncManager = syncManager
}

func (chain *BlockChain) loadFilters() error {
	var i uint32 = 1
	var utxoSet *UtxoSet
	for ; i <= chain.LongestChainHeight; i++ {
		block, err := chain.LoadBlockByHeight(i)
		if err != nil {
			logger.Error("Error try to load block at height", i, err)
			return core.ErrWrongBlockHeight
		}
		utxoSet = NewUtxoSet()
		if err = utxoSet.LoadBlockUtxos(block, chain.db); err != nil {
			logger.Error("Error Loading block utxo", err)
			return err
		}
		if err := chain.filterHolder.AddFilter(i, *block.Hash, chain.DB(), func() bloom.Filter {
			return block.GetFilterForTransactionScript(utxoSet.utxoMap)
		}); err != nil {
			logger.Error("Failed to addFilter", err)
			return err
		}
	}
	utxoSet = nil
	return nil
}

// GetTransactions search the main chain about transaction relate to give address
func (chain *BlockChain) GetTransactions(addr types.Address) ([]*types.Transaction, error) {
	payToPubKeyHashScript := *script.PayToPubKeyHashScript(addr.ScriptAddress())
	hashes := chain.filterHolder.ListMatchedBlockHashes(payToPubKeyHashScript)
	logger.Info(len(hashes), " blocks searched as related to address ", addr.String())
	utxoSet := NewUtxoSet()
	var txs []*types.Transaction
	for _, hash := range hashes {
		block, err := chain.LoadBlockByHash(hash)
		if err != nil {
			return nil, err
		}
		for _, tx := range block.Txs {
			isRelated := false
			for index, vout := range tx.Vout {
				if bytes.Equal(vout.ScriptPubKey, payToPubKeyHashScript) {
					utxoSet.AddUtxo(tx, uint32(index), block.Height)
					isRelated = true
				}
			}
			for _, vin := range tx.Vin {
				if utxoSet.FindUtxo(vin.PrevOutPoint) != nil {
					delete(utxoSet.utxoMap, vin.PrevOutPoint)
					isRelated = true
				}
			}
			if isRelated {
				txs = append(txs, tx)
			}
		}
	}
	logger.Info(len(txs), " transactions found")
	return txs, nil
}

// implement interface service.Server
var _ service.Server = (*BlockChain)(nil)

// Run launch blockchain.
func (chain *BlockChain) Run() error {
	chain.subscribeMessageNotifiee()
	chain.proc.Go(chain.loop)

	return nil
}

// DB return chain db storage.
func (chain *BlockChain) DB() storage.Table {
	return chain.db
}

// Proc returns the goprocess of the BlockChain
func (chain *BlockChain) Proc() goprocess.Process {
	return chain.proc
}

// Bus returns the goprocess of the BlockChain
func (chain *BlockChain) Bus() eventbus.Bus {
	return chain.bus
}

// Stop the blockchain service
func (chain *BlockChain) Stop() {
	chain.proc.Close()
}

func (chain *BlockChain) subscribeMessageNotifiee() {
	chain.notifiee.Subscribe(p2p.NewNotifiee(p2p.NewBlockMsg, p2p.Unique, chain.newblockMsgCh))
}

func (chain *BlockChain) loop(p goprocess.Process) {
	logger.Info("Waitting for new block message...")
	metricsTicker := time.NewTicker(metricsLoopInterval)
	defer metricsTicker.Stop()
	for {
		select {
		case msg := <-chain.newblockMsgCh:
			chain.processBlockMsg(msg)
		case <-metricsTicker.C:
			metrics.MetricsCachedBlockMsgGauge.Update(int64(len(chain.newblockMsgCh)))
			metrics.MetricsBlockOrphanPoolSizeGauge.Update(int64(len(chain.hashToOrphanBlock)))
			metrics.MetricsLruCacheBlockGauge.Update(int64(chain.cache.Len()))
		case <-p.Closing():
			logger.Info("Quit blockchain loop.")
			return
		}
	}
}

var evilBehavior = []interface{}{core.ErrInvalidTime, core.ErrNoTransactions, core.ErrBlockTooBig, core.ErrFirstTxNotCoinbase, core.ErrMultipleCoinbases, core.ErrBadMerkleRoot, core.ErrDuplicateTx, core.ErrTooManySigOps, core.ErrBadFees, core.ErrBadCoinbaseValue, core.ErrUnfinalizedTx, core.ErrWrongBlockHeight}

func (chain *BlockChain) processBlockMsg(msg p2p.Message) error {
	block := new(types.Block)
	if err := block.Unmarshal(msg.Body()); err != nil {
		return err
	}

	// process block
	if _, _, err := chain.ProcessBlock(block, false, true); err != nil && util.InArray(err, evilBehavior) {
		chain.Bus().Publish(eventbus.TopicConnEvent, msg.From(), eventbus.BadBlockEvent)
		return err
	}
	chain.Bus().Publish(eventbus.TopicConnEvent, msg.From(), eventbus.NewBlockEvent)
	return nil
}

// ProcessBlock is used to handle new blocks.
func (chain *BlockChain) ProcessBlock(block *types.Block, broadcast bool, fastConfirm bool) (bool, bool, error) {
	blockHash := block.BlockHash()
	logger.Infof("Processing block hash: %s", blockHash.String())

	// The block must not already exist in the main chain or side chains.
	if exists := chain.blockExists(*blockHash); exists {
		logger.Warnf("already have block %v", blockHash)
		return false, false, core.ErrBlockExists
	}

	// The block must not already exist as an orphan.
	if chain.isInOrphanPool(blockHash) {
		logger.Warnf("already have block (orphan) %v", blockHash.String())
		return false, false, core.ErrOrphanBlockExists
	}

	if ok, err := chain.consensus.VerifyBlock(block); err != nil || !ok {
		logger.Errorf("Failed to verify block. Hash: %v, Height: %d, Err: %s", block.BlockHash().String(), block.Height, err.Error())
		return false, false, core.ErrFailedToVerifyWithConsensus
	}

	if err := validateBlock(block, util.NewMedianTime()); err != nil {
		logger.Error("Failed to validate block. Hash: %v, Height: %d, Err: %s", block.BlockHash(), block.Height, err.Error())
		return false, false, err
	}
	prevHash := block.Header.PrevBlockHash
	if prevHashExists := chain.blockExists(prevHash); !prevHashExists {

		// Orphan block.
		logger.Infof("Adding orphan block %v with parent %v", blockHash.String(), prevHash.String())
		chain.addOrphanBlock(block, *blockHash, prevHash)
		if chain.tail.Height < block.Height {
			// trigger sync
			chain.syncManager.StartSync()
		}

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

	logger.Infof("Accepted block hash: %v", blockHash.String())
	if broadcast {
		chain.notifiee.Broadcast(p2p.NewBlockMsg, block)
	}
	if chain.consensus.ValidateMiner() && fastConfirm {
		chain.consensus.BroadcastEternalMsgToMiners(block)
	}

	return isMainChain, false, nil
}

func (chain *BlockChain) blockExists(blockHash crypto.HashType) bool {
	if chain.cache.Contains(blockHash) {
		return true
	}
	if _, err := chain.LoadBlockByHash(blockHash); err != nil {
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
	if parentBlock == nil {
		return false, fmt.Errorf("parent block does not exist")
	}

	// The height of this block must be one more than the referenced parent block.
	if block.Height != parentBlock.Height+1 {
		logger.Errorf("Block %v's height is %d, but its parent's height is %d", blockHash.String(), block.Height, parentBlock.Height)
		return false, core.ErrWrongBlockHeight
	}

	chain.cache.Add(*blockHash, block)

	// Connect the passed block to the main or side chain.
	// There are 3 cases.
	parentHash := &block.Header.PrevBlockHash
	tailHash := chain.TailBlock().BlockHash()
	utxoSet := NewUtxoSet()
	if err := utxoSet.LoadBlockUtxos(block, chain.db); err != nil {
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
	logger.Infof("REORGANIZE: Block %v is causing a reorganization.", blockHash.String())
	if err := chain.reorganize(block, utxoSet); err != nil {
		return false, err
	}

	// Notify the caller that the new block was accepted into the block chain.
	// The caller would typically want to react by relaying the inventory to other peers.
	// TODO
	// chain.sendNotification(NTBlockAccepted, block)

	// This block is now the end of the best chain.
	if err := chain.SetTailBlock(block, utxoSet); err != nil {
		logger.Errorf("Failed to set tail block. Hash: %s, Height: %d, Err: %s", block.BlockHash().String(), block.Height, err.Error())
		return false, err
	}
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

func (chain *BlockChain) ancestor(block *types.Block, height uint32) *types.Block {
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
	// if block.BlockHash.IsEqual(GenesisHash) {
	// 	str := "the coinbase for the genesis block is not spendable"
	// 	return ErrMissingTxOut
	// }
	// Validate scripts here before utxoSet is updated; otherwise it may fail mistakenly
	if err := validateBlockScripts(utxoSet, block); err != nil {
		return err
	}

	transactions := block.Txs
	// Perform several checks on the inputs for each transaction.
	// Also accumulate the total fees.
	var totalFees uint64
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
	}

	// Ensure coinbase does not output more than block reward.
	var totalCoinbaseOutput uint64
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
	if err := chain.applyBlock(block, utxoSet); err != nil {
		return err
	}
	if err := chain.SetTailBlock(block, utxoSet); err != nil {
		logger.Errorf("Failed to set tail block. Hash: %s, Height: %d, Err: %s", block.BlockHash().String(), block.Height, err.Error())
		return err
	}
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

	if err := utxoSet.LoadBlockUtxos(block, chain.db); err != nil {
		return err
	}
	if err := utxoSet.RevertBlock(block); err != nil {
		return err
	}

	chain.db.Del(BlockKey(block.BlockHash()))

	return chain.notifyBlockConnectionUpdate(block, false)
}

func (chain *BlockChain) applyBlock(block *types.Block, utxoSet *UtxoSet) error {

	if err := utxoSet.LoadBlockUtxos(block, chain.db); err != nil {
		return err
	}
	if err := utxoSet.ApplyBlock(block); err != nil {
		return err
	}
	if err := chain.StoreBlockToDb(block); err != nil {
		return err
	}

	return chain.notifyBlockConnectionUpdate(block, true)
}

func (chain *BlockChain) notifyBlockConnectionUpdate(block *types.Block, connected bool) error {
	chain.bus.Publish(eventbus.TopicChainUpdate, &UpdateMsg{
		Connected: connected,
		Block:     block,
	})
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
		chain.filterHolder.ResetFilters(detachBlock.Height)
	}

	// Attach the blocks that form the new chain to the main chain starting at the
	// common ancenstor (the point where the chain forked).
	// From fork to tip, not including fork
	for blockIdx := len(attachBlocks) - 1; blockIdx >= 0; blockIdx-- {
		attachBlock := attachBlocks[blockIdx]
		if err := chain.applyBlock(attachBlock, utxoSet); err != nil {
			return err
		}
		chain.filterHolder.AddFilter(attachBlock.Height, *attachBlock.Hash, chain.DB(), func() bloom.Filter {
			return attachBlock.GetFilterForTransactionScript(utxoSet.utxoMap)
		})
	}

	metrics.MetricsBlockRevertMeter.Mark(1)
	return nil
}

// StoreTailBlock store tail block to db.
func (chain *BlockChain) StoreTailBlock(block *types.Block) error {
	data, err := block.Marshal()
	if err != nil {
		return err
	}
	return chain.db.Put(TailKey, data)
}

// TailBlock return chain tail block.
func (chain *BlockChain) TailBlock() *types.Block {
	return chain.tail
}

// SetEternal set block eternal status.
func (chain *BlockChain) SetEternal(block *types.Block) error {
	eternal := chain.eternal
	if eternal.Height < block.Height {
		if err := chain.StoreEternalBlock(block); err != nil {
			return err
		}
		chain.eternal = block
		return nil
	}
	return core.ErrFailedToSetEternal
}

// StoreEternalBlock store eternal block to db.
func (chain *BlockChain) StoreEternalBlock(block *types.Block) error {
	eternal, err := block.Marshal()
	if err != nil {
		return err
	}
	return chain.db.Put(EternalKey, eternal)
}

// EternalBlock return chain eternal block.
func (chain *BlockChain) EternalBlock() *types.Block {
	return chain.eternal
}

// ListAllUtxos list all the available utxos for testing purpose
func (chain *BlockChain) ListAllUtxos() (map[types.OutPoint]*types.UtxoWrap, error) {
	return make(map[types.OutPoint]*types.UtxoWrap), nil
}

// LoadUtxoByAddress list all the available utxos owned by an address
func (chain *BlockChain) LoadUtxoByAddress(addr types.Address) (map[types.OutPoint]*types.UtxoWrap, error) {
	payToPubKeyHashScript := *script.PayToPubKeyHashScript(addr.ScriptAddress())
	blockHashes := chain.filterHolder.ListMatchedBlockHashes(payToPubKeyHashScript)
	logger.Debug(addr.String(), " related blocks", util.PrettyPrint(blockHashes))
	utxos := make(map[types.OutPoint]*types.UtxoWrap)
	utxoSet := NewUtxoSet()
	for _, hash := range blockHashes {
		block, err := chain.LoadBlockByHash(hash)
		if err != nil {
			return nil, err
		}
		if err = utxoSet.ApplyBlockWithScriptFilter(block, payToPubKeyHashScript); err != nil {
			return nil, err
		}
	}
	for key, value := range utxoSet.utxoMap {
		if isPrefixed(value.Output.ScriptPubKey, payToPubKeyHashScript) && !value.IsSpent {
			logger.Info("utxo: ", util.PrettyPrint(value))
			utxos[key] = value
		}
	}
	return utxos, nil
}

// is s prefixed by prefix
func isPrefixed(s, prefix []byte) bool {
	prefixLen := len(prefix)
	if len(s) < prefixLen {
		return false
	}
	s = s[:prefixLen]
	return bytes.Equal(s, prefix)
}

// GetBlockHeight returns current height of main chain
func (chain *BlockChain) GetBlockHeight() uint32 {
	return chain.LongestChainHeight
}

// GetBlockHash finds the block in target height of main chain and returns it's hash
func (chain *BlockChain) GetBlockHash(blockHeight uint32) (*crypto.HashType, error) {
	block, err := chain.LoadBlockByHeight(blockHeight)
	if err != nil {
		return nil, err
	}
	return block.BlockHash(), nil
}

// SetTailBlock sets chain tail block.
func (chain *BlockChain) SetTailBlock(tail *types.Block, utxoSet *UtxoSet) error {

	if err := chain.filterHolder.AddFilter(tail.Height, *tail.BlockHash(), chain.DB(), func() bloom.Filter {
		return tail.GetFilterForTransactionScript(utxoSet.utxoMap)
	}); err != nil {
		return err
	}
	// save utxoset to database
	if err := utxoSet.WriteUtxoSetToDB(chain.db); err != nil {
		return err
	}

	// save current tail to database
	if err := chain.StoreTailBlock(tail); err != nil {
		return err
	}

	// save candidate context
	if err := chain.consensus.StoreCandidateContext(tail.BlockHash()); err != nil {
		return err
	}
	// save tx index
	if err := chain.WirteTxIndex(tail); err != nil {
		return err
	}
	chain.LongestChainHeight = tail.Height
	chain.tail = tail
	logger.Infof("Change New Tail. Hash: %s Height: %d", tail.BlockHash().String(), tail.Height)

	metrics.MetricsBlockHeightGauge.Update(int64(tail.Height))
	metrics.MetricsBlockTailHashGauge.Update(int64(util.HashBytes(tail.BlockHash().GetBytes())))
	return nil
}

func (chain *BlockChain) loadGenesis() (*types.Block, error) {
	if ok, _ := chain.db.Has(genesisBlockKey); ok {
		genesisBlockFromDb, err := chain.LoadBlockByHash(GenesisHash)
		if err != nil {
			return nil, err
		}
		return genesisBlockFromDb, nil
	}

	genesisBin, err := GenesisBlock.Marshal()
	if err != nil {
		return nil, err
	}
	chain.db.Put(genesisBlockKey, genesisBin)

	return &GenesisBlock, nil

}

func (chain *BlockChain) loadEternalBlock() (*types.Block, error) {
	if chain.eternal != nil {
		return chain.eternal, nil
	}
	if ok, _ := chain.db.Has(EternalKey); ok {
		eternalBin, err := chain.db.Get(EternalKey)
		if err != nil {
			return nil, err
		}

		eternal := new(types.Block)
		if err := eternal.Unmarshal(eternalBin); err != nil {
			return nil, err
		}

		return eternal, nil
	}
	return &GenesisBlock, nil
}

// LoadTailBlock load tail block
func (chain *BlockChain) LoadTailBlock() (*types.Block, error) {
	if chain.tail != nil {
		return chain.tail, nil
	}
	if ok, _ := chain.db.Has(TailKey); ok {
		tailBin, err := chain.db.Get(TailKey)
		if err != nil {
			return nil, err
		}

		tailBlock := new(types.Block)
		if err := tailBlock.Unmarshal(tailBin); err != nil {
			return nil, err
		}

		return tailBlock, nil
	}

	return &GenesisBlock, nil
}

// LoadBlockByHash load block by hash from db.
func (chain *BlockChain) LoadBlockByHash(hash crypto.HashType) (*types.Block, error) {

	blockBin, err := chain.db.Get(BlockKey(&hash))
	if err != nil {
		return nil, err
	}
	if blockBin == nil {
		return nil, core.ErrBlockIsNil
	}
	block := new(types.Block)
	if err := block.Unmarshal(blockBin); err != nil {
		return nil, err
	}

	return block, nil
}

// LoadBlockByHeight load block by height from db.
func (chain *BlockChain) LoadBlockByHeight(height uint32) (*types.Block, error) {
	if height == 0 {
		return chain.genesis, nil
	}
	bytes, err := chain.db.Get(BlockHashKey(height))
	if err != nil {
		return nil, err
	}
	if bytes == nil {
		return nil, core.ErrBlockIsNil
	}
	hash := new(crypto.HashType)
	copy(hash[:], bytes)
	block, err := chain.LoadBlockByHash(*hash)
	if err != nil {
		return nil, err
	}

	return block, nil
}

// StoreBlockToDb store block to db.
func (chain *BlockChain) StoreBlockToDb(block *types.Block) error {
	batch := chain.db.NewBatch()
	defer batch.Close()

	hash := block.BlockHash()
	batch.Put(BlockHashKey(block.Height), hash[:])

	data, err := block.Marshal()
	if err != nil {
		return err
	}
	batch.Put(BlockKey(hash), data)

	return batch.Write()
}

// LoadTxByHash load transaction with hash.
func (chain *BlockChain) LoadTxByHash(hash crypto.HashType) (*types.Transaction, error) {
	txIndex, err := chain.db.Get(TxIndexKey(&hash))
	if err != nil {
		return nil, err
	}
	height, idx, err := UnmarshalTxIndex(txIndex)
	if err != nil {
		return nil, err
	}

	block, err := chain.LoadBlockByHeight(height)
	if err != nil {
		return nil, err
	}

	tx := block.Txs[idx]
	target, err := tx.TxHash()
	if err != nil {
		return nil, err
	}
	if *target == hash {
		return tx, nil
	}
	logger.Errorf("Error reading tx hash, expect: %s got: %s", hash.String(), target.String())
	return nil, errors.New("Failed to load tx with hash")
}

// WirteTxIndex build tx index in block
func (chain *BlockChain) WirteTxIndex(block *types.Block) error {
	batch := chain.db.NewBatch()
	defer batch.Close()

	for idx, v := range block.Txs {
		tiBuf, err := MarshalTxIndex(block.Height, uint32(idx))
		if err != nil {
			return err
		}
		txHash, err := v.TxHash()
		if err != nil {
			return err
		}
		batch.Put(TxIndexKey(txHash), tiBuf)
	}

	return batch.Write()
}

// LocateForkPointAndFetchHeaders return block headers when get locate fork point request for sync service.
func (chain *BlockChain) LocateForkPointAndFetchHeaders(hashes []*crypto.HashType) ([]*crypto.HashType, error) {
	tailHeight := chain.tail.Height
	for index := range hashes {
		block, err := chain.LoadBlockByHash(*hashes[index])
		if err != nil {
			if err == core.ErrBlockIsNil {
				continue
			}
			return nil, err
		}

		result := []*crypto.HashType{}
		currentHeight := block.Height + 1
		if tailHeight-block.Height+1 < MaxBlocksPerSync {
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

		var idx uint32
		for idx < MaxBlocksPerSync {
			block, err := chain.LoadBlockByHeight(currentHeight + idx)
			if err != nil {
				return nil, err
			}
			result = append(result, block.BlockHash())
			idx++
		}
		return result, nil
	}
	return nil, nil
}

// CalcRootHashForNBlocks return root hash for N blocks.
func (chain *BlockChain) CalcRootHashForNBlocks(hash crypto.HashType, num uint32) (*crypto.HashType, error) {

	block, err := chain.LoadBlockByHash(hash)
	if err != nil {
		return nil, err
	}
	if chain.tail.Height-block.Height+1 < num {
		return nil, fmt.Errorf("Invalid params num[%d] (tailHeight[%d], "+
			"currentHeight[%d])", num, chain.tail.Height, block.Height)
	}
	var idx uint32
	hashes := make([]*crypto.HashType, num)
	for idx < num {
		block, err := chain.LoadBlockByHeight(block.Height + idx)
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
func (chain *BlockChain) FetchNBlockAfterSpecificHash(hash crypto.HashType, num uint32) ([]*types.Block, error) {
	block, err := chain.LoadBlockByHash(hash)
	if err != nil {
		return nil, err
	}
	if num <= 0 || chain.tail.Height-block.Height+1 < num {
		return nil, fmt.Errorf("Invalid params num[%d], tail.Height[%d],"+
			" block height[%d]", num, chain.tail.Height, block.Height)
	}
	var idx uint32
	blocks := make([]*types.Block, num)
	for idx < num {
		block, err := chain.LoadBlockByHeight(block.Height + idx)
		if err != nil {
			return nil, err
		}
		blocks[idx] = block
		idx++
	}
	return blocks, nil
}

// isInOrphanPool checks if block already exists in orphan pool
func (chain *BlockChain) isInOrphanPool(blockHash *crypto.HashType) bool {
	_, exists := chain.hashToOrphanBlock[*blockHash]
	return exists
}
