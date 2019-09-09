// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package chain

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	sysmath "math"
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
	"github.com/BOXFoundation/boxd/core/txlogic"
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
	"github.com/BOXFoundation/boxd/vm/common/hexutil"
	"github.com/BOXFoundation/boxd/vm/common/math"
	lru "github.com/hashicorp/golang-lru"
	"github.com/jbenet/goprocess"
	peer "github.com/libp2p/go-libp2p-peer"
	"golang.org/x/crypto/ripemd160"
)

const (
	free int32 = iota
	busy
)

// const defines constants
const (
	BlockMsgChBufferSize        = 1024
	EternalBlockMsgChBufferSize = 65536

	MaxTimeOffsetSeconds = 2 * 60 * 60
	MaxBlockSize         = 32000000
	CoinbaseLib          = 100
	maxBlockSigOpCnt     = 80000

	MaxBlocksPerSync = 1024

	metricsLoopInterval = 500 * time.Millisecond
	tokenIssueFilterKey = "token_issue"
	Threshold           = 32

	AddrFilterNumbers = 10000000
)

var logger = log.NewLogger("chain") // logger

var _ service.ChainReader = (*BlockChain)(nil)

// Config defines the configurations of chain
type Config struct {
	ContractBinPath string `mapstructure:"contract_bin_path"`
	ContractABIPath string `mapstructure:"contract_abi_path"`
}

// BlockChain define chain struct
type BlockChain struct {
	cfg           *Config
	notifiee      p2p.Net
	newblockMsgCh chan p2p.Message
	consensus     Consensus
	db            storage.Table
	genesis       *types.Block
	tail          *types.Block
	eternal       *types.Block
	tailState     *state.StateDB
	// some txs may be parents of other txs in the block
	proc                      goprocess.Process
	LongestChainHeight        uint32
	blockcache                *lru.Cache
	repeatedMintCache         *lru.Cache
	heightToBlock             *lru.Cache
	splitAddrFilter           bloom.Filter
	contractAddrFilter        bloom.Filter
	bus                       eventbus.Bus
	chainLock                 sync.RWMutex
	hashToOrphanBlock         map[crypto.HashType]*types.Block
	orphanBlockHashToChildren map[crypto.HashType][]*types.Block
	syncManager               SyncManager
	stateProcessor            *StateProcessor
	vmConfig                  vm.Config
	utxoSetCache              map[uint32]*UtxoSet
	receiptsCache             map[uint32]types.Receipts
	sectionMgr                *SectionManager
	status                    int32
}

// UpdateMsg sent from blockchain to, e.g., mempool
type UpdateMsg struct {
	// block connected/disconnected from main chain
	AttachBlocks []*types.Block
	DetachBlocks []*types.Block
}

