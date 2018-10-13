// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package chain

import (
	"math"
	"sort"
	"time"

	lru "github.com/hashicorp/golang-lru"

	"github.com/BOXFoundation/boxd/core"
	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/core/utils"
	"github.com/BOXFoundation/boxd/crypto"
	"github.com/BOXFoundation/boxd/log"
	"github.com/BOXFoundation/boxd/p2p"
	"github.com/BOXFoundation/boxd/script"
	"github.com/BOXFoundation/boxd/storage"
	"github.com/BOXFoundation/boxd/util"
	"github.com/jbenet/goprocess"
)

// const defines constants
const (
	BlockMsgChBufferSize = 1024

	Tail = "tail_block"

	BlockTableName = "core_block"
	TxTableName    = "core_tx"

	// MaxTimeOffsetSeconds is the maximum number of seconds a block time
	// is allowed to be ahead of the current time.  This is currently 2 hours.
	MaxTimeOffsetSeconds = 2 * 60 * 60

	// MaxBlockSize is the maximum number of bytes within a block
	MaxBlockSize = 32000000

	// CoinbaseLib is the number of blocks required before newly mined coins (coinbase transactions) can be spent.
	CoinbaseLib int32 = 100

	// maxBlockSigOpCnt is the maximum number of signature operations in a block.
	maxBlockSigOpCnt = 80000

	// LockTimeThreshold is the number below which a lock time is
	// interpreted to be a block number. Since an average of one block
	// is generated per 10 minutes, this allows blocks for about 9,512 years.
	LockTimeThreshold = 5e8 // Tue Nov 5 00:53:20 1985 UTC

	// medianTimeBlocks is the number of previous blocks which should be
	// used to calculate the median time used to validate block timestamps.
	medianTimeBlocks = 11

	// SequenceLockTimeMask is a mask that extracts the relative locktime
	// when masked against the transaction input sequence number.
	sequenceLockTimeMask = 0x0000ffff

	// SequenceLockTimeIsSeconds is a flag that if set on a transaction
	// input's sequence number, the relative locktime has units of 512 seconds.
	sequenceLockTimeIsSeconds = 1 << 22

	// sequenceLockTimeGranularity is the defined time based granularity
	// for seconds-based relative time locks. When converting from seconds
	// to a sequence number, the value is right shifted by this amount,
	// therefore the granularity of relative time locks in 512 or 2^9
	// seconds. Enforced relative lock times are multiples of 512 seconds.
	sequenceLockTimeGranularity = 9

	// nminedHeight is the height used for the "block" height field of the
	// contextual transaction information provided in a transaction store
	// when it has not yet been mined into a block.
	unminedHeight = 0x7fffffff
)

var logger = log.NewLogger("chain") // logger

func init() {
}

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

