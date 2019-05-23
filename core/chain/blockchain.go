// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package chain

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/BOXFoundation/boxd/boxd/eventbus"
	"github.com/BOXFoundation/boxd/boxd/service"
	"github.com/BOXFoundation/boxd/core"
	"github.com/BOXFoundation/boxd/core/metrics"
	corepb "github.com/BOXFoundation/boxd/core/pb"
	"github.com/BOXFoundation/boxd/core/types"
	state "github.com/BOXFoundation/boxd/core/worldstate"
	"github.com/BOXFoundation/boxd/crypto"
	"github.com/BOXFoundation/boxd/log"
	"github.com/BOXFoundation/boxd/p2p"
	"github.com/BOXFoundation/boxd/script"
	"github.com/BOXFoundation/boxd/storage"
	"github.com/BOXFoundation/boxd/util"
	"github.com/BOXFoundation/boxd/util/bloom"
	"github.com/BOXFoundation/boxd/vm"
	"github.com/BOXFoundation/boxd/vm/common/math"
	lru "github.com/hashicorp/golang-lru"
	"github.com/jbenet/goprocess"
	peer "github.com/libp2p/go-libp2p-peer"
	"golang.org/x/crypto/ripemd160"
)

// const defines constants
const (
	BlockMsgChBufferSize        = 1024
	EternalBlockMsgChBufferSize = 65536

	MaxTimeOffsetSeconds = 2 * 60 * 60
	MaxBlockSize         = 32000000
	CoinbaseLib          = 100
	maxBlockSigOpCnt     = 80000
	PeriodDuration       = 21 * 5 * 10000

	MaxBlocksPerSync = 1024

	metricsLoopInterval = 500 * time.Millisecond
	tokenIssueFilterKey = "token_issue"
	Threshold           = 32
)

const (
	free int32 = iota
	busy
)

var logger = log.NewLogger("chain") // logger

var _ service.ChainReader = (*BlockChain)(nil)

// BlockChain define chain struct
type BlockChain struct {
	notifiee                  p2p.Net
	newblockMsgCh             chan p2p.Message
	consensus                 Consensus
	db                        storage.Table
	batch                     storage.Batch
	genesis                   *types.Block
	tail                      *types.Block
	eternal                   *types.Block
	proc                      goprocess.Process
	LongestChainHeight        uint32
	blockcache                *lru.Cache
	repeatedMintCache         *lru.Cache
	heightToBlock             *lru.Cache
	splitAddrFilter           bloom.Filter
	bus                       eventbus.Bus
	chainLock                 sync.RWMutex
	hashToOrphanBlock         map[crypto.HashType]*types.Block
	orphanBlockHashToChildren map[crypto.HashType][]*types.Block
	syncManager               SyncManager
	status                    int32
	stateProcessor            *StateProcessor
	vmConfig                  vm.Config
	stateDBCache              map[uint32]*state.StateDB
	utxoSetCache              map[uint32]*UtxoSet
	receiptsCache             map[uint32]types.Receipts
}

// UpdateMsg sent from blockchain to, e.g., mempool
type UpdateMsg struct {
	// block connected/disconnected from main chain
	AttachBlocks []*types.Block
	DetachBlocks []*types.Block
}

// NewBlockChain return a blockchain.
func NewBlockChain(parent goprocess.Process, notifiee p2p.Net, db storage.Storage, bus eventbus.Bus) (*BlockChain, error) {

	b := &BlockChain{
		notifiee:                  notifiee,
		newblockMsgCh:             make(chan p2p.Message, BlockMsgChBufferSize),
		proc:                      goprocess.WithParent(parent),
		hashToOrphanBlock:         make(map[crypto.HashType]*types.Block),
		orphanBlockHashToChildren: make(map[crypto.HashType][]*types.Block),
		stateDBCache:              make(map[uint32]*state.StateDB),
		utxoSetCache:              make(map[uint32]*UtxoSet),
		receiptsCache:             make(map[uint32]types.Receipts),
		bus:                       eventbus.Default(),
		status:                    free,
	}

	var err error
	b.blockcache, _ = lru.New(512)
	b.repeatedMintCache, _ = lru.New(512)
	b.heightToBlock, _ = lru.New(512)
	b.splitAddrFilter = bloom.NewFilter(bloom.MaxFilterSize, 0.0001)

	stateProcessor := NewStateProcessor(b)
	b.stateProcessor = stateProcessor

	if b.db, err = db.Table(BlockTableName); err != nil {
		return nil, err
	}

	if b.genesis, err = b.loadGenesis(); err != nil {
		logger.Error("Failed to load genesis block ", err)
		return nil, err
	}
	logger.Infof("load genesis block: %s", b.genesis.BlockHash())

	if b.eternal, err = b.LoadEternalBlock(); err != nil {
		logger.Error("Failed to load eternal block ", err)
		return nil, err
	}
	logger.Infof("load eternal block: %s, height: %d", b.eternal.BlockHash(),
		b.eternal.Header.Height)

	if b.tail, err = b.loadTailBlock(); err != nil {
		logger.Error("Failed to load tail block ", err)
		return nil, err
	}
	logger.Infof("load tail block: %s, height: %d", b.tail.BlockHash(),
		b.tail.Header.Height)
	b.LongestChainHeight = b.tail.Header.Height

	return b, nil
}

// IsBusy return if the chain is processing a block
func (chain *BlockChain) IsBusy() bool {
	v := atomic.LoadInt32(&chain.status)
	return v == busy
}

// Setup prepare blockchain.
func (chain *BlockChain) Setup(consensus Consensus, syncManager SyncManager) {
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

// Consensus return chain consensus.
func (chain *BlockChain) Consensus() Consensus {
	return chain.consensus
}

// DB return chain db storage.
func (chain *BlockChain) DB() storage.Table {
	return chain.db
}

// StateProcessor returns chain stateProcessor.
func (chain *BlockChain) StateProcessor() *StateProcessor {
	return chain.stateProcessor
}

// StateDBCache returns chain stateDB cache.
func (chain *BlockChain) StateDBCache() map[uint32]*state.StateDB {
	return chain.stateDBCache
}

// UtxoSetCache returns chain utxoSet cache.
func (chain *BlockChain) UtxoSetCache() map[uint32]*UtxoSet {
	return chain.utxoSetCache
}

// ReceiptsCache returns chain receipts cache.
func (chain *BlockChain) ReceiptsCache() map[uint32]types.Receipts {
	return chain.receiptsCache
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
	chain.metricsUtxos(chain.proc)
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
			metrics.MetricsLruCacheBlockGauge.Update(int64(chain.blockcache.Len()))
			metrics.MetricsTailBlockTxsSizeGauge.Update(int64(len(chain.tail.Txs)))
		case <-p.Closing():
			logger.Info("Quit blockchain loop.")
			return
		}
	}
}