// NewBlockChain return a blockchain.
func NewBlockChain(parent goprocess.Process, notifiee p2p.Net, db storage.Storage, bus eventbus.Bus, cfg *Config) (*BlockChain, error) {

	b := &BlockChain{
		cfg:                       cfg,
		notifiee:                  notifiee,
		newblockMsgCh:             make(chan p2p.Message, BlockMsgChBufferSize),
		proc:                      goprocess.WithParent(parent),
		hashToOrphanBlock:         make(map[crypto.HashType]*types.Block),
		orphanBlockHashToChildren: make(map[crypto.HashType][]*types.Block),
		utxoSetCache:              make(map[uint32]*UtxoSet),
		receiptsCache:             make(map[uint32]types.Receipts),
		bus:                       eventbus.Default(),
		status:                    free,
	}

	var err error
	b.blockcache, _ = lru.New(512)
	b.repeatedMintCache, _ = lru.New(512)
	b.heightToBlock, _ = lru.New(512)

	stateProcessor := NewStateProcessor(b)
	b.stateProcessor = stateProcessor

	if b.db, err = db.Table(BlockTableName); err != nil {
		return nil, err
	}

	if b.sectionMgr, err = NewSectionManager(b, db); err != nil {
		return nil, err
	}

	if err := b.LoadGenesisContract(); err != nil {
		logger.Error("Failed to load genesis contract ", err)
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

	if err := b.SetTailState(&b.tail.Header.RootHash, &b.tail.Header.UtxoRoot); err != nil {
		logger.Error("Failed to load tail state ", err)
		return nil, err
	}
	logger.Infof("load tail state with root: %s utxo root: %s",
		b.tail.Header.RootHash, b.tail.Header.UtxoRoot)

	b.LongestChainHeight = b.tail.Header.Height
	b.splitAddrFilter = loadAddrFilter(b.db, splitAddrBase.Bytes())
	logger.Infof("load split address bloom filter finished")
	b.contractAddrFilter = loadAddrFilter(b.db, contractAddrBase.Bytes())
	logger.Infof("load contract address bloom filter finished")

	return b, nil
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

// Consensus returns chain consensus.
func (chain *BlockChain) Consensus() Consensus {
	return chain.consensus
}

// DB returns chain db storage.
func (chain *BlockChain) DB() storage.Table {
	return chain.db
}

// Cfg returns chain config.
func (chain *BlockChain) Cfg() *Config {
	return chain.cfg
}

// StateProcessor returns chain stateProcessor.
func (chain *BlockChain) StateProcessor() *StateProcessor {
	return chain.stateProcessor
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

// IsBusy return if the chain is processing a block
func (chain *BlockChain) IsBusy() bool {
	v := atomic.LoadInt32(&chain.status)
	return v == busy
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
					// total, miss := chain.calMissRate()
					// if total != 0 {
					// 	metrics.MetricsBlockMissRateGauge.Update(int64(miss * 1000000 / total))
					// }
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
	if exist, ok := chain.repeatedMintCache.Get(block.Header.TimeStamp); ok &&
		block.Header.BookKeeper == (exist.(*types.Block)).Header.BookKeeper {
		return false
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
	logger.Infof("Prepare to process block. Hash: %s, Height: %d from %s",
		blockHash.String(), block.Header.Height, messageFrom.Pretty())

	// The block must not already exist in the main chain or side chains.
	if exists := chain.verifyExists(*blockHash); exists {
		logger.Warnf("The block already exists. Hash: %s, Height: %d", blockHash.String(), block.Header.Height)
		return core.ErrBlockExists
	}

	if ok := chain.verifyRepeatedMint(block); !ok {
		return core.ErrRepeatedMintAtSameTime
	}

	// if messageFrom != "" { // local block does not require validation
	// 	if err := chain.consensus.Verify(block); err != nil {
	// 		logger.Errorf("Failed to verify block. Hash: %s, Height: %d, Err: %s",
	// 			block.BlockHash(), block.Header.Height, err)
	// 		return err
	// 	}
	// }

	if err := validateBlock(block); err != nil {
		logger.Errorf("Failed to validate block. Hash: %s, Height: %d, Err: %s",
			block.BlockHash(), block.Header.Height, err)
		return err
	}
	prevHash := block.Header.PrevBlockHash
	if prevHashExists := chain.blockExists(prevHash); !prevHashExists {
		if chain.isInOrphanPool(*blockHash) {
			logger.Infof("block %s %d has been in orphan pool", blockHash, block.Header.Height)
			return nil
		}

		// Orphan block.
		logger.Infof("Adding orphan block %s %d with parent %s", blockHash,
			block.Header.Height, prevHash)
		chain.addOrphanBlock(block, *blockHash, prevHash)
		// chain.repeatedMintCache.Add(block.Header.TimeStamp, block)
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
		logger.Warnf("Failed to accept the block into the main chain. Err: %s", err)
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
	return chain.blockExists(blockHash)
}

func (chain *BlockChain) blockExists(blockHash crypto.HashType) bool {
	if chain.blockcache.Contains(blockHash) {
		return true
	}
	if block, _ := LoadBlockByHash(blockHash, chain.db); block != nil {
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

	// if messageFrom != "" { // local block does not require validation
	if err := chain.consensus.Verify(block); err != nil {
		logger.Errorf("Failed to verify block. Hash: %s, Height: %d, Err: %s",
			block.BlockHash(), block.Header.Height, err)
		return err
	}
	// }

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
	logger.Warnf("REORGANIZE: Block %s %d is causing a reorganization.", blockHash, block.Header.Height)

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
				logger.Warnf("process orphan %s %d error %s", orphan.BlockHash(), orphan.Header.Height, err)
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
	if block.Hash.IsEqual(chain.genesis.BlockHash()) {
		return chain.genesis
	}
	// check for genesis.
	if block.Header.PrevBlockHash.IsEqual(chain.genesis.BlockHash()) {
		return chain.genesis
	}
	if target, ok := chain.blockcache.Get(block.Header.PrevBlockHash); ok {
		return target.(*types.Block)
	}
	target, err := LoadBlockByHash(block.Header.PrevBlockHash, chain.db)
	if err != nil {
		return nil
	}
	return target
}

// tryConnectBlockToMainChain tries to append the passed block to the main chain.
// It enforces multiple rules such as double spends and script verification.
func (chain *BlockChain) tryConnectBlockToMainChain(block *types.Block, messageFrom peer.ID) error {

	logger.Infof("Try to connect block to main chain. Hash: %s, Height: %d",
		block.BlockHash(), block.Header.Height)

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
		if err := utxoSet.LoadBlockUtxos(block, true, chain.db); err != nil {
			return err
		}
		// Validate scripts here before utxoSet is updated; otherwise it may fail mistakenly
		if err := validateBlockScripts(utxoSet, block); err != nil {
			return err
		}
	}

	// Perform several checks on the inputs for each transaction.
	// Also accumulate the total fees.
	totalFees, err := validateBlockInputs(block.Txs, utxoSet)
	if err != nil {
		return err
	}

	return chain.executeBlock(block, utxoSet, totalFees, messageFrom)
}

func validateBlockInputs(txs []*types.Transaction, utxoSet *UtxoSet) (uint64, error) {
	var totalFees uint64
	for idx, tx := range txs {
		// skip coinbase tx
		if IsCoinBase(tx) || IsDynastySwitch(tx) {
			continue
		}
		txFee, err := ValidateTxInputs(utxoSet, tx)
		if err != nil {
			return 0, err
		}
		// Check contract tx from and fee
		contractVout, err := txlogic.CheckAndGetContractVout(tx)
		if err != nil {
			return 0, err
		}
		txHash, _ := tx.TxHash()
		if contractVout == nil {
			// check whether gas price is equal to TransferFee
			if txFee != core.TransferFee {
				bytes, _ := json.Marshal(tx)
				logger.Warnf("non-contract tx %s %s have wrong fee %d", txHash, string(bytes), txFee)
				return 0, core.ErrInvalidFee
			}
			// Check for overflow.
			lastTotalFees := totalFees
			totalFees += txFee
			if totalFees < lastTotalFees {
				return 0, core.ErrBadFees
			}
			continue
		}
		// smart contract tx.
		sc := script.NewScriptFromBytes(contractVout.ScriptPubKey)
		param, _, err := sc.ParseContractParams()
		if err != nil {
			return 0, err
		}
		if txFee != param.GasLimit*param.GasPrice ||
			idx == 0 && param.GasPrice != 0 ||
			idx > 0 && param.GasPrice != core.FixedGasPrice {
			logger.Warnf("contract tx %s have wrong fee %d gas price %d", txHash, txFee, param.GasPrice)
			return 0, core.ErrInvalidFee
		}
		if addr, err := FetchOutPointOwner(&tx.Vin[0].PrevOutPoint, utxoSet); err != nil ||
			*addr.Hash160() != *param.From {
			return 0, fmt.Errorf("contract tx from address mismatched")
		}
	}
	return totalFees, nil
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
		logger.Infof("DEBUG: update normal balance add %x %d", a[:], v)
		stateDB.AddBalance(a, new(big.Int).SetUint64(v))
	}
	for a, v := range bSub {
		logger.Infof("DEBUG: update normal balance sub %x %d", a[:], v)
		stateDB.SubBalance(a, new(big.Int).SetUint64(v))
	}
}

// UpdateContractUtxoState updates contract utxo in statedb
func (chain *BlockChain) UpdateContractUtxoState(statedb *state.StateDB, utxoSet *UtxoSet) error {
	for o := range utxoSet.contractUtxos {
		// address
		contractAddr := types.NewAddressHash(o.Hash[:])
		// serialize utxo wrap
		u := utxoSet.utxoMap[o]
		utxoBytes, err := SerializeUtxoWrap(u)
		if err != nil {
			return err
		}
		// update statedb utxo trie
		logger.Debugf("update utxo in statedb, account: %x, utxo: %d", contractAddr[:], u.Value())
		if err := statedb.UpdateUtxo(*contractAddr, utxoBytes); err != nil {
			return err
		}
	}
	return nil
}

func (chain *BlockChain) executeBlock(
	block *types.Block, utxoSet *UtxoSet, totalTxsFee uint64, messageFrom peer.ID,
) error {

	var (
		stateDB  *state.StateDB
		receipts types.Receipts
		err      error
	)

	blockCopy := block.Copy()
	// Split tx outputs if any
	splitTxs := chain.SplitBlockOutputs(blockCopy)

	// execute contract tx and update statedb for blocks from remote peers
	if messageFrom != "" {
		parent := chain.GetParentBlock(block)
		stateDB, err = state.New(&parent.Header.RootHash, &parent.Header.UtxoRoot, chain.db)
		if err != nil {
			logger.Error(err)
			return err
		}
		logger.Infof("new statedb with root: %s utxo root: %s block %s:%d",
			parent.Header.RootHash, parent.Header.UtxoRoot, block.BlockHash(), block.Header.Height)
		stateDB.AddBalance(block.Header.BookKeeper, new(big.Int).SetUint64(block.Txs[0].Vout[0].Value))

		// Save a deep copy before we potentially split the block's txs' outputs and mutate it
		if err := utxoSet.ApplyBlock(blockCopy); err != nil {
			return err
		}

		rcps, gasUsed, utxoTxs, err := chain.stateProcessor.Process(block, stateDB, utxoSet)
		if err != nil {
			logger.Error(err)
			return err
		}
		if err := chain.ValidateExecuteResult(block, utxoTxs, gasUsed, totalTxsFee, rcps); err != nil {
			return err
		}

		chain.UpdateNormalTxBalanceState(blockCopy, utxoSet, stateDB)
		// update genesis contract utxo in utxoSet
		op := types.NewOutPoint(types.NormalizeAddressHash(&ContractAddr), 0)
		genesisUtxoWrap := utxoSet.GetUtxo(op)
		genesisUtxoWrap.SetValue(genesisUtxoWrap.Value() + gasUsed)

		// apply internal txs.
		if len(block.InternalTxs) > 0 {
			if err := utxoSet.ApplyInternalTxs(block); err != nil {
				return err
			}
		}
		if err := chain.UpdateContractUtxoState(stateDB, utxoSet); err != nil {
			logger.Errorf("chain update utxo state error: %s", err)
			return err
		}

		root, utxoRoot, err := stateDB.Commit(false)
		if err != nil {
			logger.Errorf("stateDB commit failed: %s", err)
			return err
		}
		logger.Infof("statedb commit with root: %s, utxo root: %s block: %s:%d",
			root, utxoRoot, block.BlockHash(), block.Header.Height)
		if !root.IsEqual(&block.Header.RootHash) {
			return fmt.Errorf("Invalid state root in block header, have %s, got: %s, "+
				"block hash: %s height: %d", block.Header.RootHash, root, block.BlockHash(),
				block.Header.Height)
		}
		if (utxoRoot != nil && !utxoRoot.IsEqual(&block.Header.UtxoRoot)) &&
			!(utxoRoot == nil && block.Header.UtxoRoot == zeroHash) {
			return fmt.Errorf("Invalid utxo state root in block header, have %s, got: %s, "+
				"block hash: %s height: %d", block.Header.UtxoRoot, utxoRoot, block.BlockHash(),
				block.Header.Height)
		}
		if len(rcps) > 0 {
			receipts = rcps
		}
	} else {
		stateDB, _ = state.New(&block.Header.RootHash, &block.Header.UtxoRoot, chain.db)
		receipts = chain.receiptsCache[block.Header.Height]
		delete(chain.receiptsCache, block.Header.Height)
	}

	// check whether contract balance is identical in utxo and statedb
	for o, u := range utxoSet.ContractUtxos() {
		contractAddr := types.NewAddressHash(o.Hash[:])
		if u.Value() != stateDB.GetBalance(*contractAddr).Uint64() {
			address, _ := types.NewContractAddressFromHash(contractAddr[:])
			return fmt.Errorf("contract %s have ambiguous balance(%d in utxo and %d"+
				" in statedb)", address, u.Value(), stateDB.GetBalance(*contractAddr))
		}
	}

	if err := chain.writeBlockToDB(block, splitTxs, utxoSet, receipts); err != nil {
		return err
	}

	go chain.notifyRPCLogs(receipts)

	chain.tryToClearCache([]*types.Block{block}, nil)

	// notify mem_pool when chain update
	chain.notifyBlockConnectionUpdate([]*types.Block{block}, nil)

	// This block is now the end of the best chain.
	chain.ChangeNewTail(block)
	// set tail state
	return chain.SetTailState(&block.Header.RootHash, &block.Header.UtxoRoot)
}

func (chain *BlockChain) notifyRPCLogs(receipts types.Receipts) {
	logs := make(map[string][]*types.Log)
	for _, receipt := range receipts {
		if len(receipt.Logs) != 0 {
			contractAddr, err := types.NewContractAddressFromHash(receipt.ContractAddress.Bytes())
			if err != nil {
				logger.Errorf("Contract address convert failed. %s", receipt.ContractAddress.String())
				continue
			}
			if l, ok := logs[contractAddr.String()]; ok {
				l = append(l, receipt.Logs...)
			} else {
				logs[contractAddr.String()] = receipt.Logs
			}
		}
	}
	if len(logs) != 0 {
		chain.Bus().Publish(eventbus.TopicRPCSendNewLog, logs)
	}
}

func (chain *BlockChain) writeBlockToDB(
	block *types.Block, splitTxs map[crypto.HashType]*types.Transaction,
	utxoSet *UtxoSet, receipts types.Receipts,
) error {

	var batch = chain.db.NewBatch()
	defer batch.Close()

	if err := chain.StoreBlockWithIndex(block, batch); err != nil {
		return err
	}

	if len(receipts) > 0 {
		if err := chain.StoreReceipts(block.BlockHash(), receipts, batch); err != nil {
			return err
		}
		// write contract address to db
		for _, receipt := range receipts {
			if receipt.Deployed && !receipt.Failed {
				batch.Put(ContractAddrKey(receipt.ContractAddress[:]), nil)
				chain.contractAddrFilter.Add(receipt.ContractAddress[:])
			}
		}
	}

	// save tx index
	if err := chain.WriteTxIndex(block, splitTxs, batch); err != nil {
		return err
	}

	// save split tx
	if err := chain.StoreSplitTxs(splitTxs, batch); err != nil {
		return err
	}

	// store split addr index
	if err := chain.WriteSplitAddrIndex(block, batch); err != nil {
		logger.Error(err)
		return err
	}
	// save utxoset to database
	if err := utxoSet.WriteUtxoSetToDB(batch); err != nil {
		return err
	}
	// save current tail to database
	if err := chain.StoreTailBlock(block, batch); err != nil {
		return err
	}

	if err := batch.Write(); err != nil {
		logger.Errorf("Failed to batch write block. Hash: %s, Height: %d, Err: %s",
			block.BlockHash().String(), block.Header.Height, err.Error())
		return err
	}
	return nil
}

func checkInternalTxs(block *types.Block, utxoTxs []*types.Transaction) error {

	if len(utxoTxs) == 0 {
		block.InternalTxs = make([]*types.Transaction, 0)
		return nil
	}
	txsRoot := CalcTxsHash(utxoTxs)
	if !(&block.Header.InternalTxsRoot).IsEqual(txsRoot) {
		utxoTxsBytes, _ := json.MarshalIndent(utxoTxs, "", "  ")
		internalTxs, _ := json.MarshalIndent(block.InternalTxs, "", "  ")
		logger.Warnf("utxo txs generated: %s, internal txs in block: %v",
			string(utxoTxsBytes), string(internalTxs))
		logger.Warnf("utxo txs root: %s, internal txs root: %s", txsRoot, block.Header.InternalTxsRoot)
		return core.ErrInvalidInternalTxs
	}
	return nil
}

// ValidateExecuteResult validates evm execute result
func (chain *BlockChain) ValidateExecuteResult(
	block *types.Block, utxoTxs []*types.Transaction, usedGas, totalTxsFee uint64,
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

	netParams, err := chain.FetchNetParamsByHeight(chain.LongestChainHeight)
	if err != nil {
		return err
	}

	expectedCoinbaseOutput := netParams.BookKeeperReward.Uint64() + totalTxsFee
	// expectedCoinbaseOutput := CalcBlockSubsidy(block.Header.Height)
	if totalCoinbaseOutput != expectedCoinbaseOutput {
		logger.Errorf("coinbase transaction for block pays %d which is not equal to"+
			" expected value %d(totalTxsFee: %d)", totalCoinbaseOutput, expectedCoinbaseOutput,
			totalTxsFee)
		return core.ErrBadCoinbaseValue
	}
	// check receipt
	var receiptHash crypto.HashType
	if len(receipts) > 0 {
		receiptHash = *receipts.Hash()
	}
	if receiptHash != block.Header.ReceiptHash {
		logger.Warnf("receipt hash in block header: %s, now: %s, block hash: %s %d",
			block.Header.ReceiptHash, receiptHash, block.BlockHash(), block.Header.Height)
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

		logger.Warnf("No need to reorganize, because the forkpoint height[%d] is "+
			"lower than the latest eternal block height[%d].",
			forkpoint.Header.Height, chain.eternal.Header.Height)
		return nil
	}

	for _, detachBlock := range detachBlocks {
		stt0 := time.Now().UnixNano()
		if err := chain.tryDisConnectBlockFromMainChain(detachBlock); err != nil {
			logger.Errorf("Failed to disconnect block from main chain. Err: %v", err)
			panic("Failed to disconnect block from main chain")
		}
		stt1 := time.Now().UnixNano()
		logger.Infof("block %s %d disconnected from chain, time tracking: %d",
			detachBlock.BlockHash(), detachBlock.Header.Height, (stt1-stt0)/1e6)
	}

	for blockIdx := len(attachBlocks) - 1; blockIdx >= 0; blockIdx-- {
		stt0 := time.Now().UnixNano()
		attachBlock := attachBlocks[blockIdx]
		if err := chain.tryConnectBlockToMainChain(attachBlock, messageFrom); err != nil {
			// Roll back to the original state
			for idx := blockIdx + 1; idx < len(attachBlocks); idx++ {
				block := attachBlocks[idx]
				if err := chain.tryDisConnectBlockFromMainChain(block); err != nil {
					logger.Errorf("RollBack: Failed to disconnect block from main chain. Err: %v", err)
					panic("RollBack: Failed to disconnect block from main chain")
				}
			}
			for idx := len(detachBlocks) - 1; idx >= 0; idx-- {
				block := detachBlocks[idx]
				if err := chain.tryConnectBlockToMainChain(block, messageFrom); err != nil {
					logger.Errorf("RollBack: Failed to connect block to main chain. Err: %v", err)
					panic("RollBack: Failed to connect block to main chain")
				}
			}
			return err
		}
		stt1 := time.Now().UnixNano()
		logger.Infof("block %s %d connected to chain, time tracking: %d",
			attachBlock.BlockHash(), attachBlock.Header.Height, (stt1-stt0)/1e6)
	}

	logger.Infof("reorganize finished for block %s %d", block.BlockHash(), block.Header.Height)
	metrics.MetricsBlockRevertMeter.Mark(1)
	return nil
}

func (chain *BlockChain) tryDisConnectBlockFromMainChain(block *types.Block) error {
	dtt0 := time.Now().UnixNano()
	logger.Infof("Try to disconnect block from main chain. Hash: %s Height: %d",
		block.BlockHash(), block.Header.Height)

	// Save a deep copy before we potentially split the block's txs' outputs and mutate it
	blockCopy := block.Copy()

	// Split tx outputs if any
	splitTxs := chain.SplitBlockOutputs(blockCopy)
	dtt1 := time.Now().UnixNano()
	utxoSet := NewUtxoSet()
	if err := utxoSet.LoadBlockAllUtxos(blockCopy, false, chain.db); err != nil {
		return err
	}
	if err := utxoSet.RevertBlock(blockCopy, chain); err != nil {
		return err
	}
	// calc contract utxos, then check contract addr balance
	header := block.Header
	stateDB, _ := state.New(&header.RootHash, &header.UtxoRoot, chain.db)
	contractUtxos, err := MakeRollbackContractUtxos(block, stateDB, chain.db)
	if err != nil {
		logger.Error(err)
		return err
	}
	for k, v := range contractUtxos {
		logger.Debugf("make rollback contract utxos op: %+v, utxo wrap: %+v", k, v)
	}
	utxoSet.ImportUtxoMap(contractUtxos)

	// chain.db.EnableBatch()
	// defer chain.db.DisableBatch()
	batch := chain.db.NewBatch()
	defer batch.Close()

	dtt2 := time.Now().UnixNano()
	chain.db.Del(BlockHashKey(block.Header.Height))

	// chain.filterHolder.ResetFilters(block.Height)
	dtt3 := time.Now().UnixNano()
	// del tx index
	if err := chain.DelTxIndex(block, splitTxs, batch); err != nil {
		return err
	}

	// del split tx
	if err := chain.DelSplitTxs(splitTxs, batch); err != nil {
		return err
	}

	if err := chain.DeleteSplitAddrIndex(block, batch); err != nil {
		return err
	}
	dtt4 := time.Now().UnixNano()
	if err := utxoSet.WriteUtxoSetToDB(batch); err != nil {
		return err
	}
	dtt5 := time.Now().UnixNano()

	// del receipt
	batch.Del(ReceiptKey(block.BlockHash()))
	// store previous block as tail
	newTail := chain.GetParentBlock(block)
	if err := chain.StoreTailBlock(newTail, batch); err != nil {
		logger.Error(err)
		return err
	}

	if err := batch.Write(); err != nil {
		logger.Errorf("Failed to batch write block. Hash: %s, Height: %d, Err: %s",
			block.BlockHash().String(), block.Header.Height, err.Error())
	}
	dtt6 := time.Now().UnixNano()
	chain.tryToClearCache(nil, []*types.Block{block})

	// notify mem_pool when chain update
	chain.notifyBlockConnectionUpdate(nil, []*types.Block{block})
	dtt7 := time.Now().UnixNano()
	// This block is now the end of the best chain.
	chain.ChangeNewTail(newTail)
	if needToTracking((dtt1-dtt0)/1e6, (dtt2-dtt1)/1e6, (dtt3-dtt2)/1e6, (dtt4-dtt3)/1e6, (dtt5-dtt4)/1e6, (dtt6-dtt5)/1e6, (dtt7-dtt6)/1e6) {
		logger.Infof("dtt Time tracking: dtt0` = %d dtt1` = %d dtt2` = %d dtt3` = %d dtt4` = %d dtt5` = %d dtt6` = %d", (dtt1-dtt0)/1e6, (dtt2-dtt1)/1e6, (dtt3-dtt2)/1e6, (dtt4-dtt3)/1e6, (dtt5-dtt4)/1e6, (dtt6-dtt5)/1e6, (dtt7-dtt6)/1e6)
	}

	return chain.SetTailState(&newTail.Header.RootHash, &newTail.Header.UtxoRoot)
}

// StoreTailBlock store tail block to db.
func (chain *BlockChain) StoreTailBlock(block *types.Block, db storage.Writer) error {
	data, err := block.Marshal()
	if err != nil {
		return err
	}
	return db.Put(TailKey, data)
}

// TailBlock return chain tail block.
func (chain *BlockChain) TailBlock() *types.Block {
	return chain.tail
}

// TailState returns chain tail statedb
func (chain *BlockChain) TailState() *state.StateDB {
	return chain.tailState.Copy()
}

// SetTailState returns chain tail statedb
func (chain *BlockChain) SetTailState(root, utxoRoot *crypto.HashType) error {
	stateDB, err := state.New(root, utxoRoot, chain.db)
	if err != nil {
		return err
	}
	chain.tailState = stateDB
	return nil
}

// Genesis return chain genesis block.
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
		go func() {
			if err := chain.sectionMgr.AddBloom(eternal.Header.Height, eternal.Header.Bloom); err != nil {
				logger.Error(err)
			}
		}()
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

// LoadGenesisContract load genesis contract info.
func (chain *BlockChain) LoadGenesisContract() error {
	if ContractBin != nil && ContractAbi != nil {
		return nil
	}
	bin, err := readBin(chain.cfg.ContractBinPath)
	if err != nil {
		return err
	}
	abi, err := ReadAbi(chain.cfg.ContractABIPath)
	if err != nil {
		return err
	}
	ContractBin, ContractAbi = bin, abi
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
		adminAddr, err := types.NewAddress(Admin)
		if err != nil {
			return nil, err
		}

		ContractAddr = *types.CreateAddress(*adminAddr.Hash160(), 1)
		logger.Infof("load genesis contract addr: %v", ContractAddr)

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
	// code, err := readBin(chain.cfg.ContractBinPath)
	vmTx := types.NewVMTransaction(big.NewInt(0), big.NewInt(0), 1e8, 1, nil, types.ContractCreationType, ContractBin)
	adminAddr, err := types.NewAddress(Admin)
	if err != nil {
		return nil, err
	}
	vmTx.WithFrom(adminAddr.Hash160())
	ctx := NewEVMContext(vmTx, genesis.Header, chain)
	logConfig := vm.LogConfig{}
	structLogger := vm.NewStructLogger(&logConfig)
	vmConfig := vm.Config{Debug: true, Tracer: structLogger /*, JumpTable: vm.NewByzantiumInstructionSet()*/}

	evm := vm.NewEVM(ctx, stateDB, vmConfig)
	_, contractAddr, _, vmerr := evm.Create(vm.AccountRef(*vmTx.From()), vmTx.Data(), vmTx.Gas(), big.NewInt(0), false)
	if vmerr != nil {
		return nil, vmerr
	}
	ContractAddr = contractAddr
	addressHash := types.NormalizeAddressHash(&contractAddr)
	outPoint := types.NewOutPoint(addressHash, 0)
	utxoWrap := types.NewUtxoWrap(0, []byte{}, 0)
	utxoBytes, _ := SerializeUtxoWrap(utxoWrap)
	stateDB.UpdateUtxo(ContractAddr, utxoBytes)
	utxoSet.utxoMap[*outPoint] = utxoWrap

	chain.UpdateNormalTxBalanceState(&genesis, utxoSet, stateDB)
	root, utxoRoot, err := stateDB.Commit(false)
	if err != nil {
		return nil, err
	}
	logger.Infof("genesis root hash: %s, utxo root hash: %s", root, utxoRoot)
	genesis.Header.RootHash = *root
	if utxoRoot != nil {
		genesis.Header.UtxoRoot = *utxoRoot
	}

	batch := chain.db.NewBatch()
	defer batch.Close()
	utxoSet.WriteUtxoSetToDB(batch)
	if err := chain.WriteTxIndex(&genesis, nil, batch); err != nil {
		return nil, err
	}
	genesisBin, err := genesis.Marshal()
	if err != nil {
		return nil, err
	}
	batch.Put(BlockKey(genesis.BlockHash()), genesisBin)
	batch.Put(GenesisKey, genesisBin)
	if err := batch.Write(); err != nil {
		return nil, err
	}
	return &genesis, nil
}

func readBin(filename string) ([]byte, error) {
	code, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	return hexutil.MustDecode("0x" + strings.TrimSpace(string(code))), nil
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
func LoadBlockByHash(hash crypto.HashType, reader storage.Reader) (*types.Block, error) {

	blockBin, err := reader.Get(BlockKey(&hash))
	if err != nil {
		return nil, fmt.Errorf("db get with block hash %s error %s", hash, err)
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
	bytes, err := chain.db.Get(BlockHashKey(height))
	if err != nil {
		return nil, fmt.Errorf("db get with block height %d error %s", height, err)
	}
	if bytes == nil {
		return nil, core.ErrBlockIsNil
	}
	hash := new(crypto.HashType)
	copy(hash[:], bytes)
	block, err := LoadBlockByHash(*hash, chain.db)
	if err != nil {
		logger.Error(err)
		return nil, err
	}

	return block, nil
}

// GetLogs filter logs.
func (chain *BlockChain) GetLogs(from, to uint32, topicslist [][][]byte) ([]*types.Log, error) {
	return chain.sectionMgr.GetLogs(from, to, topicslist)
}

// FilterLogs filter logs by addrs and topicslist.
func (chain *BlockChain) FilterLogs(logs []*types.Log, topicslist [][][]byte) ([]*types.Log, error) {

	// topicslist = [][][]byte{}
	// var data []byte

	// topicslist[0] = make([][]byte, len(addrs))
	// for i, addr := range addrs {
	// 	topicslist[0][i] = addr
	// }

	// for i, topics := range topicslist {
	// 	for j, topic := range topics {
	// 		copy(topicslist[i+1][j], topic)
	// 	}
	// }

	return chain.sectionMgr.filterLogs(logs, topicslist)
}

// GetBlockLogs get logs by block hash.
func (chain *BlockChain) GetBlockLogs(hash *crypto.HashType) ([]*types.Log, error) {
	bin, err := chain.db.Get(ReceiptKey(hash))
	if err != nil {
		return nil, err
	}

	receipts := new(types.Receipts)
	if err := receipts.Unmarshal(bin); err != nil {
		return nil, err
	}
	logs := []*types.Log{}
	for _, receipt := range *receipts {
		for _, log := range receipt.Logs {
			log.BlockHash.SetBytes(hash.Bytes())
			logs = append(logs, log)
		}
	}
	return logs, nil
}

// NewEvmContextForLocalCallByHeight new a evm context for local call by block height.
func (chain *BlockChain) NewEvmContextForLocalCallByHeight(msg types.Message, height uint32) (*vm.EVM, func() error, error) {
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

// GetStateDbByHeight get statedb by block height.
func (chain *BlockChain) GetStateDbByHeight(height uint32) (*state.StateDB, error) {
	if height == 0 {
		return chain.TailState(), nil
	}
	block, err := chain.LoadBlockByHeight(height)
	if block == nil || err != nil {
		return nil, err
	}
	state, err := state.New(&block.Header.RootHash, &block.Header.UtxoRoot, chain.db)
	if state == nil || err != nil {
		return nil, err
	}
	return state, nil
}

// StoreBlockWithIndex store block to db in batch mod.
func (chain *BlockChain) StoreBlockWithIndex(block *types.Block, db storage.Writer) error {

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
func (chain *BlockChain) StoreReceipts(hash *crypto.HashType, receipts types.Receipts, db storage.Writer) error {

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
	if err != nil || len(txIndex) == 0 {
		return nil, nil, fmt.Errorf("db get txIndex with tx hash %s error %v", hash, err)
	}
	height, index, err := UnmarshalTxIndex(txIndex)
	if err != nil {
		logger.Errorf("load block info by tx %s unmarshal tx index %x error %s", hash, txIndex, err)
		return nil, nil, err
	}
	block, err := chain.LoadBlockByHeight(height)
	if err != nil {
		logger.Warn(err)
		return nil, nil, err
	}

	idx := int(index)
	var tx *types.Transaction
	if idx < len(block.Txs) {
		tx = block.Txs[idx]
	} else if idx < len(block.Txs)+len(block.InternalTxs) {
		tx = block.InternalTxs[idx-len(block.Txs)]
	} else {
		txBin, err := chain.db.Get(TxKey(&hash))
		if err != nil {
			return nil, nil, fmt.Errorf("db get tx with hash %s error %s", hash, err)
		}
		if txBin == nil {
			return nil, nil, errors.New("failed to load split tx with hash")
		}
		tx = new(types.Transaction)
		if err := tx.Unmarshal(txBin); err != nil {
			return nil, nil, err
		}
	}
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
func (chain *BlockChain) WriteTxIndex(block *types.Block, splitTxs map[crypto.HashType]*types.Transaction, db storage.Writer) error {

	allTxs := block.Txs
	if len(block.InternalTxs) > 0 {
		allTxs = append(allTxs, block.InternalTxs...)
	}
	for _, tx := range splitTxs {
		allTxs = append(allTxs, tx)
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
func (chain *BlockChain) StoreSplitTxs(
	splitTxs map[crypto.HashType]*types.Transaction, db storage.Writer,
) error {
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
func (chain *BlockChain) DelTxIndex(
	block *types.Block, splitTxs map[crypto.HashType]*types.Transaction, db storage.Writer,
) error {

	allTxs := block.Txs
	if len(block.InternalTxs) > 0 {
		allTxs = append(allTxs, block.InternalTxs...)
	}
	for _, tx := range splitTxs {
		allTxs = append(allTxs, tx)
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
func (chain *BlockChain) DelSplitTxs(splitTxs map[crypto.HashType]*types.Transaction, db storage.Writer) error {
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
		block, err := LoadBlockByHash(*hashes[index], chain.db)
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

	block, err := LoadBlockByHash(hash, chain.db)
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
	block, err := LoadBlockByHash(hash, chain.db)
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
	vout := make([]*types.TxOut, 0)
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
func (chain *BlockChain) splitTxOutput(txOut *types.TxOut) []*types.TxOut {
	// return the output itself if it cannot be split
	txOuts := []*types.TxOut{txOut}
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
	txOuts = make([]*types.TxOut, 0)
	n := len(sai.addrs)

	totalWeight := uint64(0)
	for i := 0; i < n; i++ {
		totalWeight += uint64(sai.weights[i])
	}

	totalValue := uint64(0)
	for i := 0; i < n; i++ {
		// An composite address splits value per its weight
		value := txOut.Value * uint64(sai.weights[i]) / totalWeight
		if i == n-1 {
			// Last address gets the remainder value in case value is indivisible
			value = txOut.Value - totalValue
		} else {
			totalValue += value
		}
		childTxOut := &types.TxOut{
			Value:        value,
			ScriptPubKey: *script.PayToPubKeyHashScript(sai.addrs[i][:]),
		}
		// recursively find if the child tx output is splittable
		childTxOuts := chain.splitTxOutput(childTxOut)
		txOuts = append(txOuts, childTxOuts...)
	}

	return txOuts
}

// GetTxReceipt returns a tx receipt by using given tx hash.
func (chain *BlockChain) GetTxReceipt(txHash *crypto.HashType) (*types.Receipt, error) {
	b, _, err := chain.LoadBlockInfoByTxHash(*txHash)
	if err != nil {
		logger.Warn(err)
		return nil, err
	}
	value, err := chain.db.Get(ReceiptKey(b.BlockHash()))
	if err != nil {
		return nil, err
	}
	if len(value) == 0 {
		return nil, fmt.Errorf("receipt for block %s %d not found in db", b.BlockHash(), b.Header.Height)
	}
	receipts := new(types.Receipts)
	if err := receipts.Unmarshal(value); err != nil {
		return nil, err
	}
	for _, receipt := range *receipts {
		for _, log := range receipt.Logs {
			log.BlockHash.SetBytes(b.Hash.Bytes())
		}
	}
	return receipts.GetTxReceipt(txHash), nil
}

type splitAddrInfo struct {
	addrs   []*types.AddressHash
	weights []uint32
}

// Marshall Serialize splitAddrInfo into bytes
func (s *splitAddrInfo) Marshall() ([]byte, error) {
	if len(s.addrs) != len(s.weights) {
		return nil, fmt.Errorf("invalid split addr info")
	}
	res := make([]byte, 0, len(s.addrs)*(ripemd160.Size+4))
	for i := 0; i < len(s.addrs); i++ {
		res = append(res, s.addrs[i][:]...)
		weightByte := make([]byte, 4)
		binary.LittleEndian.PutUint32(weightByte, s.weights[i])
		res = append(res, weightByte...)
	}
	return res, nil
}

// Unmarshall parse splitAddrInfo from bytes
func (s *splitAddrInfo) Unmarshall(data []byte) error {
	minLenght := ripemd160.Size + 4
	if len(data)%minLenght != 0 {
		return fmt.Errorf("invalid byte length")
	}
	count := len(data) / minLenght
	addrs := make([]*types.AddressHash, 0, count)
	weights := make([]uint32, 0, count)
	for i := 0; i < count; i++ {
		offset := i * minLenght
		addr, err := types.NewAddressPubKeyHash(data[offset : offset+ripemd160.Size])
		if err != nil {
			return err
		}
		weight := binary.LittleEndian.Uint32(data[offset+ripemd160.Size : offset+minLenght])
		addrs = append(addrs, addr.Hash160())
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
func (chain *BlockChain) WriteSplitAddrIndex(block *types.Block, db storage.Writer) error {
	for _, tx := range block.Txs {
		for i, vout := range tx.Vout {
			sc := *script.NewScriptFromBytes(vout.ScriptPubKey)
			if sc.IsSplitAddrScript() {
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
				txHash, _ := tx.TxHash()
				addr := txlogic.MakeSplitAddress(txHash, uint32(i), addrs, weights)
				k := SplitAddrKey(addr.Hash())
				db.Put(k, dataBytes)
				chain.splitAddrFilter.Add(addr.Hash())
				logger.Infof("New Split Address %x created", addr.Hash())
			}
		}
	}
	return nil
}

// DeleteSplitAddrIndex remove split address index from both db and cache
func (chain *BlockChain) DeleteSplitAddrIndex(block *types.Block, db storage.Writer) error {
	for _, tx := range block.Txs {
		for i, vout := range tx.Vout {
			sc := *script.NewScriptFromBytes(vout.ScriptPubKey)
			if sc.IsSplitAddrScript() {
				addrs, weights, err := sc.ParseSplitAddrScript()
				if err != nil {
					return err
				}
				txHash, _ := tx.TxHash()
				addr := txlogic.MakeSplitAddress(txHash, uint32(i), addrs, weights)
				k := SplitAddrKey(addr.Hash())
				db.Del(k)
				logger.Debugf("Remove Split Address: %s", addr.String())
			}
		}
	}
	return nil
}

func (u *UtxoSet) calcNormalTxBalanceChanges(block *types.Block) (add, sub BalanceChangeMap) {
	add = make(BalanceChangeMap)
	sub = make(BalanceChangeMap)
	for _, v := range block.Txs {
		if !txlogic.HasContractVout(v) {
			for _, vout := range v.Vout {
				sc := script.NewScriptFromBytes(vout.ScriptPubKey)
				// calc balance for account state, here only EOA (external owned account)
				// have balance state
				if !sc.IsPayToPubKeyHash() {
					continue
				}
				address, _ := sc.ExtractAddress()
				addr := address.Hash160()
				add[*addr] += vout.Value
				//logger.Warnf("add addr: %s, value: %d", addr, vout.Value)
			}
		}
	}

	for o, w := range u.utxoMap {
		_, exists := u.normalTxUtxoSet[o]
		if !exists {
			continue
		}
		sc := script.NewScriptFromBytes(w.Script())
		// calc balance for account state, here only EOA (external owned account)
		// have balance state
		if !sc.IsPayToPubKeyHash() {
			continue
		}
		address, _ := sc.ExtractAddress()
		addr := address.Hash160()
		if w.IsSpent() {
			sub[*addr] += w.Value()
		}
	}
	return
}

// NOTE: key in db must be pattern "/abc/def"
func loadAddrFilter(reader storage.Reader, addrPrefix []byte) bloom.Filter {
	filter := bloom.NewFilter(AddrFilterNumbers, bloom.DefaultConflictRate)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for key := range reader.IterKeysWithPrefix(ctx, addrPrefix) {
		splits := bytes.Split(key, []byte{'/'})
		if len(splits) != 3 {
			logger.Errorf("addr filter key %v in db is not 3 sections", key)
			continue
		}
		addrHash, err := hex.DecodeString(string(splits[2]))
		if err != nil {
			logger.Error(err)
			continue
		}
		filter.Add(addrHash)
	}
	return filter
}

func (chain *BlockChain) calcScores() ([]*big.Int, error) {
	return nil, nil
}

// MakeInternalContractTx creates a coinbase give bookkeeper address and block height
func (chain *BlockChain) MakeInternalContractTx(
	from types.AddressHash, amount uint64, nonce uint64, blockHeight uint32,
	method string, params ...interface{},
) (*types.Transaction, error) {
	abiObj, err := ReadAbi(chain.cfg.ContractABIPath)
	if err != nil {
		return nil, err
	}
	var code []byte
	if len(params) == 0 {
		code, err = abiObj.Pack(method)
	} else {
		code, err = abiObj.Pack(method, params)
	}
	if err != nil {
		return nil, err
	}

	coinbaseScriptSig := script.StandardCoinbaseSignatureScript(blockHeight)
	contractAddr, err := types.NewContractAddressFromHash(ContractAddr[:])
	if err != nil {
		return nil, err
	}
	vout, err := txlogic.MakeContractCallVout(&from, contractAddr.Hash160(), amount, 1e9, 0, nonce)
	if err != nil {
		return nil, err
	}
	var index uint32
	// if method == "calcBonus" {
	// 	index = sysmath.MaxUint32
	// } else {
	// 	index = 0
	// }
	switch method {
	case CalcBonus:
		index = sysmath.MaxUint32
	case ExecBonus:
		index = sysmath.MaxUint32 - 1
	case CalcScore:
		index = sysmath.MaxUint32 - 2
	}

	tx := &types.Transaction{
		Version: 1,
		Vin: []*types.TxIn{
			{
				PrevOutPoint: types.OutPoint{
					Hash:  zeroHash,
					Index: index,
				},
				ScriptSig: *coinbaseScriptSig,
				Sequence:  sysmath.MaxUint32,
			},
		},
		Vout: []*types.TxOut{vout},
	}
	tx.WithData(types.ContractDataType, code)
	logger.Infof("InternalContractTx from: %s nonce: %d to %s amount: %d", from, nonce, contractAddr, amount)
	return tx, nil
}

// IsContractAddr check addr whether is contract address
func IsContractAddr(
	addr *types.AddressHash, filter bloom.Filter, db storage.Reader, utxoSet *UtxoSet,
) bool {
	if addr == nil {
		return false
	}
	// may be the contract address is generated in this block
	for op := range utxoSet.contractUtxos {
		if *types.NormalizeAddressHash(addr) == op.Hash {
			return true
		}
	}
	// check in bloom filter
	if !filter.Matches(addr[:]) {
		// Definitely not a contract address
		return false
	}
	// May be a contract address, query db to find out
	if ok, err := db.Has(ContractAddrKey(addr[:])); err != nil || !ok {
		return false
	}
	return true
}
