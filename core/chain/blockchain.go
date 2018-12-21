// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package chain

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"

	"golang.org/x/crypto/ripemd160"

	"github.com/BOXFoundation/boxd/boxd/eventbus"
	"github.com/BOXFoundation/boxd/boxd/service"
	"github.com/BOXFoundation/boxd/core"
	"github.com/BOXFoundation/boxd/core/metrics"
	"github.com/BOXFoundation/boxd/core/pb"
	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/crypto"
	"github.com/BOXFoundation/boxd/log"
	"github.com/BOXFoundation/boxd/p2p"
	"github.com/BOXFoundation/boxd/script"
	"github.com/BOXFoundation/boxd/storage"
	"github.com/BOXFoundation/boxd/storage/key"
	"github.com/BOXFoundation/boxd/util"
	"github.com/BOXFoundation/boxd/util/bloom"
	lru "github.com/hashicorp/golang-lru"
	"github.com/jbenet/goprocess"
	peer "github.com/libp2p/go-libp2p-peer"
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
	PeriodDuration       = 3600 * 24 * 100 / 5

	MaxBlocksPerSync = 1024

	metricsLoopInterval = 500 * time.Millisecond
	tokenIssueFilterKey = "token_issue"
	Threshold           = 32
)

var logger = log.NewLogger("chain") // logger

var _ service.ChainReader = (*BlockChain)(nil)