func (chain *BlockChain) metricsUtxos(parent goprocess.Process) {
	goprocess.WithParent(parent).Go(
		func(p goprocess.Process) {
			ticker := time.NewTicker(20 * time.Second)
			gcTicker := time.NewTicker(time.Hour)
			missRateTicker := time.NewTicker(10 * time.Minute)

			memstats := &runtime.MemStats{}
			for {
				select {
				case <-ticker.C:
					runtime.ReadMemStats(memstats)
					metrics.MetricsMemAllocGauge.Update(int64(memstats.Alloc))
					metrics.MetricsMemTotalAllocGauge.Update(int64(memstats.TotalAlloc))
					metrics.MetricsMemSysGauge.Update(int64(memstats.Sys))
					metrics.MetricsMemLookupsGauge.Update(int64(memstats.Lookups))
					metrics.MetricsMemMallocsGauge.Update(int64(memstats.Mallocs))
					metrics.MetricsMemFreesGauge.Update(int64(memstats.Frees))
					metrics.MetricsMemHeapAllocGauge.Update(int64(memstats.HeapAlloc))
					metrics.MetricsMemHeapSysGauge.Update(int64(memstats.HeapSys))
					metrics.MetricsMemHeapIdleGauge.Update(int64(memstats.HeapIdle))
					metrics.MetricsMemHeapInuseGauge.Update(int64(memstats.HeapInuse))
					metrics.MetricsMemHeapReleasedGauge.Update(int64(memstats.HeapReleased))
					metrics.MetricsMemHeapObjectsGauge.Update(int64(memstats.HeapObjects))
					metrics.MetricsMemStackInuseGauge.Update(int64(memstats.StackInuse))
					metrics.MetricsMemStackSysGauge.Update(int64(memstats.StackSys))
					metrics.MetricsMemMSpanInuseGauge.Update(int64(memstats.MSpanInuse))
					metrics.MetricsMemMSpanSysGauge.Update(int64(memstats.MSpanSys))
					metrics.MetricsMemMCacheInuseGauge.Update(int64(memstats.MCacheInuse))
					metrics.MetricsMemMCacheSysGauge.Update(int64(memstats.MCacheInuse))
					metrics.MetricsMemBuckHashSysGauge.Update(int64(memstats.BuckHashSys))
					metrics.MetricsMemGCSysGauge.Update(int64(memstats.GCSys))
					metrics.MetricsMemOtherSysGauge.Update(int64(memstats.OtherSys))
					metrics.MetricsMemNextGCGauge.Update(int64(memstats.NextGC))
					metrics.MetricsMemNumForcedGCGauge.Update(int64(memstats.NumForcedGC))

					ctx, cancel := context.WithTimeout(context.Background(), 18*time.Second)
					defer cancel()
					i := 0
					for range chain.db.IterKeysWithPrefix(ctx, utxoBase.Bytes()) {
						i++
					}
					metrics.MetricsUtxoSizeGauge.Update(int64(i))
				case <-missRateTicker.C:
					total, miss := chain.calMissRate()
					if total != 0 {
						metrics.MetricsBlockMissRateGauge.Update(int64(miss * 1000000 / total))
					}
				case <-gcTicker.C:
					logger.Infof("FreeOSMemory invoked.")
					debug.FreeOSMemory()
				case <-p.Closing():
					logger.Info("Quit metricsUtxos loop.")
					return
				}
			}
		})
}

func (chain *BlockChain) calMissRate() (total uint32, miss uint32) {
	logger.Debugf("calMissRate invoked")

	var ts int64
	var height uint32

	val, err := chain.db.Get(MissrateKey)
	if err == nil {
		h, m, t, err := UnmarshalMissData(val)
		if err == nil {
			height, miss, ts = h, m, t
		} else {
			logger.Errorf("UnmarshalMissData Err: %v.", err)
		}
	}

	tail := chain.tail
	if tail == nil {
		return 0, miss
	}

	minersCh := make(chan []string)
	chain.bus.Send(eventbus.TopicMiners, minersCh)
	miners := <-minersCh

	if ts == 0 {
		block, err := chain.LoadBlockByHeight(1)
		if err != nil {
			return 0, miss
		}
		ts = block.Header.TimeStamp
		total = uint32(tail.Header.TimeStamp - ts)
	}

	errCh := make(chan error)
	var curTs int64
	var block *types.Block
	for tstmp := ts; tstmp < tail.Header.TimeStamp; tstmp++ {
		chain.bus.Send(eventbus.TopicCheckMiner, tstmp, errCh)
		err = <-errCh

		if err != nil {
			continue
		}
		curTs = 0
		for ; ; height++ {
			block, err = chain.LoadBlockByHeight(height)
			if err != nil || block == nil {
				break
			}
			if block.Header.TimeStamp >= tstmp {
				curTs = block.Header.TimeStamp
				height = block.Header.Height + 1
				break
			}
		}
		if curTs > tstmp {
			miss++
		}
	}

	if val, err := MarshalMissData(tail.Header.Height, miss, tail.Header.TimeStamp); err == nil {
		chain.db.Put(MissrateKey, val)
	}
	return tail.Header.Height / uint32(len(miners)), miss
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

	if msg.From() == "" {
		return core.ErrInvalidMessageSender
	}

	// process block
	if err := chain.ProcessBlock(block, core.RelayMode, msg.From()); err != nil && util.InArray(err, core.EvilBehavior) {
		chain.Bus().Publish(eventbus.TopicConnEvent, msg.From(), eventbus.BadBlockEvent)
		return err
	}
	chain.Bus().Publish(eventbus.TopicConnEvent, msg.From(), eventbus.NewBlockEvent)
	return nil
}