func (chain *BlockChain) loadGenesis() (*types.Block, error) {

	if ok, _ := chain.dbBlock.Has(genesisHash[:]); ok {
		genesisBlockFromDb, err := chain.LoadBlockByHashFromDb(genesisHash)
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

// LoadBlockByHashFromDb load block by hash from db.
func (chain *BlockChain) LoadBlockByHashFromDb(hash crypto.HashType) (*types.Block, error) {

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

// StoreBlockToDb store block to db.
func (chain *BlockChain) StoreBlockToDb(block *types.Block) error {
	data, err := block.Marshal()
	if err != nil {
		return err
	}
	hash := block.BlockHash()
	return chain.dbBlock.Put((*hash)[:], data)
}

// Run launch blockchain.
func (chain *BlockChain) Run() {
	chain.subscribeMessageNotifiee(chain.notifiee)
	go chain.loop()
}

func (chain *BlockChain) subscribeMessageNotifiee(notifiee p2p.Net) {
	notifiee.Subscribe(p2p.NewNotifiee(p2p.NewBlockMsg, chain.newblockMsgCh))
}

func (chain *BlockChain) loop() {
	logger.Info("Waitting for new block message...")
	for {
		select {
		case msg := <-chain.newblockMsgCh:
			chain.processBlockMsg(msg)
		case <-chain.proc.Closing():
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

func (chain *BlockChain) blockExists(blockHash crypto.HashType) bool {
	if chain.cache.Contains(blockHash) {
		return true
	}
	block, err := chain.LoadBlockByHashFromDb(blockHash)
	if err != nil || block == nil {
		return false
	}
	return true
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

	// Perform preliminary sanity checks on the block and its transactions.
	if err := checkBlockWithoutContext(block, util.NewMedianTime()); err != nil {
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

// checkBlockWithContext validates the block, taking into account its position relative to the chain.
func (chain *BlockChain) checkBlockWithContext(block *types.Block) error {
	blockTime := block.Header.TimeStamp

	// Ensure all transactions are finalized.
	for _, tx := range block.Txs {
		if !IsTxFinalized(tx, block.Height, blockTime) {
			txHash, _ := tx.TxHash()
			logger.Errorf("block contains unfinalized transaction %v", txHash)
			return core.ErrUnfinalizedTx
		}
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
	target, err := chain.LoadBlockByHashFromDb(block.Header.PrevBlockHash)
	if err != nil {
		return nil
	}
	return target
}

func (chain *BlockChain) calcPastMedianTime(block *types.Block) time.Time {

	timestamps := make([]int64, medianTimeBlocks)
	i := 0
	for iterBlock := block; i < medianTimeBlocks && iterBlock != nil; i++ {
		timestamps[i] = iterBlock.Header.TimeStamp
		iterBlock = chain.getParentBlock(iterBlock)
	}
	timestamps = timestamps[:i]
	sort.Sort(timeSorter(timestamps))
	medianTimestamp := timestamps[i/2]
	return time.Unix(medianTimestamp, 0)
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

// LockTime represents the relative lock-time in seconds
type LockTime struct {
	Seconds     int64
	BlockHeight int32
}

func (chain *BlockChain) calcLockTime(utxoSet *utils.UtxoSet, block *types.Block, tx *types.Transaction) (*LockTime, error) {

	lockTime := &LockTime{Seconds: -1, BlockHeight: -1}

	// lock-time does not apply to coinbase tx.
	if utils.IsCoinBase(tx) {
		return lockTime, nil
	}

	for _, txIn := range tx.Vin {
		utxo := utxoSet.FindUtxo(txIn.PrevOutPoint)
		if utxo == nil {
			logger.Errorf("Failed to calc lockTime, output %+v does not exist or has already been spent", txIn.PrevOutPoint)
			return lockTime, core.ErrMissingTxOut
		}

		utxoHeight := utxo.BlockHeight
		sequenceNum := txIn.Sequence
		relativeLock := int64(sequenceNum & sequenceLockTimeMask)

		if sequenceNum&sequenceLockTimeIsSeconds == sequenceLockTimeIsSeconds {

			prevInputHeight := utxoHeight - 1
			if prevInputHeight < 0 {
				prevInputHeight = 0
			}
			ancestor := chain.ancestor(block, prevInputHeight)
			medianTime := chain.calcPastMedianTime(ancestor)

			timeLockSeconds := (relativeLock << sequenceLockTimeGranularity) - 1
			timeLock := medianTime.Unix() + timeLockSeconds
			if timeLock > lockTime.Seconds {
				lockTime.Seconds = timeLock
			}
		} else {

			blockHeight := utxoHeight + int32(relativeLock-1)
			if blockHeight > lockTime.BlockHeight {
				lockTime.BlockHeight = blockHeight
			}
		}
	}

	return lockTime, nil
}

func sequenceLockActive(timeLock *LockTime, blockHeight int32, medianTimePast time.Time) bool {
	return timeLock.Seconds < medianTimePast.Unix() && timeLock.BlockHeight < blockHeight
}

func validateTxs(txValidateItems []*script.TxValidateItem) error {
	// TODO: execute and verify script
	return nil
}

func checkBlockScripts(block *types.Block) error {

	numInputs := 0
	for _, tx := range block.Txs {
		numInputs += len(tx.Vin)
	}
	txValItems := make([]*script.TxValidateItem, 0, numInputs)
	// Skip coinbases.
	for _, tx := range block.Txs[1:] {
		for txInIdx, txIn := range tx.Vin {
			txVI := &script.TxValidateItem{
				TxInIndex: txInIdx,
				TxIn:      txIn,
				Tx:        tx,
			}
			txValItems = append(txValItems, txVI)
		}
	}

	// Validate all of the inputs.
	start := time.Now()
	if err := validateTxs(txValItems); err != nil {
		return err
	}
	elapsed := time.Since(start)

	logger.Debugf("block %v took %v to verify", block.BlockHash(), elapsed)
	return nil
}

// tryConnectBlockToMainChain tries to append the passed block to the main chain.
// It enforces multiple rules such as double spends and script verification.
func (chain *BlockChain) tryConnectBlockToMainChain(block *types.Block, utxoSet *utils.UtxoSet) error {
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
		txFee, err := utils.ValidateTxInputs(utxoSet, tx, block.Height)
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
	expectedCoinbaseOutput := utils.CalcBlockSubsidy(block.Height) + totalFees
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

	// Verify script evaluates to true.
	// Doing this last helps since it's CPU intensive.
	if err := checkBlockScripts(block); err != nil {
		return err
	}

	// utxoSet.StoreToDB()

	// This block is now the end of the best chain.
	chain.SetTailBlock(block, utxoSet)

	// Notify others such as mempool.
	chain.notifyBlockConnectionUpdate(block, true)
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

func (chain *BlockChain) revertBlock(block *types.Block, utxoSet *utils.UtxoSet) error {

	if err := utxoSet.LoadBlockUtxos(block, chain.DbTx); err != nil {
		return err
	}

	if err := utxoSet.RevertBlock(block); err != nil {
		return err
	}

	return chain.notifyBlockConnectionUpdate(block, false)
}

func (chain *BlockChain) applyBlock(block *types.Block, utxoSet *utils.UtxoSet) error {

	if err := utxoSet.LoadBlockUtxos(block, chain.DbTx); err != nil {
		return err
	}

	if err := utxoSet.ApplyBlock(block); err != nil {
		return err
	}

	return chain.notifyBlockConnectionUpdate(block, true)
}

func (chain *BlockChain) notifyBlockConnectionUpdate(block *types.Block, connected bool) error {
	chainUpdateMsg := utils.ChainUpdateMsg{
		Connected: connected,
		Block:     block,
	}
	chain.notifiee.Notify(utils.NewLocalMessage(p2p.ChainUpdateMsg, chainUpdateMsg))
	return nil
}

func (chain *BlockChain) reorganize(block *types.Block, utxoSet *utils.UtxoSet) error {
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

// IsTxFinalized checks if a transaction is finalized.
func IsTxFinalized(tx *types.Transaction, blockHeight int32, blockTime int64) bool {
	// The tx is finalized if lock time is 0.
	lockTime := tx.LockTime
	if lockTime == 0 {
		return true
	}

	// When lock time field is less than the threshold, it is a block height.
	// Otherwise it is a timestamp.
	blockTimeOrHeight := int64(0)
	if lockTime < LockTimeThreshold {
		blockTimeOrHeight = int64(blockHeight)
	} else {
		blockTimeOrHeight = blockTime
	}
	if lockTime < blockTimeOrHeight {
		return true
	}

	// A tx is still considered finalized if all input sequence numbers are maxed out.
	for _, txIn := range tx.Vin {
		if txIn.Sequence != math.MaxUint32 {
			return false
		}
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

	// Validate the block within chain context.
	if err := chain.checkBlockWithContext(block); err != nil {
		return false, err
	}

	if err := chain.StoreBlockToDb(block); err != nil {
		return false, err
	}
	chain.cache.Add(*blockHash, block)

	// Connect the passed block to the main or side chain.
	// There are 3 cases.
	parentHash := &block.Header.PrevBlockHash
	tailHash := chain.TailBlock().BlockHash()
	utxoSet := utils.NewUtxoSet()
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

// checkBlockHeaderWithoutContext performs context-free checks on a block header.
func checkBlockHeaderWithoutContext(header *types.BlockHeader, timeSource util.MedianTimeSource) error {
	// TODO: PoW check here
	// err := checkProofOfWork(header, powLimit, flags)

	// A block timestamp must not have a greater precision than one second.
	// Go time.Time values support
	// nanosecond precision whereas the consensus rules only apply to
	// seconds and it's much nicer to deal with standard Go time values
	// instead of converting to seconds everywhere.
	timestamp := time.Unix(0, header.TimeStamp)

	// Ensure the block time is not too far in the future.
	maxTimestamp := timeSource.AdjustedTime().Add(time.Second * MaxTimeOffsetSeconds)
	if timestamp.After(maxTimestamp) {
		logger.Errorf("block timestamp of %v is too far in the future", header.TimeStamp)
		return core.ErrTimeTooNew
	}

	return nil
}

// return number of transactions in a script
func getSigOpCount(script []byte) int {
	// TODO after adding script
	return 1
}

// return the number of signature operations for all transaction
// input and output scripts in the provided transaction.
func countSigOps(tx *types.Transaction) int {
	// Accumulate the number of signature operations in all transaction inputs.
	totalSigOps := 0
	for _, txIn := range tx.Vin {
		numSigOps := getSigOpCount(txIn.ScriptSig)
		totalSigOps += numSigOps
	}

	// Accumulate the number of signature operations in all transaction outputs.
	for _, txOut := range tx.Vout {
		numSigOps := getSigOpCount(txOut.ScriptPubKey)
		totalSigOps += numSigOps
	}

	return totalSigOps
}

// checkBlockWithoutContext performs context-free checks on a block.
func checkBlockWithoutContext(block *types.Block, timeSource util.MedianTimeSource) error {
	header := block.Header

	if err := checkBlockHeaderWithoutContext(header, timeSource); err != nil {
		return err
	}

	// Can't have no tx
	numTx := len(block.Txs)
	if numTx == 0 {
		logger.Errorf("block does not contain any transactions")
		return core.ErrNoTransactions
	}

	// TODO: check before deserialization
	// // A block must not exceed the maximum allowed block payload when serialized.
	// serializedSize := msgBlock.SerializeSizeStripped()
	// if serializedSize > MaxBlockSize {
	// 	logger.Errorf("serialized block is too big - got %d, "+
	// 		"max %d", serializedSize, MaxBlockSize)
	// 	return ErrBlockTooBig
	// }

	// First tx must be coinbase.
	transactions := block.Txs
	if !utils.IsCoinBase(transactions[0]) {
		logger.Errorf("first transaction in block is not a coinbase")
		return core.ErrFirstTxNotCoinbase
	}

	// There should be only one coinbase.
	for i, tx := range transactions[1:] {
		if utils.IsCoinBase(tx) {
			logger.Errorf("block contains second coinbase at index %d", i+1)
			return core.ErrMultipleCoinbases
		}
	}

	// Sanity checks each transaction.
	for _, tx := range transactions {
		if err := utils.SanityCheckTransaction(tx); err != nil {
			return err
		}
	}

	// Calculate merkle tree root and ensure it matches with the block header.
	// TODO: caching all of the transaction hashes in the block to speed up future hashing
	calculatedMerkleRoot := util.CalcTxsHash(transactions)
	if !header.TxsRoot.IsEqual(calculatedMerkleRoot) {
		logger.Errorf("block merkle root is invalid - block "+
			"header indicates %v, but calculated value is %v",
			header.TxsRoot, *calculatedMerkleRoot)
		return core.ErrBadMerkleRoot
	}

	// Detect duplicate transactions.
	existingTxHashes := make(map[*crypto.HashType]struct{})
	for _, tx := range transactions {
		txHash, _ := tx.TxHash()
		if _, exists := existingTxHashes[txHash]; exists {
			logger.Errorf("block contains duplicate transaction %v", txHash)
			return core.ErrDuplicateTx
		}
		existingTxHashes[txHash] = struct{}{}
	}

	// Enforce number of signature operations.
	totalSigOpCnt := 0
	for _, tx := range transactions {
		totalSigOpCnt += countSigOps(tx)
		if totalSigOpCnt > maxBlockSigOpCnt {
			logger.Errorf("block contains too many signature "+
				"operations - got %v, max %v", totalSigOpCnt, maxBlockSigOpCnt)
			return core.ErrTooManySigOps
		}
	}

	return nil
}

// TailBlock return chain tail block.
func (chain *BlockChain) TailBlock() *types.Block {
	return chain.tail
}

//LoadUnspentUtxo load related unspent utxo
// func (chain *BlockChain) LoadUnspentUtxo(tx *types.Transaction) (*utils.UtxoUnspentCache, error) {

// 	outPointMap := make(map[types.OutPoint]struct{})
// 	prevOut := types.OutPoint{Hash: *tx.Hash}
// 	for txOutIdx := range tx.Vout {
// 		prevOut.Index = uint32(txOutIdx)
// 		outPointMap[prevOut] = struct{}{}
// 	}
// 	if !utils.IsCoinBase(tx) {
// 		for _, txIn := range tx.Vin {
// 			outPointMap[txIn.PrevOutPoint] = struct{}{}
// 		}
// 	}

// 	// Request the utxos from the point of view of the end of the main
// 	// chain.
// 	// uup := NewUtxoUnspentCache()
// 	uup := UtxoUnspentCachePool.Get().(*UtxoUnspentCache)
// 	UtxoUnspentCachePool.Put(uup)
// 	// TODO: add mutex?
// 	err := uup.LoadUtxoFromDB(chain.db, outPointMap)

// 	return uup, err
// }

// LoadUtxoByPubKey loads utxos of a public key
func (chain *BlockChain) LoadUtxoByPubKey(pubkey []byte) (map[types.OutPoint]*utils.UtxoWrap, error) {
	res := make(map[types.OutPoint]*utils.UtxoWrap)
	// for out, entry := range chain.utxoSet.utxoMap {
	// 	if bytes.Equal(pubkey, entry.Output.ScriptPubKey) {
	// 		res[out] = entry
	// 	}
	// }
	return res, nil
}

//ListAllUtxos list all the available utxos for testing purpose
func (chain *BlockChain) ListAllUtxos() map[types.OutPoint]*utils.UtxoWrap {
	return nil
}

// ValidateTransactionScripts verify crypto signatures for each input
func (chain *BlockChain) ValidateTransactionScripts(tx *types.Transaction) error {
	txIns := tx.Vin
	txValItems := make([]*script.TxValidateItem, 0, len(txIns))
	for txInIdx, txIn := range txIns {
		// Skip coinbases.
		if txIn.PrevOutPoint.Index == math.MaxUint32 {
			continue
		}

		txVI := &script.TxValidateItem{
			TxInIndex: txInIdx,
			TxIn:      txIn,
			Tx:        tx,
		}
		txValItems = append(txValItems, txVI)
	}

	// Validate all of the inputs.
	// validator := NewTxValidator(unspentUtxo, flags, sigCache, hashCache)
	// return validator.Validate(txValItems)
	return nil
}

// SetTailBlock sets chain tail block.
func (chain *BlockChain) SetTailBlock(tail *types.Block, utxoSet *utils.UtxoSet) error {

	// save utxoset to database
	if err := utxoSet.WriteUtxoSetToDB(chain.DbTx); err != nil {
		return err
	}

	// save current tail to database
	if err := chain.StoreTailBlock(tail); err != nil {
		return err
	}
	chain.LongestChainHeight = tail.Height
	chain.tail = tail
	logger.Infof("Change New Tail. Hash: %s Height: %d", tail.BlockHash().String(), tail.Height)
	return nil
}