// BlockChain define chain struct
type BlockChain struct {
	notifiee                  p2p.Net
	newblockMsgCh             chan p2p.Message
	consensus                 types.Consensus
	db                        storage.Table
	batch                     storage.Batch
	genesis                   *types.Block
	tail                      *types.Block
	eternal                   *types.Block
	proc                      goprocess.Process
	LongestChainHeight        uint32
	cache                     *lru.Cache
	repeatedMintCache         *lru.Cache
	heightToBlock             *lru.Cache
	hashToSplitAddr           *lru.Cache
	bus                       eventbus.Bus
	orphanLock                sync.RWMutex
	chainLock                 sync.RWMutex
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
	b.repeatedMintCache, _ = lru.New(512)
	b.heightToBlock, _ = lru.New(512)
	b.hashToSplitAddr, _ = lru.New(512)

	if b.db, err = db.Table(BlockTableName); err != nil {
		return nil, err
	}

	if b.genesis, err = b.loadGenesis(); err != nil {
		logger.Error("Failed to load genesis block ", err)
		return nil, err
	}

	if b.eternal, err = b.LoadEternalBlock(); err != nil {
		logger.Error("Failed to load eternal block ", err)
		return nil, err
	}

	if b.tail, err = b.loadTailBlock(); err != nil {
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
	chain.notifiee.Subscribe(p2p.NewNotifiee(p2p.NewBlockMsg, chain.newblockMsgCh))
}

func (chain *BlockChain) loop(p goprocess.Process) {
	logger.Info("Waitting for new block message...")
	metricsTicker := time.NewTicker(metricsLoopInterval)
	defer metricsTicker.Stop()
	for {
		select {
		case msg := <-chain.newblockMsgCh:
			if err := chain.processBlockMsg(msg); err != nil {
				logger.Warnf("Failed to processBlockMsg. Err: %s", err.Error())
			}
		case <-metricsTicker.C:
			metrics.MetricsCachedBlockMsgGauge.Update(int64(len(chain.newblockMsgCh)))
			metrics.MetricsBlockOrphanPoolSizeGauge.Update(int64(len(chain.hashToOrphanBlock)))
			metrics.MetricsLruCacheBlockGauge.Update(int64(chain.cache.Len()))
			metrics.MetricsTailBlockTxsSizeGauge.Update(int64(len(chain.tail.Txs)))
		case <-p.Closing():
			logger.Info("Quit blockchain loop.")
			return
		}
	}
}

func (chain *BlockChain) verifyRepeatedMint(block *types.Block) bool {
	if exist, ok := chain.repeatedMintCache.Get(block.Header.TimeStamp); ok {
		if !block.BlockHash().IsEqual(exist.(*types.Block).BlockHash()) {
			return false
		}
	}
	return true
}

func (chain *BlockChain) processBlockMsg(msg p2p.Message) error {

	block := new(types.Block)
	if err := block.Unmarshal(msg.Body()); err != nil {
		return err
	}

	if err := VerifyBlockTimeOut(block); err != nil {
		return err
	}

	// process block
	if err := chain.ProcessBlock(block, p2p.RelayMode, true, msg.From()); err != nil && util.InArray(err, core.EvilBehavior) {
		chain.Bus().Publish(eventbus.TopicConnEvent, msg.From(), eventbus.BadBlockEvent)
		return err
	}
	chain.Bus().Publish(eventbus.TopicConnEvent, msg.From(), eventbus.NewBlockEvent)
	return nil
}

// ProcessBlock is used to handle new blocks.
func (chain *BlockChain) ProcessBlock(block *types.Block, transferMode p2p.TransferMode, fastConfirm bool, messageFrom peer.ID) error {

	if ok, err := chain.consensus.VerifySign(block); err != nil || !ok {
		logger.Errorf("Failed to verify block signature. Hash: %v, Height: %d, Err: %v", block.BlockHash().String(), block.Height, err)
		return core.ErrFailedToVerifyWithConsensus
	}

	chain.chainLock.Lock()
	defer chain.chainLock.Unlock()

	blockHash := block.BlockHash()
	logger.Infof("Prepare to process block. Hash: %s, Height: %d", blockHash.String(), block.Height)

	// The block must not already exist in the main chain or side chains.
	if exists := chain.verifyExists(*blockHash); exists {
		logger.Warnf("The block already exists. Hash: %s, Height: %d", blockHash.String(), block.Height)
		return core.ErrBlockExists
	}

	if ok := chain.verifyRepeatedMint(block); !ok {
		return core.ErrRepeatedMintAtSameTime
	}

	if err := validateBlock(block); err != nil {
		logger.Errorf("Failed to validate block. Hash: %v, Height: %d, Err: %s", block.BlockHash(), block.Height, err.Error())
		return err
	}
	prevHash := block.Header.PrevBlockHash
	if prevHashExists := chain.blockExists(prevHash); !prevHashExists {

		// Orphan block.
		logger.Infof("Adding orphan block %v with parent %v", blockHash.String(), prevHash.String())
		chain.addOrphanBlock(block, *blockHash, prevHash)
		chain.repeatedMintCache.Add(block.Header.TimeStamp, block)
		height := chain.tail.Height
		if height < block.Height && messageFrom != "" {
			if block.Height-height < Threshold {
				return chain.syncManager.ActiveLightSync(messageFrom)
			}
			// trigger sync
			chain.syncManager.StartSync()
		}
		return nil
	}

	// All context-free checks pass, try to accept the block into the chain.
	if err := chain.tryAcceptBlock(block); err != nil {
		logger.Errorf("Failed to accept the block into the main chain. Err: %s", err.Error())
		return err
	}

	if err := chain.processOrphans(block); err != nil {
		logger.Errorf("Failed to processOrphans. Err: %s", err.Error())
		return err
	}

	switch transferMode {
	case p2p.BroadcastMode:
		logger.Debugf("Broadcast New Block. Hash: %v Height: %d", blockHash.String(), block.Height)
		go chain.notifiee.Broadcast(p2p.NewBlockMsg, block)
	case p2p.RelayMode:
		logger.Debugf("Relay New Block. Hash: %v Height: %d", blockHash.String(), block.Height)
		go chain.notifiee.Relay(p2p.NewBlockMsg, block)
	default:
	}
	if chain.consensus.ValidateMiner() && fastConfirm {
		go chain.consensus.BroadcastEternalMsgToMiners(block)
		go chain.consensus.TryToUpdateEternalBlock(block)
	}

	logger.Infof("Accepted New Block. Hash: %v Height: %d TxsNum: %d", blockHash.String(), block.Height, len(block.Txs))
	return nil
}

func (chain *BlockChain) verifyExists(blockHash crypto.HashType) bool {
	return chain.blockExists(blockHash) || chain.isInOrphanPool(blockHash)
}

func (chain *BlockChain) blockExists(blockHash crypto.HashType) bool {
	if chain.cache.Contains(blockHash) {
		return true
	}
	if block, _ := chain.LoadBlockByHash(blockHash); block != nil {
		return true
	}
	return false
}

// isInOrphanPool checks if block already exists in orphan pool
func (chain *BlockChain) isInOrphanPool(blockHash crypto.HashType) bool {
	_, exists := chain.hashToOrphanBlock[blockHash]
	return exists
}

// tryAcceptBlock validates block within the chain context and see if it can be accepted.
// Return whether it is on the main chain or not.
func (chain *BlockChain) tryAcceptBlock(block *types.Block) error {
	blockHash := block.BlockHash()
	// must not be orphan if reaching here
	parentBlock := chain.getParentBlock(block)
	if parentBlock == nil {
		return core.ErrParentBlockNotExist
	}

	// verify miner epoch
	// if err := chain.consensus.VerifyMinerEpoch(block); err != nil {
	// 	logger.Errorf("Failed to verify miner epoch. Hash: %v, Height: %d, Err: %v", block.BlockHash().String(), block.Height, err)
	// 	return core.ErrFailedToVerifyWithConsensus
	// }

	// The height of this block must be one more than the referenced parent block.
	if block.Height != parentBlock.Height+1 {
		logger.Errorf("Block %v's height is %d, but its parent's height is %d", blockHash.String(), block.Height, parentBlock.Height)
		return core.ErrWrongBlockHeight
	}

	chain.cache.Add(*blockHash, block)

	// Connect the passed block to the main or side chain.
	// There are 3 cases.
	parentHash := &block.Header.PrevBlockHash
	tailHash := chain.TailBlock().BlockHash()

	batch := chain.db.NewBatch()
	defer batch.Close()

	// Case 1): The new block extends the main chain.
	// We expect this to be the most common case.
	if parentHash.IsEqual(tailHash) {
		return chain.tryConnectBlockToMainChain(block, batch)
	}

	// Case 2): The block extends or creats a side chain, which is not longer than the main chain.
	if block.Height <= chain.LongestChainHeight {
		logger.Infof("Block %v extends a side chain to height %d without causing reorg, main chain height %d",
			blockHash, block.Height, chain.LongestChainHeight)
		return nil
	}

	// Case 3): Extended side chain is longer than the main chain and becomes the new main chain.
	logger.Infof("REORGANIZE: Block %v is causing a reorganization.", blockHash.String())
	if err := chain.reorganize(block, batch); err != nil {
		return err
	}

	// This block is now the end of the best chain.
	if err := chain.SetTailBlock(block, batch); err != nil {
		logger.Errorf("Failed to set tail block. Hash: %s, Height: %d, Err: %s", block.BlockHash().String(), block.Height, err.Error())
		return err
	}

	return batch.Write()
}

func (chain *BlockChain) addOrphanBlock(orphan *types.Block, orphanHash crypto.HashType, parentHash crypto.HashType) {
	chain.hashToOrphanBlock[orphanHash] = orphan
	// Add to parent hash map lookup index for faster dependency lookups.
	chain.orphanBlockHashToChildren[parentHash] = append(chain.orphanBlockHashToChildren[parentHash], orphan)
}

func (chain *BlockChain) processOrphans(block *types.Block) error {

	// Start with processing at least the passed block.
	acceptedBlocks := []*types.Block{block}

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
			if err := chain.tryAcceptBlock(orphan); err != nil {
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
	if block.Header.PrevBlockHash.IsEqual(chain.genesis.BlockHash()) {
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

// tryConnectBlockToMainChain tries to append the passed block to the main chain.
// It enforces multiple rules such as double spends and script verification.
func (chain *BlockChain) tryConnectBlockToMainChain(block *types.Block, batch storage.Batch) error {

	logger.Debugf("Try to connect block to main chain. Hash: %s, Height: %d", block.BlockHash().String(), block.Height)
	utxoSet := NewUtxoSet()
	if err := utxoSet.LoadBlockUtxos(block, chain.db); err != nil {
		return err
	}

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

	if err := chain.applyBlock(block, utxoSet, batch); err != nil {
		return err
	}
	if err := chain.SetTailBlock(block, batch); err != nil {
		logger.Errorf("Failed to set tail block. Hash: %s, Height: %d, Err: %s", block.BlockHash().String(), block.Height, err.Error())
		return err
	}

	return batch.Write()
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

func (chain *BlockChain) revertBlock(block *types.Block, batch storage.Batch) error {

	utxoSet := NewUtxoSet()
	if err := utxoSet.LoadBlockUtxos(block, chain.db); err != nil {
		return err
	}
	if err := utxoSet.RevertBlock(block, chain); err != nil {
		return err
	}
	// save utxoset to database
	if err := utxoSet.WriteUtxoSetToDB(batch); err != nil {
		return err
	}

	batch.Del(BlockKey(block.BlockHash()))
	batch.Del(BlockHashKey(block.Height))

	chain.filterHolder.ResetFilters(block.Height)

	// save tx index
	if err := chain.DelTxIndex(block, batch); err != nil {
		return err
	}

	return chain.notifyBlockConnectionUpdate(block, false)
}

func (chain *BlockChain) applyBlock(block *types.Block, utxoSet *UtxoSet, batch storage.Batch) error {

	// Save a deep copy before we potentially split the block's txs' outputs and mutate it
	blockCopy := block.Copy()

	// Split tx outputs if any
	chain.splitBlockOutputs(blockCopy)

	if utxoSet == nil {
		utxoSet = NewUtxoSet()
		if err := utxoSet.LoadBlockUtxos(blockCopy, chain.db); err != nil {
			return err
		}
	}

	if err := utxoSet.ApplyBlock(blockCopy); err != nil {
		return err
	}
	// save utxoset to database
	if err := utxoSet.WriteUtxoSetToDB(batch); err != nil {
		return err
	}

	if err := chain.StoreBlockToDb(block, batch); err != nil {
		return err
	}

	if err := chain.filterHolder.AddFilter(block.Height, *block.BlockHash(), chain.DB(), batch, func() bloom.Filter {
		return GetFilterForTransactionScript(blockCopy, utxoSet.utxoMap)
	}); err != nil {
		return err
	}

	// save candidate context
	if err := chain.consensus.StoreCandidateContext(block.BlockHash(), batch); err != nil {
		return err
	}

	// save tx index
	if err := chain.WriteTxIndex(block, batch); err != nil {
		return err
	}

	// store split addr index
	if err := chain.WriteSplitAddrIndex(block, batch); err != nil {
		logger.Error(err)
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

func (chain *BlockChain) reorganize(block *types.Block, batch storage.Batch) error {
	// Find the common ancestor of the main chain and side chain
	_, detachBlocks, attachBlocks := chain.findFork(block)

	// Detach the blocks that form the (now) old fork from the main chain.
	// From tail to fork, not including fork
	for _, detachBlock := range detachBlocks {
		if err := chain.revertBlock(detachBlock, batch); err != nil {
			return err
		}
	}

	// Attach the blocks that form the new chain to the main chain starting at the
	// common ancenstor (the point where the chain forked).
	// From fork to tail, not including fork
	for blockIdx := len(attachBlocks) - 1; blockIdx >= 0; blockIdx-- {
		attachBlock := attachBlocks[blockIdx]
		if err := chain.applyBlock(attachBlock, nil, batch); err != nil {
			return err
		}
	}

	metrics.MetricsBlockRevertMeter.Mark(1)
	return nil
}

// StoreTailBlock store tail block to db.
func (chain *BlockChain) StoreTailBlock(block *types.Block, batch storage.Batch) error {
	data, err := block.Marshal()
	if err != nil {
		return err
	}
	batch.Put(TailKey, data)
	return nil
}

// TailBlock return chain tail block.
func (chain *BlockChain) TailBlock() *types.Block {
	return chain.tail
}

// Genesis return chain tail block.
func (chain *BlockChain) Genesis() *types.Block {
	return chain.genesis
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
	result := make(map[types.OutPoint]*types.UtxoWrap)
	keyBytes := chain.db.KeysWithPrefix(utxoBase.Bytes())
	for _, k := range keyBytes {
		key := key.NewKeyFromBytes(k)
		if len(key.List()) != 3 {
			return nil, fmt.Errorf("invalid utxo key")
		}
		serialized, err := chain.db.Get(k)
		if err != nil || serialized == nil {
			return nil, err
		}
		wrap := new(types.UtxoWrap)
		if err := wrap.Unmarshal(serialized); err != nil {
			return nil, err
		}
		hash := &crypto.HashType{}
		if err := hash.SetString(key.List()[1]); err != nil {
			return nil, err
		}
		index, err := strconv.ParseUint(key.List()[2], 16, 32)
		if err != nil {
			return nil, fmt.Errorf("invalid utxo key")
		}
		result[types.OutPoint{
			Hash:  *hash,
			Index: uint32(index),
		}] = wrap
	}
	return result, nil
}

// LoadUtxoByAddress list all the available utxos owned by an address, including token utxos
func (chain *BlockChain) LoadUtxoByAddress(addr types.Address) (map[types.OutPoint]*types.UtxoWrap, error) {
	payToPubKeyHashScript := *script.PayToPubKeyHashScript(addr.Hash())
	blockHashes := chain.filterHolder.ListMatchedBlockHashes(payToPubKeyHashScript)
	utxos := make(map[types.OutPoint]*types.UtxoWrap)
	utxoSet := NewUtxoSet()
	for _, hash := range blockHashes {
		block, err := chain.LoadBlockByHash(hash)
		if block != nil {
			// Split tx outputs if any
			chain.splitBlockOutputs(block)

			if err = utxoSet.ApplyBlockWithScriptFilter(block, payToPubKeyHashScript); err != nil {
				return nil, err
			}
		}

	}
	for key, value := range utxoSet.utxoMap {
		if util.IsPrefixed(value.Output.ScriptPubKey, payToPubKeyHashScript) && !value.IsSpent {
			utxos[key] = value
		}
	}
	return utxos, nil
}

// LoadSpentUtxos loads UtxoWrap info of input outpoints
func (chain *BlockChain) LoadSpentUtxos(outpoints []types.OutPoint) (map[types.OutPoint]*types.UtxoWrap, error) {
	var relatedTxHashes = make(map[crypto.HashType]bool)
	for _, op := range outpoints {
		relatedTxHashes[op.Hash] = true
	}
	var relatedTxs = make(map[crypto.HashType]*types.Transaction)
	for h := range relatedTxHashes {
		tx, err := chain.LoadTxByHash(h)
		if err != nil {
			return nil, err
		}
		relatedTxs[h] = tx
	}

	utxos := make(map[types.OutPoint]*types.UtxoWrap)
	for _, op := range outpoints {
		tx, ok := relatedTxs[op.Hash]
		if !ok || tx == nil || len(tx.Vout) <= int(op.Index) {
			return nil, fmt.Errorf("fail to find transaction: %v", op.Hash)
		}
		txOut := tx.Vout[op.Index]
		utxos[op] = &types.UtxoWrap{
			Output:      txOut,
			BlockHeight: 0, // Warning: BlockHeight unfilled
			IsCoinBase:  IsCoinBase(tx),
			IsSpent:     true,
			IsModified:  false,
		}
	}

	return utxos, nil
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
func (chain *BlockChain) SetTailBlock(tail *types.Block, batch storage.Batch) error {

	// save current tail to database
	if err := chain.StoreTailBlock(tail, batch); err != nil {
		return err
	}

	chain.repeatedMintCache.Add(tail.Header.TimeStamp, tail)
	// chain.heightToBlock.Add(tail.Height, tail)
	chain.LongestChainHeight = tail.Height
	chain.tail = tail
	logger.Infof("Change New Tail. Hash: %s Height: %d", tail.BlockHash().String(), tail.Height)

	metrics.MetricsBlockHeightGauge.Update(int64(tail.Height))
	metrics.MetricsBlockTailHashGauge.Update(int64(util.HashBytes(tail.BlockHash().GetBytes())))
	return nil
}

func (chain *BlockChain) loadGenesis() (*types.Block, error) {

	if ok, _ := chain.db.Has(GenesisKey); ok {
		genesisBin, err := chain.db.Get(GenesisKey)
		if err != nil {
			return nil, err
		}
		genesis := new(types.Block)
		if err := genesis.Unmarshal(genesisBin); err != nil {
			return nil, err
		}

		return genesis, nil
	}

	genesis := GenesisBlock
	genesisTxs, err := TokenPreAllocation()
	if err != nil {
		return nil, err
	}
	genesis.Txs = genesisTxs
	genesis.Header.TxsRoot = *CalcTxsHash(genesisTxs)

	genesisBin, err := genesis.Marshal()
	if err != nil {
		return nil, err
	}
	batch := chain.db.NewBatch()
	utxoSet := NewUtxoSet()
	for _, v := range genesis.Txs {
		for idx := range v.Vout {
			utxoSet.AddUtxo(v, uint32(idx), genesis.Height)
		}
	}
	utxoSet.WriteUtxoSetToDB(batch)
	batch.Put(BlockKey(genesis.BlockHash()), genesisBin)
	if err := batch.Write(); err != nil {
		return nil, err
	}
	logger.Errorf("genesis hash : %v", genesis.BlockHash().String())
	return &genesis, nil

}

// LoadEternalBlock returns the current highest eternal block
func (chain *BlockChain) LoadEternalBlock() (*types.Block, error) {
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
	return chain.genesis, nil
}

// loadTailBlock load tail block
func (chain *BlockChain) loadTailBlock() (*types.Block, error) {
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

	return chain.genesis, nil
}

// IsCoinBase checks if an transaction is coinbase transaction
func (chain *BlockChain) IsCoinBase(tx *types.Transaction) bool {
	return IsCoinBase(tx)
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
	// if block, ok := chain.heightToBlock.Get(height); ok {
	// 	return block.(*types.Block), nil
	// }

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
func (chain *BlockChain) StoreBlockToDb(block *types.Block, batch storage.Batch) error {

	hash := block.BlockHash()
	batch.Put(BlockHashKey(block.Height), hash[:])

	data, err := block.Marshal()
	if err != nil {
		return err
	}
	batch.Put(BlockKey(hash), data)
	return nil
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

// LoadBlockInfoByTxHash returns block and txIndex of transaction with the input param hash
func (chain *BlockChain) LoadBlockInfoByTxHash(hash crypto.HashType) (*types.Block, uint32, error) {
	txIndex, err := chain.db.Get(TxIndexKey(&hash))
	if err != nil {
		return nil, 0, err
	}
	height, idx, err := UnmarshalTxIndex(txIndex)
	if err != nil {
		return nil, 0, err
	}
	block, err := chain.LoadBlockByHeight(height)
	if err != nil {
		return nil, 0, err
	}

	tx := block.Txs[idx]
	target, err := tx.TxHash()
	if err != nil {
		return nil, 0, err
	}
	if *target == hash {
		return block, idx, nil
	}
	logger.Errorf("Error reading tx hash, expect: %s got: %s", hash.String(), target.String())
	return nil, 0, errors.New("failed to load tx with hash")
}

// WriteTxIndex builds tx index in block
func (chain *BlockChain) WriteTxIndex(block *types.Block, batch storage.Batch) error {

	for idx, tx := range block.Txs {
		tiBuf, err := MarshalTxIndex(block.Height, uint32(idx))
		if err != nil {
			return err
		}
		txHash, err := tx.TxHash()
		if err != nil {
			return err
		}
		batch.Put(TxIndexKey(txHash), tiBuf)
	}
	return nil
}

// DelTxIndex deletes tx index in block
func (chain *BlockChain) DelTxIndex(block *types.Block, batch storage.Batch) error {

	for _, tx := range block.Txs {
		txHash, err := tx.TxHash()
		if err != nil {
			return err
		}
		batch.Del(TxIndexKey(txHash))
	}

	return nil
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

// GetFilterForTransactionScript returns the bloom filter for all the script address
// of the transactions in the block, it will use the pre-calculated filter if there
// is any
func GetFilterForTransactionScript(block *types.Block, utxoUsed map[types.OutPoint]*types.UtxoWrap) bloom.Filter {
	var vin, vout [][]byte
	for _, tx := range block.Txs {
		for _, out := range tx.Vout {
			vout = append(vout, out.ScriptPubKey)
		}
	}
	for _, utxo := range utxoUsed {
		if utxo != nil && utxo.Output != nil {
			// TODO: add script index for previous output
			vin = append(vin, utxo.Output.ScriptPubKey)
		}
	}
	filter := bloom.NewFilter(uint32(len(vin)+len(vout)+1), 0.0001)
	for _, scriptBytes := range vin {
		filter.Add(scriptBytes)
	}
	for _, tx := range block.Txs {
		for idx, out := range tx.Vout {
			indexedBytes := out.ScriptPubKey
			sc := script.NewScriptFromBytes(out.ScriptPubKey)
			if sc.IsTokenIssue() || sc.IsTokenTransfer() {
				// token output: only store the p2pkh prefix part so we can retrieve it later
				indexedBytes = *sc.P2PKHScriptPrefix()
			} else if sc.IsSplitAddrScript() {
				// split address output: only store up to the hashed address part so we can retrieve it later
				indexedBytes = *sc.GetSplitAddrScriptPrefix()
			}
			filter.Add(indexedBytes)
			hash, _ := tx.TxHash()
			if sc.IsTokenIssue() {
				filter.Add([]byte(tokenIssueFilterKey))
				tokenID := &script.TokenID{
					OutPoint: types.OutPoint{
						Hash:  *hash,
						Index: uint32(idx),
					},
				}
				filter.Add([]byte(tokenID.String()))
			} else if sc.IsTokenTransfer() {
				param, _ := sc.GetTransferParams()
				filter.Add([]byte(param.TokenID.String()))
			}
		}
	}
	logger.Debugf("Create Block filter with %d inputs and %d outputs", len(vin), len(vout))
	return filter
}

func (chain *BlockChain) loadFilters() error {
	var i uint32 = 1
	var utxoSet *UtxoSet
	batch := chain.db.NewBatch()
	defer batch.Close()
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
		if err := chain.filterHolder.AddFilter(i, *block.Hash, chain.DB(), batch, func() bloom.Filter {
			return GetFilterForTransactionScript(block, utxoSet.utxoMap)
		}); err != nil {
			logger.Error("Failed to addFilter", err)
			return err
		}
	}
	utxoSet = nil
	return batch.Write()
}

// GetTransactionsByAddr search the main chain about transaction relate to give address
func (chain *BlockChain) GetTransactionsByAddr(addr types.Address) ([]*types.Transaction, error) {
	payToPubKeyHashScript := *script.PayToPubKeyHashScript(addr.Hash())
	hashes := chain.filterHolder.ListMatchedBlockHashes(payToPubKeyHashScript)
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
	utxoSet = nil
	return txs, nil
}

// ListTokenIssueTransactions returns transactions which contains token issue info
func (chain *BlockChain) ListTokenIssueTransactions() ([]*types.Transaction, []*types.BlockHeader, error) {
	hashes := chain.filterHolder.ListMatchedBlockHashes([]byte(tokenIssueFilterKey))
	logger.Infof("%v blocks related to token issue", len(hashes))
	var txs []*types.Transaction
	var blockHeaders []*types.BlockHeader
	for _, hash := range hashes {
		block, err := chain.LoadBlockByHash(hash)
		if err != nil {
			return nil, nil, err
		}
		for _, tx := range block.Txs {
			for _, vout := range tx.Vout {
				sc := *script.NewScriptFromBytes(vout.ScriptPubKey)
				if sc.IsTokenIssue() {
					txs = append(txs, tx)
					blockHeaders = append(blockHeaders, block.Header)
				}
			}
		}
	}
	return txs, blockHeaders, nil
}

// GetTokenTransactions returns transactions history of a tokenID
func (chain *BlockChain) GetTokenTransactions(tokenID *script.TokenID) ([]*types.Transaction, error) {
	hashes := chain.filterHolder.ListMatchedBlockHashes([]byte(tokenID.String()))
	logger.Infof("%v blocks related to token %v", len(hashes), tokenID)
	var txs []*types.Transaction
	for _, hash := range hashes {
		block, err := chain.LoadBlockByHash(hash)
		if err != nil {
			return nil, err
		}
		for _, tx := range block.Txs {
			hash, _ := tx.TxHash()
			for idx, vout := range tx.Vout {
				sc := *script.NewScriptFromBytes(vout.ScriptPubKey)
				if sc.IsTokenIssue() && hash.IsEqual(&tokenID.Hash) && uint32(idx) == tokenID.Index {
					txs = append(txs, tx)
					break
				}
				if sc.IsTokenTransfer() {
					params, _ := sc.GetTransferParams()
					if params.Hash.IsEqual(&tokenID.Hash) && params.Index == tokenID.Index {
						txs = append(txs, tx)
						break
					}
				}
			}
		}
	}
	return txs, nil
}

// split outputs of txs in the block where applicable
func (chain *BlockChain) splitBlockOutputs(block *types.Block) {
	for _, tx := range block.Txs {
		chain.splitTxOutputs(tx)
	}
}

// split outputs in the tx where applicable
func (chain *BlockChain) splitTxOutputs(tx *types.Transaction) {
	vout := make([]*corepb.TxOut, 0)
	for _, txOut := range tx.Vout {
		txOuts := chain.splitTxOutput(txOut)
		vout = append(vout, txOuts...)
	}
	tx.Vout = vout
}

// split an output to a split address into  multiple outputs to composite addresses
func (chain *BlockChain) splitTxOutput(txOut *corepb.TxOut) []*corepb.TxOut {
	// return the output itself if it cannot be split
	txOuts := []*corepb.TxOut{txOut}
	addr, err := script.NewScriptFromBytes(txOut.ScriptPubKey).ExtractAddress()
	if err != nil {
		logger.Debugf("Tx output does not contain a valid address")
		return txOuts
	}
	isSplitAddr, sai, err := chain.findSplitAddr(addr)
	if !isSplitAddr {
		return txOuts
	}
	if err != nil {
		logger.Errorf("Split address %v parse error: %v", addr, err)
		return txOuts
	}

	// split it
	txOuts = make([]*corepb.TxOut, 0)
	n := len(sai.addrs)

	totalWeight := uint64(0)
	for i := 0; i < n; i++ {
		totalWeight += sai.weights[i]
	}

	totalValue := uint64(0)
	for i := 0; i < n; i++ {
		// An composite address splits value per its weight
		value := txOut.Value * sai.weights[i] / totalWeight
		if i == n-1 {
			// Last address gets the remainder value in case value is indivisible
			value = txOut.Value - totalValue
		} else {
			totalValue += value
		}
		childTxOut := &corepb.TxOut{
			Value:        value,
			ScriptPubKey: *script.PayToPubKeyHashScript(sai.addrs[i].Hash()),
		}
		// recursively find if the child tx output is splittable
		childTxOuts := chain.splitTxOutput(childTxOut)
		txOuts = append(txOuts, childTxOuts...)
	}

	return txOuts
}

type splitAddrInfo struct {
	addrs   []types.Address
	weights []uint64
}

// Marshall Serialize splitAddrInfo into bytes
func (s *splitAddrInfo) Marshall() ([]byte, error) {
	if len(s.addrs) != len(s.weights) {
		return nil, fmt.Errorf("invalid split addr info")
	}
	res := make([]byte, 0, len(s.addrs)*(ripemd160.Size+8))
	for i := 0; i < len(s.addrs); i++ {
		res = append(res, s.addrs[i].Hash()...)
		weightByte := make([]byte, 8)
		binary.BigEndian.PutUint64(weightByte, s.weights[i])
		res = append(res, weightByte...)
	}
	return res, nil
}

// Unmarshall parse splitAddrInfo from bytes
func (s *splitAddrInfo) Unmarshall(data []byte) error {
	minLenght := ripemd160.Size + 8
	if len(data)%minLenght != 0 {
		return fmt.Errorf("invalid byte length")
	}
	count := len(data) / minLenght
	addrs := make([]types.Address, 0, count)
	weights := make([]uint64, 0, count)
	for i := 0; i < count; i++ {
		offset := i * minLenght
		addr, err := types.NewAddressPubKeyHash(data[offset : offset+ripemd160.Size])
		if err != nil {
			return err
		}
		weight := binary.BigEndian.Uint64(data[offset+ripemd160.Size : offset+minLenght])
		addrs = append(addrs, addr)
		weights = append(weights, weight)
	}
	s.addrs = addrs
	s.weights = weights
	return nil
}

// findSplitAddr search the main chain to see if the address is a split address.
// If yes, return split address parameters
func (chain *BlockChain) findSplitAddr(addr types.Address) (bool, *splitAddrInfo, error) {
	if splitInfo, ok := chain.hashToSplitAddr.Get(addr.Hash160()); ok {
		return ok, splitInfo.(*splitAddrInfo), nil
	}
	data, err := chain.db.Get(SplitAddrKey(addr.Hash()))
	if err != nil {
		return false, nil, err
	}
	if data == nil {
		return false, nil, nil
	}
	info := new(splitAddrInfo)
	if err := info.Unmarshall(data); err != nil {
		return false, nil, err
	}
	chain.hashToSplitAddr.Add(addr.Hash160(), info)
	return true, info, nil
}

// WriteSplitAddrIndex writes split addr info index
func (chain *BlockChain) WriteSplitAddrIndex(block *types.Block, batch storage.Batch) error {
	for _, tx := range block.Txs {
		for _, vout := range tx.Vout {
			sc := *script.NewScriptFromBytes(vout.ScriptPubKey)
			if sc.IsSplitAddrScript() {
				addr, err := sc.ExtractAddress()
				if err != nil {
					return err
				}
				addrs, weights, err := sc.ParseSplitAddrScript()
				if err != nil {
					return err
				}
				sai := &splitAddrInfo{
					addrs:   addrs,
					weights: weights,
				}
				dataBytes, err := sai.Marshall()
				if err != nil {
					return err
				}
				k := SplitAddrKey(addr.Hash())
				batch.Put(k, dataBytes)
				chain.hashToSplitAddr.Add(addr.Hash160(), sai)
				logger.Infof("New Split Address created")
			}
		}
	}
	return nil
}