// ProcessBlock is used to handle new blocks.
func (chain *BlockChain) ProcessBlock(block *types.Block, transferMode core.TransferMode, messageFrom peer.ID) error {
	chain.chainLock.Lock()
	defer func() {
		chain.chainLock.Unlock()
		atomic.StoreInt32(&chain.status, free)
	}()

	atomic.StoreInt32(&chain.status, busy)

	t0 := time.Now().UnixNano()
	blockHash := block.BlockHash()
	logger.Infof("Prepare to process block. Hash: %s, Height: %d", blockHash.String(), block.Header.Height)

	// The block must not already exist in the main chain or side chains.
	if exists := chain.verifyExists(*blockHash); exists {
		logger.Warnf("The block already exists. Hash: %s, Height: %d", blockHash.String(), block.Header.Height)
		return core.ErrBlockExists
	}

	if ok := chain.verifyRepeatedMint(block); !ok {
		return core.ErrRepeatedMintAtSameTime
	}

	if messageFrom != "" { // local block does not require validation
		if err := chain.consensus.Verify(block); err != nil {
			logger.Errorf("Failed to verify block. Hash: %v, Height: %d, Err: %v", block.BlockHash().String(), block.Header.Height, err)
			return err
		}
	}

	if err := validateBlock(block); err != nil {
		logger.Errorf("Failed to validate block. Hash: %v, Height: %d, Err: %s", block.BlockHash(), block.Header.Height, err.Error())
		return err
	}
	prevHash := block.Header.PrevBlockHash
	if prevHashExists := chain.blockExists(prevHash); !prevHashExists {

		// Orphan block.
		logger.Infof("Adding orphan block %v with parent %v", blockHash.String(), prevHash.String())
		chain.addOrphanBlock(block, *blockHash, prevHash)
		chain.repeatedMintCache.Add(block.Header.TimeStamp, block)
		height := chain.tail.Header.Height
		if height < block.Header.Height && messageFrom != core.SyncFlag {
			if block.Header.Height-height < Threshold {
				return chain.syncManager.ActiveLightSync(messageFrom)
			}
			// trigger sync
			chain.syncManager.StartSync()
		}
		return nil
	}

	t1 := time.Now().UnixNano()
	// All context-free checks pass, try to accept the block into the chain.
	if err := chain.tryAcceptBlock(block, messageFrom); err != nil {
		logger.Errorf("Failed to accept the block into the main chain. Err: %s", err)
		return err
	}

	t2 := time.Now().UnixNano()
	if err := chain.processOrphans(block, messageFrom); err != nil {
		logger.Errorf("Failed to processOrphans. Err: %s", err.Error())
		return err
	}

	chain.BroadcastOrRelayBlock(block, transferMode)
	go chain.Bus().Publish(eventbus.TopicRPCSendNewBlock, block)

	logger.Debugf("Accepted New Block. Hash: %v Height: %d TxsNum: %d", blockHash.String(), block.Header.Height, len(block.Txs))
	t3 := time.Now().UnixNano()
	if needToTracking((t1-t0)/1e6, (t2-t1)/1e6, (t3-t2)/1e6) {
		logger.Infof("Time tracking: t0` = %d t1` = %d t2` = %d", (t1-t0)/1e6, (t2-t1)/1e6, (t3-t2)/1e6)
	}

	return nil
}

func needToTracking(t ...int64) bool {
	for _, v := range t {
		if v >= 200 {
			return true
		}
	}
	return false
}

func (chain *BlockChain) verifyExists(blockHash crypto.HashType) bool {
	return chain.blockExists(blockHash) || chain.isInOrphanPool(blockHash)
}

