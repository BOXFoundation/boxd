// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package chain

import (
	"errors"
	"math"
	"sort"
	"time"

	lru "github.com/hashicorp/golang-lru"

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

	// MaxBlockSigOps is the maximum number of signature operations
	// allowed for a block.
	MaxBlockSigOps = 80000

	// LockTimeThreshold is the number below which a lock time is
	// interpreted to be a block number. Since an average of one block
	// is generated per 10 minutes, this allows blocks for about 9,512 years.
	LockTimeThreshold = 5e8 // Tue Nov 5 00:53:20 1985 UTC

	// coinbase only spendable after this many blocks
	coinbaseMaturity = 0

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

// error
var (
	ErrBlockExists        = errors.New("Block already exists")
	ErrInvalidTime        = errors.New("Invalid time")
	ErrTimeTooNew         = errors.New("Block time too new")
	ErrNoTransactions     = errors.New("Block does not contain any transaction")
	ErrBlockTooBig        = errors.New("Block too big")
	ErrFirstTxNotCoinbase = errors.New("First transaction in block is not a coinbase")
	ErrMultipleCoinbases  = errors.New("Block contains multiple coinbase transactions")
	ErrBadMerkleRoot      = errors.New("Merkel root mismatch")
	ErrDuplicateTx        = errors.New("Duplicate transactions in a block")
	ErrTooManySigOps      = errors.New("Too many signature operations in a block")
	ErrImmatureSpend      = errors.New("Attempting to spend an immature coinbase")
	ErrSpendTooHigh       = errors.New("Transaction is attempting to spend more value than the sum of all of its inputs")
	ErrBadFees            = errors.New("total fees for block overflows accumulator")
	ErrBadCoinbaseValue   = errors.New("Coinbase pays more than expected value")
	ErrUnfinalizedTx      = errors.New("Transaction has not been finalized")
	ErrMissingTxOut       = errors.New("Referenced utxo does not exist")
)

var logger = log.NewLogger("chain") // logger

func init() {
}

// BlockChain define chain struct
type BlockChain struct {
	notifiee      p2p.Net
	newblockMsgCh chan p2p.Message
	dbBlock       storage.Table
	DbTx          storage.Table
	genesis       *types.Block
	tail          *types.Block
	proc          goprocess.Process

	// longest chain
	LongestChainHeight int32
	longestChainTip    *types.Block

	// Actually a tree-shaped structure where any block can have
	// multiple children.  However, there can only be one active branch (longest) which does
	// indeed form a chain from the tip all the way back to the genesis block.
	// It includes main chain and side chains, but not orphan blocks
	// hashToBlock map[crypto.HashType]*types.Block
	cache *lru.Cache

	// orphan block pool
	hashToOrphanBlock map[crypto.HashType]*types.Block
	// orphan block's children; one parent can have multiple orphan children
	orphanBlockHashToChildren map[crypto.HashType][]*types.Block

	// all utxos for main chain
	// UtxoSet *UtxoSet
}

// NewBlockChain return a blockchain.
func NewBlockChain(parent goprocess.Process, notifiee p2p.Net, db storage.Storage) (*BlockChain, error) {

	b := &BlockChain{
		notifiee:                  notifiee,
		newblockMsgCh:             make(chan p2p.Message, BlockMsgChBufferSize),
		proc:                      goprocess.WithParent(parent),
		hashToOrphanBlock:         make(map[crypto.HashType]*types.Block),
		orphanBlockHashToChildren: make(map[crypto.HashType][]*types.Block),
		// UtxoSet:                   NewUtxoSet(),
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

// blockExists determines whether a block with the given hash exists either in
// the main chain or any side chains.
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

// processOrphans determines if there are any orphans which depend on the accepted
// block hash (they are no longer orphans if true) and potentially accepts them.
// It repeats the process for the newly accepted blocks (to detect further
// orphans which may no longer be orphans) until there are no more.
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
			if _, err := chain.maybeAcceptBlock(orphan); err != nil {
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

// ProcessBlock is the main workhorse for handling insertion of new blocks into
// the block chain. It includes functionality such as rejecting duplicate
// blocks, ensuring blocks follow all rules, orphan handling, and insertion into
// the block chain along with best chain selection and reorganization.
//
// The first return value indicates if the block is on the main chain.
// The second indicates if the block is an orphan.
func (chain *BlockChain) ProcessBlock(block *types.Block, broadcast bool) (bool, bool, error) {
	blockHash := block.BlockHash()
	logger.Infof("Processing block hash: %v", *blockHash)

	// The block must not already exist in the main chain or side chains.
	if exists := chain.blockExists(*blockHash); exists {
		logger.Warnf("already have block %v", blockHash)
		return false, false, ErrBlockExists
	}

	// The block must not already exist as an orphan.
	if _, exists := chain.hashToOrphanBlock[*blockHash]; exists {
		logger.Warnf("already have block (orphan) %v", blockHash)
		return false, false, ErrBlockExists
	}

	// Perform preliminary sanity checks on the block and its transactions.
	if err := sanityCheckBlock(block, util.NewMedianTime()); err != nil {
		logger.Error(err)
		return false, false, err
	}

	// Handle orphan blocks.
	prevHash := block.Header.PrevBlockHash
	if prevHashExists := chain.blockExists(prevHash); !prevHashExists {
		logger.Infof("Adding orphan block %v with parent %v", *blockHash, prevHash)
		chain.addOrphanBlock(block, *blockHash, prevHash)

		return false, true, nil
	}

	// The block has passed all context independent checks and appears sane
	// enough to potentially accept it into the block chain.
	isMainChain, err := chain.maybeAcceptBlock(block)
	if err != nil {
		logger.Error(err)
		return false, false, err
	}

	// Accept any orphan blocks that depend on this block (they are no longer orphans)
	// and repeat for those accepted blocks until there are no more.
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

// checkBlockContext peforms several validation checks on the block which depend
// on its position within the block chain.
func (chain *BlockChain) checkBlockContext(block *types.Block) error {
	// using the current median time past of the past block's
	// timestamps for all lock-time based checks.
	blockTime := block.Header.TimeStamp

	// Ensure all transactions in the block are finalized.
	for _, tx := range block.Txs {
		if !IsFinalizedTransaction(tx, block.Height, blockTime) {
			txHash, _ := tx.TxHash()
			logger.Errorf("block contains unfinalized transaction %v", txHash)
			return ErrUnfinalizedTx
		}
	}

	return nil
}

// CheckTransactionInputs performs a series of checks on the inputs to a
// transaction to ensure they are valid.  An example of some of the checks
// include verifying all inputs exist, ensuring the coinbase seasoning
// requirements are met, detecting double spends, validating all values and fees
// are in the legal range and the total output amount doesn't exceed the input
// amount, and verifying the signatures to prove the spender was the owner of
// the bitcoins and therefore allowed to spend them.  As it checks the inputs,
// it also calculates the total fees for the transaction and returns that value.
func (chain *BlockChain) CheckTransactionInputs(utxoSet *UtxoSet, tx *types.Transaction, txHeight int32) (int64, error) {
	// Coinbase transactions have no inputs.
	if utils.IsCoinBase(tx) {
		return 0, nil
	}

	txHash, _ := tx.TxHash()
	var totalInputAmount int64
	for txInIndex, txIn := range tx.Vin {
		// Ensure the referenced input transaction is available.
		utxo := utxoSet.FindUtxo(txIn.PrevOutPoint)
		if utxo == nil || utxo.IsSpent {
			logger.Errorf("output %v referenced from transaction %s:%d does not exist or"+
				"has already been spent", txIn.PrevOutPoint, txHash, txInIndex)
			return 0, ErrMissingTxOut
		}

		// Ensure the transaction is not spending coins which have not
		// yet reached the required coinbase maturity.
		if utxo.IsCoinBase {
			originHeight := utxo.BlockHeight
			blocksSincePrev := txHeight - originHeight
			if blocksSincePrev < coinbaseMaturity {
				logger.Errorf("tried to spend coinbase transaction output %v from height %v "+
					"at height %v before required maturity of %v blocks", txIn.PrevOutPoint,
					originHeight, txHeight, coinbaseMaturity)
				return 0, ErrImmatureSpend
			}
		}

		// Ensure the transaction amounts are in range. Each of the
		// output values of the input transactions must not be negative
		// or more than the max allowed per transaction.
		utxoAmount := utxo.Value()
		if utxoAmount < 0 {
			logger.Errorf("transaction output has negative value of %v", utxoAmount)
			return 0, utils.ErrBadTxOutValue
		}
		if utxoAmount > utils.TotalSupply {
			logger.Errorf("transaction output value of %v is higher than max allowed value of %v", utxoAmount, utils.TotalSupply)
			return 0, utils.ErrBadTxOutValue
		}

		// The total of all outputs must not be more than the max allowed per transaction.
		// Also, we could potentially overflow the accumulator so check for overflow.
		lastAmount := totalInputAmount
		totalInputAmount += utxoAmount
		if totalInputAmount < lastAmount || totalInputAmount > utils.TotalSupply {
			logger.Errorf("total value of all transaction inputs is %v which is higher than max "+
				"allowed value of %v", totalInputAmount, utils.TotalSupply)
			return 0, utils.ErrBadTxOutValue
		}
	}

	// Calculate the total output amount for this transaction.  It is safe
	// to ignore overflow and out of range errors here because those error
	// conditions would have already been caught by SanityCheckTransaction.
	var totalOutputAmount int64
	for _, txOut := range tx.Vout {
		totalOutputAmount += txOut.Value
	}

	// Ensure the transaction does not spend more than its inputs.
	if totalInputAmount < totalOutputAmount {
		logger.Errorf("total value of all transaction inputs for "+
			"transaction %v is %v which is less than the amount "+
			"spent of %v", txHash, totalInputAmount, totalOutputAmount)
		return 0, ErrSpendTooHigh
	}

	txFee := totalInputAmount - totalOutputAmount
	return txFee, nil
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

// calcPastMedianTime calculates the median time of the previous few blocks
// prior to, and including, the block.
func (chain *BlockChain) calcPastMedianTime(block *types.Block) time.Time {
	// Create a slice of the previous few block timestamps used to calculate
	// the median per the number defined by the constant medianTimeBlocks.
	timestamps := make([]int64, medianTimeBlocks)
	i := 0
	for iterBlock := block; i < medianTimeBlocks && iterBlock != nil; i++ {
		timestamps[i] = iterBlock.Header.TimeStamp
		iterBlock = chain.getParentBlock(iterBlock)
	}

	// Prune the slice to the actual number of available timestamps which
	// will be fewer than desired near the beginning of the block chain and sort them.
	timestamps = timestamps[:i]
	sort.Sort(timeSorter(timestamps))

	// NOTE: The consensus rules incorrectly calculate the median for even
	// numbers of blocks. A true median averages the middle two elements
	// for a set with an even number of elements in it.
	medianTimestamp := timestamps[i/2]
	return time.Unix(medianTimestamp, 0)
}

// ancestor returns the ancestor block at the provided height by following
// the chain backwards from this block.  The returned block will be nil when a
// height is requested that is after the height of the passed block or is less than zero.
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

// SequenceLock represents the converted relative lock-time in seconds, and
// absolute block-height for a transaction input's relative lock-times.
// According to SequenceLock, after the referenced input has been confirmed within a block,
// a transaction spending that input can be included into a block either after 'seconds'
// (according to past median time), or once the 'BlockHeight' has been reached.
type SequenceLock struct {
	Seconds     int64
	BlockHeight int32
}

// calcSequenceLock computes the relative lock-times for the passed transaction.
func (chain *BlockChain) calcSequenceLock(utxoSet *UtxoSet, block *types.Block, tx *types.Transaction) (*SequenceLock, error) {
	// A value of -1 for each relative lock type represents a relative time lock value
	// that will allow a transaction to be included in a block at any given height or time.
	sequenceLock := &SequenceLock{Seconds: -1, BlockHeight: -1}

	// Sequence lock does not apply to coinbase tx.
	if utils.IsCoinBase(tx) {
		return sequenceLock, nil
	}

	// Grab the next height from the PoV of the passed block to use for inputs present in the mempool.
	nextHeight := block.Height + 1

	for txInIndex, txIn := range tx.Vin {
		txHash, _ := tx.TxHash()
		utxo := utxoSet.FindUtxo(txIn.PrevOutPoint)
		if utxo == nil {
			logger.Errorf("output %v referenced from transaction %v:%d either does not exist or "+
				"has already been spent", txIn.PrevOutPoint, txHash, txInIndex)
			return sequenceLock, ErrMissingTxOut
		}

		// Referenced utxo's block height
		inputHeight := utxo.BlockHeight
		// If the input height is set to the mempool height, then we assume the transaction makes it
		// into the next block when evaluating its sequence blocks.
		if inputHeight == unminedHeight {
			inputHeight = nextHeight
		}

		// Given a sequence number, we apply the relative time lock mask in order to obtain the time lock delta
		// required before this input can be spent.
		sequenceNum := txIn.Sequence
		relativeLock := int64(sequenceNum & sequenceLockTimeMask)

		if sequenceNum&sequenceLockTimeIsSeconds == sequenceLockTimeIsSeconds {
			// This input requires a relative time lock expressed in seconds before it can be spent.
			// Therefore, we need to query for the block prior to the one in which this input was included within so
			// we can compute the past median time for the block prior to the one which included this referenced output.
			prevInputHeight := inputHeight - 1
			if prevInputHeight < 0 {
				prevInputHeight = 0
			}
			ancestor := chain.ancestor(block, prevInputHeight)
			medianTime := chain.calcPastMedianTime(ancestor)

			// Time based relative time-locks have a time granularity of 512 seconds,
			// so we shift left by this amount to convert to the proper relative time-lock.
			// We also subtract one from the relative lock to maintain the original lockTime semantics.
			timeLockSeconds := (relativeLock << sequenceLockTimeGranularity) - 1
			timeLock := medianTime.Unix() + timeLockSeconds
			if timeLock > sequenceLock.Seconds {
				sequenceLock.Seconds = timeLock
			}
		} else {
			// The relative lock-time for this input is expressed in blocks so we calculate
			// the relative offset from the input's height as its converted absolute lock-time.
			// We subtract one from the relative lock to maintain the original lockTime semantics.
			blockHeight := inputHeight + int32(relativeLock-1)
			if blockHeight > sequenceLock.BlockHeight {
				sequenceLock.BlockHeight = blockHeight
			}
		}
	}

	return sequenceLock, nil
}

// SequenceLockActive determines if a transaction's sequence locks have been
// met, meaning that all the inputs of a given transaction have reached a
// height or time sufficient for their relative lock-time maturity.
func sequenceLockActive(sequenceLock *SequenceLock, blockHeight int32, medianTimePast time.Time) bool {
	return sequenceLock.Seconds < medianTimePast.Unix() && sequenceLock.BlockHeight < blockHeight
}

// Validates the scripts for all of the passed transaction inputs
func validateTxs(txValidateItems []*script.TxValidateItem) error {
	// TODO: execute and verify script
	return nil
}

// checkBlockScripts executes and validates the scripts for all transactions in
// the passed block using multiple goroutines.
func checkBlockScripts(block *types.Block) error {
	// Collect all of the transaction inputs and required information for
	// validation for all transactions in the block into a single slice.
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

// maybeConnectBlock performs several checks to confirm connecting the passed
// block to the chain does not violate any rules.
// In addition, the utxo set is updated to spend all of the referenced
// outputs and add all of the new utxos created by block.
//
// An example of some of the checks performed are ensuring connecting the block
// would not cause any duplicate transaction hashes for old transactions that
// aren't already fully spent, double spends, exceeding the maximum allowed
// signature operations per block, invalid values in relation to the expected
// block subsidy, or fail transaction script validation.
func (chain *BlockChain) maybeConnectBlock(block *types.Block) error {
	// // TODO: needed?
	// // The coinbase for the Genesis block is not spendable, so just return
	// // an error now.
	// if block.BlockHash.IsEqual(genesisHash) {
	// 	str := "the coinbase for the genesis block is not spendable"
	// 	return ErrMissingTxOut
	// }

	transactions := block.Txs

	utxoSet, err := LoadBlockUtxos(block, chain.DbTx)
	if err != nil {
		return err
	}

	// Perform several checks on the inputs for each transaction.  Also
	// accumulate the total fees.
	var totalFees int64
	for _, tx := range transactions {
		txFee, err := chain.CheckTransactionInputs(utxoSet, tx, block.Height)
		if err != nil {
			return err
		}

		// Sum the total fees and ensure we don't overflow the
		// accumulator.
		lastTotalFees := totalFees
		totalFees += txFee
		if totalFees < lastTotalFees {
			return ErrBadFees
		}

		// Update utxos by applying this tx
		if err := utxoSet.ApplyTx(tx, block.Height); err != nil {
			return err
		}
	}

	// The total output values of the coinbase transaction must not exceed
	// the expected subsidy value plus total transaction fees gained from
	// mining the block. It is safe to ignore overflow and out of range
	// errors here because those error conditions would have already been
	// caught by SanityCheckTransaction.
	var totalCoinbaseOutput int64
	for _, txOut := range transactions[0].Vout {
		totalCoinbaseOutput += txOut.Value
	}
	expectedCoinbaseOutput := utils.CalcBlockSubsidy(block.Height) + totalFees
	if totalCoinbaseOutput > expectedCoinbaseOutput {
		logger.Errorf("coinbase transaction for block pays %v which is more than expected value of %v",
			totalCoinbaseOutput, expectedCoinbaseOutput)
		return ErrBadCoinbaseValue
	}

	// We obtain the MTP of the *previous* block in order to
	// determine if transactions in the current block are final.
	medianTime := chain.calcPastMedianTime(chain.getParentBlock(block))

	// Enforce the relative sequence number based lock-times within
	// the inputs of all transactions in this candidate block.
	for _, tx := range transactions {
		// A transaction can only be included within a block
		// once the sequence locks of *all* its inputs are active.
		sequenceLock, err := chain.calcSequenceLock(utxoSet, block, tx)
		if err != nil {
			return err
		}
		if !sequenceLockActive(sequenceLock, block.Height, medianTime) {
			logger.Errorf("block contains transaction whose input sequence locks are not met")
			return ErrUnfinalizedTx
		}
	}

	// Now that the inexpensive checks are done and have passed, verify the
	// transactions are actually allowed to spend the coins by running the
	// expensive ECDSA signature check scripts. Doing this last helps
	// prevent CPU exhaustion attacks.
	if err := checkBlockScripts(block); err != nil {
		return err
	}

	// This block is now the end of the best chain.
	chain.SetTailBlock(block)

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
	attachBlocks := []*types.Block{block}

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

// connectBlockToChain handles connecting the passed block to the chain while
// respecting proper chain selection according to the longest chain.
// In the typical case, the new block simply extends the main
// chain. However, it may also be extending (or creating) a side chain (fork)
// which may or may not end up becoming the main chain depending on which fork
// cumulatively has the most proof of work.  It returns whether or not the block
// ended up on the main chain (either due to extending the main chain or causing
// a reorganization to become the main chain).
func (chain *BlockChain) connectBlockToChain(block *types.Block) (bool, error) {
	blockHash := block.BlockHash()
	parentHash := &block.Header.PrevBlockHash
	tailHash := chain.TailBlock().BlockHash()
	if parentHash.IsEqual(tailHash) {
		// We are extending the main (best) chain with a new block. This is the most common case.
		// Perform several checks to verify the block can be connected to the
		// main chain without violating any rules before actually connecting the block.
		err := chain.maybeConnectBlock(block)
		if err != nil {
			return false, err
		}
		return true, nil
	}

	// We're extending (or creating) a side chain, but the new side chain is not long enough to make it the main chain.
	if block.Height <= chain.LongestChainHeight {
		logger.Infof("Block %v extends a side chain to height %d, shorter than main chain of height %d",
			blockHash, block.Height, chain.LongestChainHeight)
		return false, nil
	}

	// We're extending a side chain longer than the old best chain, so this side
	// chain needs to become the main chain.
	logger.Infof("REORGANIZE: Block %v is causing a reorganization.", blockHash)
	err := chain.reorganizeChain(block)
	if err != nil {
		return false, err
	}
	// This block is now the end of the best chain.
	chain.SetTailBlock(block)
	return true, err
}

func (chain *BlockChain) revertBlock(block *types.Block) error {

	utxoSet, err := LoadBlockUtxos(block, chain.DbTx)
	if err != nil {
		return err
	}

	// Revert UTXOs
	if err := utxoSet.RevertBlock(block); err != nil {
		return err
	}

	return chain.notifyBlockConnectionUpdate(block, false)
}

func (chain *BlockChain) applyBlock(block *types.Block) error {

	utxoSet, err := LoadBlockUtxos(block, chain.DbTx)
	if err != nil {
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

func (chain *BlockChain) reorganizeChain(block *types.Block) error {
	// Find the common ancestor of the main chain and side chain
	_, detachBlocks, attachBlocks := chain.findFork(block)

	// Detach the blocks that form the (now) old fork from the main chain.
	// From tip to fork, not including fork
	for _, detachBlock := range detachBlocks {
		if err := chain.revertBlock(detachBlock); err != nil {
			return err
		}
	}

	// Attach the blocks that form the new chain to the main chain starting at the
	// common ancenstor (the point where the chain forked).
	// From fork to tip, not including fork
	for blockIdx := len(attachBlocks) - 1; blockIdx >= 0; blockIdx-- {
		attachBlock := attachBlocks[blockIdx]
		if err := chain.applyBlock(attachBlock); err != nil {
			return err
		}
	}

	return nil
}

// IsFinalizedTransaction determines whether or not a transaction is finalized.
func IsFinalizedTransaction(tx *types.Transaction, blockHeight int32, blockTime int64) bool {
	// Lock time of zero means the transaction is finalized.
	lockTime := tx.LockTime
	if lockTime == 0 {
		return true
	}

	// The lock time field of a transaction is either a block height at
	// which the transaction is finalized or a timestamp depending on if the
	// value is before the LockTimeThreshold.  When it is under the
	// threshold it is a block height.
	blockTimeOrHeight := int64(0)
	if lockTime < LockTimeThreshold {
		blockTimeOrHeight = int64(blockHeight)
	} else {
		blockTimeOrHeight = blockTime
	}
	if lockTime < blockTimeOrHeight {
		return true
	}

	// At this point, the transaction's lock time hasn't occurred yet, but
	// the transaction might still be finalized if the sequence number
	// for all transaction inputs is maxed out.
	for _, txIn := range tx.Vin {
		if txIn.Sequence != math.MaxUint32 {
			return false
		}
	}
	return true
}

// maybeAcceptBlock potentially accepts a block into the block chain and, if
// accepted, returns whether or not it is on the main chain.  It performs
// several validation checks which depend on its position within the block chain
// before adding it. The block is expected to have already gone through
// ProcessBlock before calling this function with it.
func (chain *BlockChain) maybeAcceptBlock(block *types.Block) (bool, error) {
	// must not be orphan block if reaching here
	parentBlock := chain.getParentBlock(block)

	// The height of this block is one more than the referenced previous block.
	block.Height = parentBlock.Height + 1

	// The block must pass all of the validation rules which depend on the
	// position of the block within the block chain.
	if err := chain.checkBlockContext(block); err != nil {
		return false, err
	}

	blockHash := block.BlockHash()
	if err := chain.StoreBlockToDb(block); err != nil {
		return false, err
	}
	chain.cache.Add(*blockHash, block)

	// Connect the passed block to the chain while respecting proper chain
	// selection according to the longest chain.
	// This also handles validation of the transaction scripts.
	isMainChain, err := chain.connectBlockToChain(block)
	if err != nil {
		return false, err
	}

	// Notify the caller that the new block was accepted into the block chain.
	// The caller would typically want to react by relaying the inventory to other peers.
	// TODO
	// chain.sendNotification(NTBlockAccepted, block)

	return isMainChain, nil
}

// sanityCheckBlockHeader performs some preliminary checks on a block header to
// ensure it is sane before continuing with processing.  These checks are
// context free.
func sanityCheckBlockHeader(header *types.BlockHeader, timeSource util.MedianTimeSource) error {
	// TODO: PoW check here
	// err := checkProofOfWork(header, powLimit, flags)

	// A block timestamp must not have a greater precision than one second.
	// Go time.Time values support
	// nanosecond precision whereas the consensus rules only apply to
	// seconds and it's much nicer to deal with standard Go time values
	// instead of converting to seconds everywhere.
	timestamp := time.Unix(header.TimeStamp, 0)

	// Ensure the block time is not too far in the future.
	maxTimestamp := timeSource.AdjustedTime().Add(time.Second * MaxTimeOffsetSeconds)
	if timestamp.After(maxTimestamp) {
		logger.Errorf("block timestamp of %v is too far in the future", header.TimeStamp)
		return ErrTimeTooNew
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

// sanityCheckBlock performs some preliminary checks on a block to ensure it is
// sane before continuing with block processing.  These checks are context free.
func sanityCheckBlock(block *types.Block, timeSource util.MedianTimeSource) error {
	header := block.Header

	if err := sanityCheckBlockHeader(header, timeSource); err != nil {
		return err
	}

	// A block must have at least one transaction.
	numTx := len(block.Txs)
	if numTx == 0 {
		logger.Errorf("block does not contain any transactions")
		return ErrNoTransactions
	}

	// TODO: check before deserialization
	// // A block must not exceed the maximum allowed block payload when serialized.
	// serializedSize := msgBlock.SerializeSizeStripped()
	// if serializedSize > MaxBlockSize {
	// 	logger.Errorf("serialized block is too big - got %d, "+
	// 		"max %d", serializedSize, MaxBlockSize)
	// 	return ErrBlockTooBig
	// }

	// The first transaction in a block must be a coinbase.
	transactions := block.Txs
	if !utils.IsCoinBase(transactions[0]) {
		logger.Errorf("first transaction in block is not a coinbase")
		return ErrFirstTxNotCoinbase
	}

	// A block must not have more than one coinbase.
	for i, tx := range transactions[1:] {
		if utils.IsCoinBase(tx) {
			logger.Errorf("block contains second coinbase at index %d", i+1)
			return ErrMultipleCoinbases
		}
	}

	// Do some preliminary checks on each transaction to ensure they are
	// sane before continuing.
	for _, tx := range transactions {
		err := utils.SanityCheckTransaction(tx)
		if err != nil {
			return err
		}
	}

	// Build merkle tree and ensure the calculated merkle root matches the entry in the block header.
	// TODO: caching all of the transaction hashes in the block to speed up future hashing
	calculatedMerkleRoot := util.CalcTxsHash(transactions)
	if !header.TxsRoot.IsEqual(calculatedMerkleRoot) {
		logger.Errorf("block merkle root is invalid - block "+
			"header indicates %v, but calculated value is %v",
			header.TxsRoot, *calculatedMerkleRoot)
		return ErrBadMerkleRoot
	}

	// Check for duplicate transactions.
	existingTxHashes := make(map[*crypto.HashType]struct{})
	for _, tx := range transactions {
		txHash, _ := tx.TxHash()
		if _, exists := existingTxHashes[txHash]; exists {
			logger.Errorf("block contains duplicate transaction %v", txHash)
			return ErrDuplicateTx
		}
		existingTxHashes[txHash] = struct{}{}
	}

	// The number of signature operations must be less than the maximum
	// allowed per block.
	totalSigOps := 0
	for _, tx := range transactions {
		totalSigOps += countSigOps(tx)
		if totalSigOps > MaxBlockSigOps {
			logger.Errorf("block contains too many signature "+
				"operations - got %v, max %v", totalSigOps, MaxBlockSigOps)
			return ErrTooManySigOps
		}
	}

	return nil
}

// TailBlock return chain tail block.
func (chain *BlockChain) TailBlock() *types.Block {
	return chain.tail
}

//LoadUnspentUtxo load related unspent utxo
// func (chain *BlockChain) LoadUnspentUtxo(tx *types.Transaction) (*UtxoUnspentCache, error) {

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
func (chain *BlockChain) LoadUtxoByPubKey(pubkey []byte) (map[types.OutPoint]*UtxoEntry, error) {
	res := make(map[types.OutPoint]*UtxoEntry)
	// for out, entry := range chain.utxoSet.utxoMap {
	// 	if bytes.Equal(pubkey, entry.Output.ScriptPubKey) {
	// 		res[out] = entry
	// 	}
	// }
	return res, nil
}

//ListAllUtxos list all the available utxos for testing purpose
func (chain *BlockChain) ListAllUtxos() map[types.OutPoint]*UtxoEntry {
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

func mergeUtxoCache(cacheA *UtxoUnspentCache, cacheB *UtxoUnspentCache) {
	viewAEntries := cacheA.outPointMap
	for outpoint, entryB := range cacheB.outPointMap {
		if entryA, exists := viewAEntries[outpoint]; !exists ||
			entryA == nil || entryA.IsPacked {
			viewAEntries[outpoint] = entryB
		}
	}
}

func (chain *BlockChain) spendTransaction(blockUtxos *UtxoUnspentCache, tx *types.Transaction, height int32) error {
	for _, txIn := range tx.Vin {
		utxowrap := blockUtxos.FindByOutPoint(txIn.PrevOutPoint)
		if utxowrap != nil {
			utxowrap.IsPacked = true
		}
	}

	blockUtxos.AddTxOuts(tx, height)
	return nil
}

// SetTailBlock sets chain tail block.
func (chain *BlockChain) SetTailBlock(tail *types.Block) error {

	if err := chain.StoreTailBlock(tail); err != nil {
		return err
	}
	chain.LongestChainHeight = tail.Height
	chain.tail = tail
	logger.Infof("Change New Tail. Hash: %s Height: %d", tail.BlockHash().String(), tail.Height)
	return nil
}