func (chain *BlockChain) blockExists(blockHash crypto.HashType) bool {
	if chain.blockcache.Contains(blockHash) {
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
func (chain *BlockChain) tryAcceptBlock(block *types.Block, messageFrom peer.ID) error {
	blockHash := block.BlockHash()
	// must not be orphan if reaching here
	parentBlock := chain.GetParentBlock(block)
	if parentBlock == nil {
		return core.ErrParentBlockNotExist
	}

	// The height of this block must be one more than the referenced parent block.
	if block.Header.Height != parentBlock.Header.Height+1 {
		logger.Errorf("Block %v's height is %d, but its parent's height is %d", blockHash.String(), block.Header.Height, parentBlock.Header.Height)
		return core.ErrWrongBlockHeight
	}

	// chain.blockcache.Add(*blockHash, block)

	// Connect the passed block to the main or side chain.
	// There are 3 cases.
	parentHash := &block.Header.PrevBlockHash
	tailHash := chain.TailBlock().BlockHash()

	// Case 1): The new block extends the main chain.
	// We expect this to be the most common case.
	if parentHash.IsEqual(tailHash) {
		return chain.tryConnectBlockToMainChain(block, messageFrom)
	}

	// Case 2): The block extends or creats a side chain, which is not longer than the main chain.
	if block.Header.Height <= chain.LongestChainHeight {
		if block.Header.Height > chain.eternal.Header.Height {
			logger.Warnf("Block %v extends a side chain to height %d without causing reorg, "+
				"main chain height %d", blockHash, block.Header.Height, chain.LongestChainHeight)
			// we can store the side chain block, But we should not go on the chain.
			if err := chain.StoreBlock(block); err != nil {
				return err
			}
			if err := chain.processOrphans(block, messageFrom); err != nil {
				logger.Errorf("Failed to processOrphans. Err: %s", err.Error())
				return err
			}
			return core.ErrBlockInSideChain
		}
		logger.Warnf("Block %v extends a side chain height[%d] is lower than eternal block height[%d]", blockHash, block.Header.Height, chain.eternal.Header.Height)
		return core.ErrExpiredBlock
	}

	// Case 3): Extended side chain is longer than the main chain and becomes the new main chain.
	logger.Warnf("REORGANIZE: Block %v is causing a reorganization.", blockHash.String())

	return chain.reorganize(block, messageFrom)
}

// BroadcastOrRelayBlock broadcast or relay block to other peers.
func (chain *BlockChain) BroadcastOrRelayBlock(block *types.Block, transferMode core.TransferMode) {

	blockHash := block.BlockHash()
	switch transferMode {
	case core.BroadcastMode:
		logger.Debugf("Broadcast New Block. Hash: %v Height: %d", blockHash.String(), block.Header.Height)
		go func() {
			if err := chain.notifiee.Broadcast(p2p.NewBlockMsg, block); err != nil {
				logger.Errorf("Failed to broadcast block. Hash: %s Err: %v", blockHash.String(), err)
			}
		}()
	case core.RelayMode:
		logger.Debugf("Relay New Block. Hash: %v Height: %d", blockHash.String(), block.Header.Height)
		go func() {
			if err := chain.notifiee.Relay(p2p.NewBlockMsg, block); err != nil {
				logger.Errorf("Failed to relay block. Hash: %s Err: %v", blockHash.String(), err)
			}
		}()
	default:
	}
}

func (chain *BlockChain) addOrphanBlock(orphan *types.Block, orphanHash crypto.HashType, parentHash crypto.HashType) {
	chain.hashToOrphanBlock[orphanHash] = orphan
	// Add to parent hash map lookup index for faster dependency lookups.
	chain.orphanBlockHashToChildren[parentHash] = append(chain.orphanBlockHashToChildren[parentHash], orphan)
}

func (chain *BlockChain) processOrphans(block *types.Block, messageFrom peer.ID) error {

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
			if err := chain.tryAcceptBlock(orphan, messageFrom); err != nil {
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

// GetParentBlock Finds the parent of a block. Return nil if nonexistent
func (chain *BlockChain) GetParentBlock(block *types.Block) *types.Block {

	// check for genesis.
	if block.Header.PrevBlockHash.IsEqual(chain.genesis.BlockHash()) {
		return chain.genesis
	}
	if target, ok := chain.blockcache.Get(block.Header.PrevBlockHash); ok {
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
func (chain *BlockChain) tryConnectBlockToMainChain(block *types.Block, messageFrom peer.ID) error {

	logger.Infof("Try to connect block to main chain. Hash: %s, Height: %d", block.BlockHash().String(), block.Header.Height)
	var utxoSet *UtxoSet
	if messageFrom == "" { // locally generated block
		us, ok := chain.utxoSetCache[block.Header.Height]
		if !ok {
			return errors.New("utxoSet does not exist in cache")
		}
		delete(chain.utxoSetCache, block.Header.Height)
		utxoSet = us
	} else {
		utxoSet = NewUtxoSet()
		if err := utxoSet.LoadBlockUtxos(block, chain.db); err != nil {
			return err
		}
		// Validate scripts here before utxoSet is updated; otherwise it may fail mistakenly
		if err := validateBlockScripts(utxoSet, block); err != nil {
			return err
		}
	}

	// // Validate scripts here before utxoSet is updated; otherwise it may fail mistakenly
	// if err := validateBlockScripts(utxoSet, block); err != nil {
	// 	return err
	// }
	transactions := block.Txs
	// Perform several checks on the inputs for each transaction.
	// Also accumulate the total fees.
	var totalFees uint64
	for _, tx := range transactions {
		txFee, err := ValidateTxInputs(utxoSet, tx, block.Header.Height)
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

	return chain.applyBlock(block, utxoSet, totalFees, messageFrom)
}

func (chain *BlockChain) tryToClearCache(attachBlocks, detachBlocks []*types.Block) {
	for _, v := range detachBlocks {
		chain.blockcache.Remove(*v.BlockHash())
	}
	for _, v := range attachBlocks {
		chain.blockcache.Add(*v.BlockHash(), v)
	}

}

// findFork returns final common block between the passed block and the main chain (i.e., fork point)
// and blocks to be detached and attached
func (chain *BlockChain) findFork(block *types.Block) (*types.Block, []*types.Block, []*types.Block) {
	if block.Header.Height <= chain.LongestChainHeight {
		logger.Panicf("Side chain (height: %d) is not longer than main chain (height: %d) during chain reorg",
			block.Header.Height, chain.LongestChainHeight)
	}
	detachBlocks := make([]*types.Block, 0)
	attachBlocks := make([]*types.Block, 0)

	// Start both chain from same height by moving up side chain
	sideChainBlock := block
	for i := block.Header.Height; i > chain.LongestChainHeight; i-- {
		if sideChainBlock == nil {
			logger.Panicf("Block on side chain shall not be nil before reaching main chain height during reorg")
		}
		attachBlocks = append(attachBlocks, sideChainBlock)
		sideChainBlock = chain.GetParentBlock(sideChainBlock)
	}

	// Compare two blocks at the same height till they are identical: the fork point
	mainChainBlock, found := chain.TailBlock(), false
	for mainChainBlock != nil && sideChainBlock != nil {
		if mainChainBlock.Header.Height != sideChainBlock.Header.Height {
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
		mainChainBlock, sideChainBlock = chain.GetParentBlock(mainChainBlock), chain.GetParentBlock(sideChainBlock)
	}
	if !found {
		logger.Panicf("Fork point not found, but main chain and side chain share at least one common block, i.e., genesis")
	}
	if len(attachBlocks) <= len(detachBlocks) {
		logger.Panicf("Blocks to be attached (%d) should be strictly more than ones to be detached (%d)", len(attachBlocks), len(detachBlocks))
	}
	return mainChainBlock, detachBlocks, attachBlocks
}

// UpdateNormalTxBalanceState updates the balance state of normal tx
func (chain *BlockChain) UpdateNormalTxBalanceState(block *types.Block, utxoset *UtxoSet, stateDB *state.StateDB) {
	// update EOA accounts' balance state
	bAdd, bSub := utxoset.calcNormalTxBalanceChanges(block)
	for a, v := range bAdd {
		stateDB.AddBalance(a, new(big.Int).SetUint64(v))
	}
	for a, v := range bSub {
		stateDB.SubBalance(a, new(big.Int).SetUint64(v))
	}
}

// UpdateUtxoState updates contract utxo in statedb
func (chain *BlockChain) UpdateUtxoState(statedb *state.StateDB, utxoSet *UtxoSet) error {
	for _, o := range utxoSet.contractUtxos {
		// address
		contractAddr := new(types.AddressHash)
		contractAddr.SetBytes(o.Hash[:])
		// serialize utxo wrap
		u := utxoSet.utxoMap[*o]
		utxoBytes, err := SerializeUtxoWrap(u)
		if err != nil {
			return err
		}
		// update statedb utxo trie
		if err := statedb.UpdateUtxo(*contractAddr, utxoBytes); err != nil {
			logger.Error(err)
			return err
		}
	}
	return nil
}

func (chain *BlockChain) applyBlock(block *types.Block, utxoSet *UtxoSet, totalTxsFee uint64, messageFrom peer.ID) error {

	ttt1 := time.Now().UnixNano()

	blockCopy := block.Copy()
	// Split tx outputs if any
	splitTxs := chain.SplitBlockOutputs(blockCopy)

	// execute contract tx and update statedb for blocks from remote peers
	if messageFrom != "" {
		parent := chain.GetParentBlock(block)
		var rootHash, utxoRootHash *crypto.HashType
		if parent != nil {
			if parent.Header.RootHash != zeroHash {
				rootHash = &parent.Header.RootHash
			}
			if parent.Header.UtxoRoot != zeroHash {
				utxoRootHash = &parent.Header.UtxoRoot
			}
		}
		stateDB, err := state.New(rootHash, utxoRootHash, chain.db)
		if err != nil {
			logger.Error(err)
			return err
		}

		// Save a deep copy before we potentially split the block's txs' outputs and mutate it
		if err := utxoSet.ApplyBlock(blockCopy, chain.db); err != nil {
			return err
		}

		receipts, gasUsed, gasRemainingFee, utxoTxs, err := chain.stateProcessor.Process(
			block, stateDB, utxoSet)
		if err != nil {
			logger.Error(err)
			return err
		}
		if err := chain.ValidateExecuteResult(block, utxoTxs, gasUsed, gasRemainingFee,
			totalTxsFee, receipts); err != nil {
			return err
		}

		chain.UpdateNormalTxBalanceState(blockCopy, utxoSet, stateDB)

		// apply internal txs.
		if len(block.InternalTxs) > 0 {
			if err := utxoSet.ApplyInternalTxs(block, chain.db); err != nil {
				return err
			}
		}
		if err := chain.UpdateUtxoState(stateDB, utxoSet); err != nil {
			logger.Errorf("chain update utxo state error: %s", err)
			return err
		}

		root, utxoRoot, err := stateDB.Commit(false)
		if err != nil {
			logger.Errorf("stateDB commit failed: %s", err)
			return err
		}
		if !root.IsEqual(&block.Header.RootHash) {
			return fmt.Errorf("Invalid state root in block header, have %s, got: %s, "+
				"block hash: %s height: %d", block.Header.RootHash, root, block.BlockHash(),
				block.Header.Height)
		}
		if !utxoRoot.IsEqual(&block.Header.UtxoRoot) &&
			!(utxoRoot == nil && block.Header.UtxoRoot == zeroHash) {
			return fmt.Errorf("Invalid utxo state root in block header, have %s, "+
				"got: %s, block hash: %s height: %d", block.Header.UtxoRoot, utxoRoot,
				block.BlockHash(), block.Header.Height)
		}
		chain.stateDBCache[block.Header.Height] = stateDB
		if len(receipts) > 0 {
			chain.receiptsCache[block.Header.Height] = receipts
		}
	}

	chain.db.EnableBatch()
	defer chain.db.DisableBatch()

	if err := chain.StoreBlockWithIndex(block, chain.db); err != nil {
		return err
	}

	receipts := chain.receiptsCache[block.Header.Height]
	if len(receipts) > 0 {
		if err := chain.StoreReceipts(block.BlockHash(), receipts, chain.db); err != nil {
			return err
		}
	}

	ttt2 := time.Now().UnixNano()

	if err := chain.consensus.Process(block, chain.db); err != nil {
		return err
	}

	// save tx index
	if err := chain.WriteTxIndex(block, splitTxs, chain.db); err != nil {
		return err
	}

	// save split tx
	if err := chain.StoreSplitTxs(splitTxs, chain.db); err != nil {
		return err
	}

	ttt3 := time.Now().UnixNano()
	// store split addr index
	if err := chain.WriteSplitAddrIndex(block, chain.db); err != nil {
		logger.Error(err)
		return err
	}
	ttt4 := time.Now().UnixNano()
	// save utxoset to database
	if err := utxoSet.WriteUtxoSetToDB(chain.db); err != nil {
		return err
	}
	ttt5 := time.Now().UnixNano()
	// save current tail to database
	if err := chain.StoreTailBlock(block, chain.db); err != nil {
		return err
	}

	if err := chain.db.Flush(); err != nil {
		logger.Errorf("Failed to batch write block. Hash: %s, Height: %d, Err: %s",
			block.BlockHash().String(), block.Header.Height, err.Error())
	}
	ttt6 := time.Now().UnixNano()
	chain.tryToClearCache([]*types.Block{block}, nil)

	// notify mem_pool when chain update
	chain.notifyBlockConnectionUpdate([]*types.Block{block}, nil)

	// This block is now the end of the best chain.
	chain.ChangeNewTail(block)
	ttt7 := time.Now().UnixNano()
	if needToTracking((ttt2-ttt1)/1e6, (ttt3-ttt2)/1e6, (ttt4-ttt3)/1e6, (ttt5-ttt4)/1e6, (ttt6-ttt5)/1e6, (ttt7-ttt6)/1e6) {
		logger.Infof("ttt Time tracking: ttt1` = %d ttt2` = %d ttt3` = %d ttt4` = %d ttt5` = %d ttt6` = %d ", (ttt2-ttt1)/1e6, (ttt3-ttt2)/1e6, (ttt4-ttt3)/1e6, (ttt5-ttt4)/1e6, (ttt6-ttt5)/1e6, (ttt7-ttt6)/1e6)
	}
	return nil
}

func checkInternalTxs(block *types.Block, utxoTxs []*types.Transaction) error {

	//if len(utxoTxs) != len(block.InternalTxs) {
	//	logger.Warnf("utxo txs generated len: %d, internal txs in block len: %d", len(utxoTxs),
	//		len(block.InternalTxs))
	//	return core.ErrInvalidInternalTxs
	//}
	if len(utxoTxs) > 0 {
		txsRoot := CalcTxsHash(utxoTxs)
		if !(&block.Header.InternalTxsRoot).IsEqual(txsRoot) {
			utxoTxsBytes, _ := json.MarshalIndent(utxoTxs, "", "  ")
			internalTxs, _ := json.MarshalIndent(block.InternalTxs, "", "  ")
			logger.Warnf("utxo txs generated: %s, internal txs in block: %v",
				string(utxoTxsBytes), string(internalTxs))
			logger.Warnf("utxo txs root: %s, internal txs root: %s", txsRoot, block.Header.InternalTxsRoot)
			return core.ErrInvalidInternalTxs
		}
	} else {
		block.InternalTxs = make([]*types.Transaction, 0)
	}
	return nil
}

// ValidateExecuteResult validates evm execute result
func (chain *BlockChain) ValidateExecuteResult(
	block *types.Block, utxoTxs []*types.Transaction, usedGas, gasRemainingFee, totalTxsFee uint64,
	receipts types.Receipts,
) error {

	if err := checkInternalTxs(block, utxoTxs); err != nil {
		return err
	}
	if block.Header.GasUsed != usedGas {
		logger.Warnf("gas used in block header: %d, now: %d", block.Header.GasUsed, usedGas)
		return errors.New("Invalid gasUsed in block header")
	}

	// Ensure coinbase does not output more than block reward.
	var totalCoinbaseOutput uint64
	for _, txOut := range block.Txs[0].Vout {
		totalCoinbaseOutput += txOut.Value
	}
	expectedCoinbaseOutput := CalcBlockSubsidy(block.Header.Height) + totalTxsFee - gasRemainingFee
	if totalCoinbaseOutput > expectedCoinbaseOutput {
		logger.Errorf("coinbase transaction for block pays %v which is more than expected value %v("+
			"totalTxsFee: %d, gas remaining: %d)", totalCoinbaseOutput, expectedCoinbaseOutput,
			totalTxsFee, gasRemainingFee)
		return core.ErrBadCoinbaseValue
	}
	// check receipt
	var receiptHash crypto.HashType
	if len(receipts) > 0 {
		receiptHash = *receipts.Hash()
	}
	if receiptHash != block.Header.ReceiptHash {
		logger.Warnf("receipt hash in block header: %s, now: %s, block hash: %s",
			block.Header.ReceiptHash, receiptHash, block.BlockHash())
		return errors.New("Invalid receipt hash in block header")
	}

	return nil
}

func (chain *BlockChain) notifyBlockConnectionUpdate(attachBlocks, detachBlocks []*types.Block) error {
	chain.bus.Publish(eventbus.TopicChainUpdate, &UpdateMsg{
		AttachBlocks: attachBlocks,
		DetachBlocks: detachBlocks,
	})
	return nil
}

func (chain *BlockChain) notifyUtxoChange(utxoSet *UtxoSet) {
	chain.bus.Publish(eventbus.TopicUtxoUpdate, utxoSet)
}

func (chain *BlockChain) reorganize(block *types.Block, messageFrom peer.ID) error {
	// Find the common ancestor of the main chain and side chain
	forkpoint, detachBlocks, attachBlocks := chain.findFork(block)
	if forkpoint.Header.Height < chain.eternal.Header.Height {
		// delete all block from forkpoint.
		for _, attachBlock := range attachBlocks {
			delete(chain.hashToOrphanBlock, *attachBlock.BlockHash())
			delete(chain.orphanBlockHashToChildren, *attachBlock.BlockHash())
			chain.RemoveBlock(attachBlock)
		}

		logger.Warnf("No need to reorganize, because the forkpoint height[%d] is lower than the latest eternal block height[%d].", forkpoint.Header.Height, chain.eternal.Header.Height)
		return nil
	}

	for _, detachBlock := range detachBlocks {
		stt0 := time.Now().UnixNano()
		if err := chain.tryDisConnectBlockFromMainChain(detachBlock); err != nil {
			logger.Errorf("Failed to disconnect block from main chain. Err: %v", err)
			panic("Failed to disconnect block from main chain")
		}
		stt1 := time.Now().UnixNano()
		logger.Infof("Disconnect block %s, time tracking: %d", detachBlock.BlockHash(), (stt1-stt0)/1e6)
	}

	for blockIdx := len(attachBlocks) - 1; blockIdx >= 0; blockIdx-- {
		stt0 := time.Now().UnixNano()
		attachBlock := attachBlocks[blockIdx]
		if err := chain.tryConnectBlockToMainChain(attachBlock, messageFrom); err != nil {
			return err
		}
		stt1 := time.Now().UnixNano()
		logger.Infof("Connect block %s, time tracking: %d", attachBlock.BlockHash(), (stt1-stt0)/1e6)
	}

	metrics.MetricsBlockRevertMeter.Mark(1)
	return nil
}

func (chain *BlockChain) tryDisConnectBlockFromMainChain(block *types.Block) error {
	dtt0 := time.Now().UnixNano()
	logger.Debugf("Try to disconnect block from main chain. Hash: %s Height: %d", block.BlockHash().String(), block.Header.Height)
	// batch := chain.db.NewBatch()
	// defer batch.Close()
	chain.db.EnableBatch()
	defer chain.db.DisableBatch()

	// Save a deep copy before we potentially split the block's txs' outputs and mutate it
	blockCopy := block.Copy()

	// Split tx outputs if any
	splitTxs := chain.SplitBlockOutputs(blockCopy)
	dtt1 := time.Now().UnixNano()
	utxoSet := NewUtxoSet()
	if err := utxoSet.LoadBlockAllUtxos(blockCopy, chain.db); err != nil {
		return err
	}
	if err := utxoSet.RevertBlock(blockCopy, chain); err != nil {
		return err
	}

	// update EOA accounts' balance state
	//bAdd, bSub := utxoSet.calcBalanceChanges()
	//for a, v := range bAdd {
	//	stateDB.AddBalance(a, new(big.Int).SetUint64(v))
	//}
	//for a, v := range bSub {
	//	stateDB.SubBalance(a, new(big.Int).SetUint64(v))
	//}

	dtt2 := time.Now().UnixNano()
	// batch.Del(BlockKey(block.BlockHash()))
	chain.db.Del(BlockHashKey(block.Header.Height))

	// chain.filterHolder.ResetFilters(block.Height)
	dtt3 := time.Now().UnixNano()
	// del tx index
	if err := chain.DelTxIndex(block, splitTxs, chain.db); err != nil {
		return err
	}

	// del split tx
	if err := chain.DelSplitTxs(splitTxs, chain.db); err != nil {
		return err
	}

	if err := chain.DeleteSplitAddrIndex(block, chain.db); err != nil {
		return err
	}
	dtt4 := time.Now().UnixNano()
	if err := utxoSet.WriteUtxoSetToDB(chain.db); err != nil {
		return err
	}
	dtt5 := time.Now().UnixNano()

	if err := chain.db.Flush(); err != nil {
		logger.Errorf("Failed to batch write block. Hash: %s, Height: %d, Err: %s",
			block.BlockHash().String(), block.Header.Height, err.Error())
	}
	dtt6 := time.Now().UnixNano()
	chain.tryToClearCache(nil, []*types.Block{block})

	// notify mem_pool when chain update
	chain.notifyBlockConnectionUpdate(nil, []*types.Block{block})
	dtt7 := time.Now().UnixNano()
	// This block is now the end of the best chain.
	// chain.ChangeNewTail(block)
	if needToTracking((dtt1-dtt0)/1e6, (dtt2-dtt1)/1e6, (dtt3-dtt2)/1e6, (dtt4-dtt3)/1e6, (dtt5-dtt4)/1e6, (dtt6-dtt5)/1e6, (dtt7-dtt6)/1e6) {
		logger.Infof("dtt Time tracking: dtt0` = %d dtt1` = %d dtt2` = %d dtt3` = %d dtt4` = %d dtt5` = %d dtt6` = %d", (dtt1-dtt0)/1e6, (dtt2-dtt1)/1e6, (dtt3-dtt2)/1e6, (dtt4-dtt3)/1e6, (dtt5-dtt4)/1e6, (dtt6-dtt5)/1e6, (dtt7-dtt6)/1e6)
	}
	return nil
}

// StoreTailBlock store tail block to db.
func (chain *BlockChain) StoreTailBlock(block *types.Block, db storage.Table) error {
	data, err := block.Marshal()
	if err != nil {
		return err
	}
	db.Put(TailKey, data)
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
	if eternal.Header.Height < block.Header.Height {
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

// GetBlockHeight returns current height of main chain
func (chain *BlockChain) GetBlockHeight() uint32 {
	return chain.LongestChainHeight
}

// GetBalance finds the block in target height of main chain and returns it's hash
func (chain *BlockChain) GetBalance(addr types.Address) (uint64, error) {
	stateDB := chain.stateDBCache[chain.LongestChainHeight]
	if stateDB == nil {
		return 0, errors.New("state db is nil")
	}
	return stateDB.GetBalance(*addr.Hash160()).Uint64(), nil
}

// GetBlockHash finds the block in target height of main chain and returns it's hash
func (chain *BlockChain) GetBlockHash(blockHeight uint32) (*crypto.HashType, error) {
	block, err := chain.LoadBlockByHeight(blockHeight)
	if err != nil {
		return nil, err
	}
	return block.BlockHash(), nil
}

// ChangeNewTail change chain tail block.
func (chain *BlockChain) ChangeNewTail(tail *types.Block) {

	if err := chain.consensus.Finalize(tail); err != nil {
		panic("Failed to change new tail in consensus.")
	}

	chain.repeatedMintCache.Add(tail.Header.TimeStamp, tail)
	// chain.heightToBlock.Add(tail.Height, tail)
	chain.LongestChainHeight = tail.Header.Height
	chain.tail = tail
	logger.Infof("Change New Tail. Hash: %s Height: %d txsNum: %d", tail.BlockHash().String(), tail.Header.Height, len(tail.Txs))

	metrics.MetricsBlockHeightGauge.Update(int64(tail.Header.Height))
	metrics.MetricsBlockTailHashGauge.Update(int64(util.HashBytes(tail.BlockHash().GetBytes())))
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

	utxoSet := NewUtxoSet()
	for _, v := range genesis.Txs {
		for idx := range v.Vout {
			utxoSet.AddUtxo(v, uint32(idx), genesis.Header.Height)
		}
	}
	// statedb
	stateDB, err := state.New(nil, nil, chain.db)
	if err != nil {
		return nil, err
	}
	chain.UpdateNormalTxBalanceState(&genesis, utxoSet, stateDB)
	root, _, err := stateDB.Commit(false)
	if err != nil {
		return nil, err
	}
	logger.Infof("genesis root hash: %s", root)
	genesis.Header.RootHash = *root

	chain.db.EnableBatch()
	defer chain.db.DisableBatch()

	utxoSet.WriteUtxoSetToDB(chain.db)
	if err := chain.WriteTxIndex(&genesis, nil, chain.db); err != nil {
		return nil, err
	}
	genesisBin, err := genesis.Marshal()
	if err != nil {
		return nil, err
	}
	chain.db.Put(BlockKey(genesis.BlockHash()), genesisBin)
	chain.db.Put(GenesisKey, genesisBin)
	if err := chain.db.Flush(); err != nil {
		return nil, err
	}
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

// ReadBlockFromDB reads a block from db by hash and returns block and it's size
func (chain *BlockChain) ReadBlockFromDB(hash *crypto.HashType) (*types.Block, int, error) {

	blockBin, err := chain.db.Get(BlockKey(hash))
	if err != nil {
		return nil, 0, err
	}
	if blockBin == nil {
		return nil, 0, core.ErrBlockIsNil
	}
	n := len(blockBin)
	block := new(types.Block)
	if err := block.Unmarshal(blockBin); err != nil {
		return nil, 0, err
	}

	return block, n, nil
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

// GetEvmByHeight get evm by block height.
func (chain *BlockChain) GetEvmByHeight(msg types.Message, height uint32) (*vm.EVM, func() error, error) {
	if height == 0 {
		height = chain.tail.Header.Height
	}
	block, err := chain.LoadBlockByHeight(height)
	if block == nil || err != nil {
		return nil, nil, err
	}
	state, err := state.New(&block.Header.RootHash, &block.Header.UtxoRoot, chain.db)
	if state == nil || err != nil {
		return nil, nil, err
	}
	state.SetBalance(*msg.From(), math.MaxBig256)
	context := NewEVMContext(msg, block.Header, chain)
	return vm.NewEVM(context, state, vm.Config{}), state.Error, nil
}

// GetLatestNonce get nonce in chain now
func (chain *BlockChain) GetLatestNonce(address *types.AddressHash) (uint64, error) {
	header := chain.TailBlock().Header
	state, err := state.New(&header.RootHash, &header.UtxoRoot, chain.db)
	if state == nil || err != nil {
		return 0, err
	}
	return state.GetNonce(*address), nil
}

// StoreBlockWithIndex store block to db in batch mod.
func (chain *BlockChain) StoreBlockWithIndex(block *types.Block, db storage.Table) error {

	hash := block.BlockHash()
	db.Put(BlockHashKey(block.Header.Height), hash[:])
	return chain.StoreBlock(block)
}

// StoreBlock store block to db.
func (chain *BlockChain) StoreBlock(block *types.Block) error {

	hash := block.BlockHash()
	data, err := block.Marshal()
	if err != nil {
		return err
	}
	chain.db.Put(BlockKey(hash), data)
	return nil
}

// RemoveBlock store block to db.
func (chain *BlockChain) RemoveBlock(block *types.Block) {

	hash := block.BlockHash()
	if ok, _ := chain.db.Has(BlockKey(hash)); ok {
		chain.db.Del(BlockKey(hash))
	}
}

// StoreReceipts store receipts to db in batch mod.
func (chain *BlockChain) StoreReceipts(hash *crypto.HashType, receipts types.Receipts, db storage.Table) error {

	data, err := receipts.Marshal()
	if err != nil {
		return err
	}
	db.Put(ReceiptKey(hash), data)
	return nil
}

// LoadBlockInfoByTxHash returns block and txIndex of transaction with the input param hash
func (chain *BlockChain) LoadBlockInfoByTxHash(hash crypto.HashType) (*types.Block, *types.Transaction, error) {
	txIndex, err := chain.db.Get(TxIndexKey(&hash))
	if err != nil {
		return nil, nil, err
	}
	height, idx, err := UnmarshalTxIndex(txIndex)
	if err != nil {
		return nil, nil, err
	}
	block, err := chain.LoadBlockByHeight(height)
	if err != nil {
		return nil, nil, err
	}

	var tx *types.Transaction
	if idx < uint32(len(block.Txs)) {
		tx = block.Txs[idx]
	} else {
		txBin, err := chain.db.Get(TxKey(&hash))
		if err != nil {
			return nil, nil, err
		}
		if txBin == nil {
			return nil, nil, errors.New("failed to load split tx with hash")
		}
		tx = new(types.Transaction)
		if err := tx.Unmarshal(txBin); err != nil {
			return nil, nil, err
		}
	}
	// tx := block.Txs[idx]
	target, err := tx.TxHash()
	if err != nil {
		return nil, nil, err
	}
	if *target == hash {
		return block, tx, nil
	}
	logger.Errorf("Error reading tx hash, expect: %s got: %s", hash.String(), target.String())
	return nil, nil, errors.New("failed to load tx with hash")
}

// WriteTxIndex builds tx index in block
// Save split transaction copies before and after split. The latter is needed when reverting a transaction during reorg,
// spending from utxo/coin received at a split address
func (chain *BlockChain) WriteTxIndex(block *types.Block, splitTxs map[crypto.HashType]*types.Transaction, db storage.Table) error {

	allTxs := block.Txs
	for _, v := range splitTxs {
		allTxs = append(block.Txs, v)
	}
	for idx, tx := range allTxs {
		tiBuf, err := MarshalTxIndex(block.Header.Height, uint32(idx))
		if err != nil {
			return err
		}
		txHash, err := tx.TxHash()
		if err != nil {
			return err
		}
		db.Put(TxIndexKey(txHash), tiBuf)
	}

	return nil
}

// StoreSplitTxs store split txs.
func (chain *BlockChain) StoreSplitTxs(splitTxs map[crypto.HashType]*types.Transaction, db storage.Table) error {
	for hash, tx := range splitTxs {
		txHash, err := tx.TxHash()
		if err != nil {
			return err
		}
		txBin, err := tx.Marshal()
		if err != nil {
			return err
		}
		db.Put(SplitTxHashKey(&hash), txBin)
		db.Put(TxKey(txHash), txBin)
	}
	return nil
}

// DelTxIndex deletes tx index in block
// Delete split transaction copies saved earlier, both before and after split
func (chain *BlockChain) DelTxIndex(block *types.Block, splitTxs map[crypto.HashType]*types.Transaction, db storage.Table) error {

	allTxs := block.Txs
	for _, v := range splitTxs {
		allTxs = append(block.Txs, v)
	}

	for _, tx := range allTxs {
		txHash, err := tx.TxHash()
		if err != nil {
			return err
		}
		db.Del(TxIndexKey(txHash))
	}

	return nil
}

// DelSplitTxs del split txs.
func (chain *BlockChain) DelSplitTxs(splitTxs map[crypto.HashType]*types.Transaction, db storage.Table) error {
	for hash, tx := range splitTxs {
		txHash, err := tx.TxHash()
		if err != nil {
			return err
		}
		db.Del(TxKey(txHash))
		db.Del(SplitTxHashKey(&hash))
	}
	return nil
}

// LocateForkPointAndFetchHeaders return block headers when get locate fork point request for sync service.
func (chain *BlockChain) LocateForkPointAndFetchHeaders(hashes []*crypto.HashType) ([]*crypto.HashType, error) {
	tailHeight := chain.tail.Header.Height
	for index := range hashes {
		block, err := chain.LoadBlockByHash(*hashes[index])
		if err != nil {
			continue
		}
		// Important: make sure the block is on main chain !!!
		b, _ := chain.LoadBlockByHeight(block.Header.Height)
		if !b.BlockHash().IsEqual(block.BlockHash()) {
			continue
		}

		result := []*crypto.HashType{}
		currentHeight := block.Header.Height + 1
		if tailHeight-block.Header.Height+1 < MaxBlocksPerSync {
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
	if chain.tail.Header.Height-block.Header.Height+1 < num {
		return nil, fmt.Errorf("Invalid params num[%d] (tailHeight[%d], "+
			"currentHeight[%d])", num, chain.tail.Header.Height, block.Header.Height)
	}
	var idx uint32
	hashes := make([]*crypto.HashType, num)
	for idx < num {
		block, err := chain.LoadBlockByHeight(block.Header.Height + idx)
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
	if num <= 0 || chain.tail.Header.Height-block.Header.Height+1 < num {
		return nil, fmt.Errorf("Invalid params num[%d], tail.Height[%d],"+
			" block height[%d]", num, chain.tail.Header.Height, block.Header.Height)
	}
	var idx uint32
	blocks := make([]*types.Block, num)
	for idx < num {
		block, err := chain.LoadBlockByHeight(block.Header.Height + idx)
		if err != nil {
			return nil, err
		}
		blocks[idx] = block
		idx++
	}
	return blocks, nil
}

// SplitBlockOutputs split outputs of txs in the block where applicable
// return all split transactions, i.e., transactions containing at least one output to a split address
func (chain *BlockChain) SplitBlockOutputs(block *types.Block) map[crypto.HashType]*types.Transaction {
	splitTxs := make(map[crypto.HashType]*types.Transaction, 0)

	for _, tx := range block.Txs {
		hash, _ := tx.TxHash()
		if chain.splitTxOutputs(tx) {
			splitTxs[*hash] = tx
		}
	}

	return splitTxs
}

// split outputs in the tx where applicable
// return if the transaction contains split address output
func (chain *BlockChain) splitTxOutputs(tx *types.Transaction) bool {
	isSplitTx := false
	vout := make([]*corepb.TxOut, 0)
	for _, txOut := range tx.Vout {
		txOuts := chain.splitTxOutput(txOut)
		vout = append(vout, txOuts...)
		if len(txOuts) > 1 {
			isSplitTx = true
		}
	}

	if isSplitTx {
		tx.ResetTxHash()
		tx.Vout = vout
	}

	return isSplitTx
}

// split an output to a split address into  multiple outputs to composite addresses
func (chain *BlockChain) splitTxOutput(txOut *corepb.TxOut) []*corepb.TxOut {
	// return the output itself if it cannot be split
	txOuts := []*corepb.TxOut{txOut}
	sc := script.NewScriptFromBytes(txOut.ScriptPubKey)
	if !sc.IsPayToPubKeyHash() {
		return txOuts
	}
	addr, err := sc.ExtractAddress()
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

// GetReceipt get receipt.
func (chain *BlockChain) GetReceipt(hash *crypto.HashType) (*types.Receipt, error) {

	key := ReceiptKey(hash)
	value, err := chain.db.Get(key)
	if err != nil {
		return nil, err
	}

	receipt := new(types.Receipt)
	if err := receipt.Unmarshal(value); err != nil {
		return nil, err
	}
	return receipt, nil
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
	if !chain.splitAddrFilter.Matches(addr.Hash()) {
		// Definitely not a split address
		return false, nil, nil
	}
	// May be a split address
	// Query db to find out
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
	return true, info, nil
}

// GetDataFromDB get data from db
func (chain *BlockChain) GetDataFromDB(key []byte) ([]byte, error) {
	return chain.db.Get(key)
}

// WriteSplitAddrIndex writes split addr info index
func (chain *BlockChain) WriteSplitAddrIndex(block *types.Block, db storage.Table) error {
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
				db.Put(k, dataBytes)
				chain.splitAddrFilter.Add(addr.Hash())
				logger.Debugf("New Split Address created")
			}
		}
	}
	return nil
}

// DeleteSplitAddrIndex remove split address index from both db and cache
func (chain *BlockChain) DeleteSplitAddrIndex(block *types.Block, db storage.Table) error {
	for _, tx := range block.Txs {
		for _, vout := range tx.Vout {
			sc := *script.NewScriptFromBytes(vout.ScriptPubKey)
			if sc.IsSplitAddrScript() {
				addr, err := sc.ExtractAddress()
				if err != nil {
					return err
				}
				k := SplitAddrKey(addr.Hash())
				db.Del(k)
				logger.Debugf("Remove Split Address: %s", addr.String())
			}
		}
	}
	return nil
}

// FetchOwnerOfOutPoint fetches the owner of an outpoint
func fetchOwnerOfOutPoint(op *types.OutPoint, reader storage.Reader) (types.Address, error) {
	// use sender in vin[0] as VMTransaction sender
	utxo, err := fetchUtxoWrapFromDB(reader, op)
	if err != nil {
		return nil, err
	}
	return script.NewScriptFromBytes(utxo.Script()).ExtractAddress()
}

// ExtractVMTransactions extract Transaction to VMTransaction
func (chain *BlockChain) ExtractVMTransactions(tx *types.Transaction) (*types.VMTransaction, error) {
	// check
	if !HasContractVout(tx) {
		return nil, nil
	}
	sender, err := fetchOwnerOfOutPoint(&tx.Vin[0].PrevOutPoint, chain.DB())
	if err != nil {
		return nil, err
	}
	if !strings.HasPrefix(sender.String(), types.AddrTypeP2PKHPrefix) {
		return nil, fmt.Errorf("cannot extract vm tx from tx that does not contain box utxo")
	}
	// HashWith
	txHash, _ := tx.TxHash()

	for _, o := range tx.Vout {
		sc := script.NewScriptFromBytes(o.ScriptPubKey)
		if sc.IsContractPubkey() {
			p, t, e := sc.ParseContractParams()
			if e != nil {
				return nil, e
			}
			vmTx := types.NewVMTransaction(big.NewInt(int64(o.Value)),
				big.NewInt(int64(p.GasPrice)), p.GasLimit, p.Nonce, txHash, t, p.Code).
				WithSender(sender.Hash160())
			if t == types.ContractCallType {
				vmTx.WithReceiver(p.Receiver)
			}
			return vmTx, nil
		}
	}
	return nil, fmt.Errorf("no vm tx in tx: %s", txHash)
}

// HasContractVout return true if tx has a vout with contract creation or call
func HasContractVout(tx *types.Transaction) bool {
	for _, o := range tx.Vout {
		sc := script.NewScriptFromBytes(o.ScriptPubKey)
		if sc.IsContractPubkey() {
			return true
		}
	}
	return false
}

// HasContractSpend return true if tx has a vin with Op Spend script sig
func HasContractSpend(tx *types.Transaction) bool {
	for _, i := range tx.Vin {
		sc := script.NewScriptFromBytes(i.ScriptSig)
		if sc.IsContractSig() {
			return true
		}
	}
	return false
}
