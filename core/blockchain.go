// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package core

import (
	"container/heap"
	"errors"
	"math"
	"time"

	corepb "github.com/BOXFoundation/Quicksilver/core/pb"

	"github.com/BOXFoundation/Quicksilver/core/types"
	"github.com/BOXFoundation/Quicksilver/crypto"
	"github.com/BOXFoundation/Quicksilver/log"
	"github.com/BOXFoundation/Quicksilver/p2p"
	"github.com/BOXFoundation/Quicksilver/storage"
	"github.com/BOXFoundation/Quicksilver/util"
	proto "github.com/gogo/protobuf/proto"
	"github.com/jbenet/goprocess"
)

// const defines constants
const (
	BlockMsgChBufferSize = 1024

	Tail = "tail_block"

	// MaxTimeOffsetSeconds is the maximum number of seconds a block time
	// is allowed to be ahead of the current time.  This is currently 2 hours.
	MaxTimeOffsetSeconds = 2 * 60 * 60

	// MaxBlockSize is the maximum number of bytes within a block
	MaxBlockSize = 32000000

	// CoinbaseLib is the number of blocks required before newly mined coins (coinbase transactions) can be spent.
	CoinbaseLib = 100

	// decimals is the number of digits after decimal point of value/amount
	decimals = 8

	// MinCoinbaseScriptLen is the minimum length a coinbase script can be.
	MinCoinbaseScriptLen = 2

	// MaxCoinbaseScriptLen is the maximum length a coinbase script can be.
	MaxCoinbaseScriptLen = 1000

	// MaxBlockSigOps is the maximum number of signature operations
	// allowed for a block.
	MaxBlockSigOps = 80000
)

var (
	// zeroHash is the zero value for a hash
	zeroHash crypto.HashType

	// totalSupply is the total supply of box: 3 billion
	totalSupply = (int64)(3e9 * math.Pow10(decimals))
)

// error
var (
	ErrBlockExists          = errors.New("Block already exists")
	ErrInvalidTime          = errors.New("Invalid time")
	ErrTimeTooNew           = errors.New("Block time too new")
	ErrNoTransactions       = errors.New("Block does not contain any transaction")
	ErrBlockTooBig          = errors.New("Block too big")
	ErrFirstTxNotCoinbase   = errors.New("First transaction in block is not a coinbase")
	ErrMultipleCoinbases    = errors.New("Block contains multiple coinbase transactions")
	ErrNoTxInputs           = errors.New("Transaction has no inputs")
	ErrNoTxOutputs          = errors.New("Transaction has no outputs")
	ErrBadTxOutValue        = errors.New("Invalid output value")
	ErrDuplicateTxInputs    = errors.New("Transaction contains duplicate inputs")
	ErrBadCoinbaseScriptLen = errors.New("Coinbase scriptSig out of range")
	ErrBadTxInput           = errors.New("Transaction input refers to null out point")
	ErrBadMerkleRoot        = errors.New("Merkel root mismatch")
	ErrDuplicateTx          = errors.New("Duplicate transactions in a block")
	ErrTooManySigOps        = errors.New("Too many signature operations in a block")
)

// isNullOutPoint determines whether or not a previous transaction output point is set.
func isNullOutPoint(outPoint *types.OutPoint) bool {
	return outPoint.Index == math.MaxUint32 && outPoint.Hash == zeroHash
}

var logger = log.NewLogger("core") // logger

func init() {
}

// BlockChain define chain struct
type BlockChain struct {
	notifiee      p2p.Net
	newblockMsgCh chan p2p.Message
	txpool        *TransactionPool
	db            storage.Storage
	genesis       *types.Block
	tail          *types.Block
	proc          goprocess.Process

	// longest chain
	longestChainHeight int
	longestChainTip    *types.MsgBlock

	// Actually a tree-shaped structure where any node can have
	// multiple children.  However, there can only be one active branch (longest) which does
	// indeed form a chain from the tip all the way back to the genesis block.
	// It includes main chain and side chains, but not orphan blocks
	hashToBlock map[crypto.HashType]*types.MsgBlock

	// orphan block pool
	hashToOrphanBlock map[crypto.HashType]*types.MsgBlock
	// orphan block's children; one parent can have multiple orphan children
	orphanBlockHashToChildren map[crypto.HashType][]*types.MsgBlock
}

// NewBlockChain return a blockchain.
func NewBlockChain(parent goprocess.Process, notifiee p2p.Net, db storage.Storage) (*BlockChain, error) {

	b := &BlockChain{
		notifiee:      notifiee,
		newblockMsgCh: make(chan p2p.Message, BlockMsgChBufferSize),
		proc:          goprocess.WithParent(parent),
		db:            db,
	}

	b.txpool = NewTransactionPool(parent, notifiee, b)
	genesis, err := b.loadGenesis()
	if err != nil {
		return nil, err
	}
	b.genesis = genesis

	tail, err := b.loadTailBlock()
	if err != nil {
		return nil, err
	}
	b.tail = tail

	return b, nil
}

func (chain *BlockChain) loadGenesis() (*types.Block, error) {

	if ok, _ := chain.db.Has(genesisHash[:]); ok {
		genesisMsgBlock, err := chain.LoadBlockByHashFromDb(genesisHash)
		if err != nil {
			return nil, err
		}
		genesis := &types.Block{
			Hash:     &genesisHash,
			MsgBlock: genesisMsgBlock,
			Height:   0,
		}
		return genesis, nil
	}

	genesispb, err := genesisBlock.Serialize()
	if err != nil {
		return nil, err
	}
	genesisBin, err := proto.Marshal(genesispb)
	chain.db.Put(genesisHash[:], genesisBin)

	genesis := &types.Block{
		Hash:     &genesisHash,
		MsgBlock: &genesisBlock,
		Height:   0,
	}
	return genesis, nil

}

func (chain *BlockChain) loadTailBlock() (*types.Block, error) {

	if ok, _ := chain.db.Has([]byte(Tail)); ok {
		tailBin, err := chain.db.Get([]byte(Tail))
		if err != nil {
			return nil, err
		}

		pbblock := new(corepb.MsgBlock)
		if err := proto.Unmarshal(tailBin, pbblock); err != nil {
			return nil, err
		}

		tailMsgBlock := new(types.MsgBlock)
		if err := tailMsgBlock.Deserialize(pbblock); err != nil {
			return nil, err
		}

		tail := &types.Block{
			Hash:     &genesisHash,
			MsgBlock: tailMsgBlock,
			Height:   0,
		}
		return tail, nil

	}

	tailpb, err := genesisBlock.Serialize()
	if err != nil {
		return nil, err
	}
	tailBin, err := proto.Marshal(tailpb)
	if err != nil {
		return nil, err
	}
	chain.db.Put([]byte(Tail), tailBin)

	tail := &types.Block{
		Hash:     &genesisHash,
		MsgBlock: &genesisBlock,
		Height:   0,
	}

	return tail, nil

}

// LoadBlockByHashFromDb load block by hash from db.
func (chain *BlockChain) LoadBlockByHashFromDb(hash crypto.HashType) (*types.MsgBlock, error) {

	blockBin, err := chain.db.Get(hash[:])
	if err != nil {
		return nil, err
	}

	pbblock := new(corepb.MsgBlock)
	if err := proto.Unmarshal(blockBin, pbblock); err != nil {
		return nil, err
	}

	block := new(types.MsgBlock)
	if err := block.Deserialize(pbblock); err != nil {
		return nil, err
	}

	return block, nil
}

// Run launch blockchain.
func (chain *BlockChain) Run() {

	chain.subscribeMessageNotifiee(chain.notifiee)
	go chain.loop()
	chain.txpool.Run()
}

func (chain *BlockChain) subscribeMessageNotifiee(notifiee p2p.Net) {
	notifiee.Subscribe(p2p.NewNotifiee(p2p.NewBlockMsg, chain.newblockMsgCh))
}

func (chain *BlockChain) loop() {
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

	body := msg.Body()
	pbblock := new(corepb.MsgBlock)
	if err := proto.Unmarshal(body, pbblock); err != nil {
		return err
	}
	block := new(types.MsgBlock)
	if err := block.Deserialize(pbblock); err != nil {
		return err
	}

	// process block
	chain.ProcessBlock(block)

	return nil
}

// blockExists determines whether a block with the given hash exists either in
// the main chain or any side chains.
func (chain *BlockChain) blockExists(blockHash crypto.HashType) bool {
	_, exists := chain.hashToBlock[blockHash]
	return exists
}

func (chain *BlockChain) addOrphanBlock(block *types.MsgBlock, blockHash crypto.HashType, prevHash crypto.HashType) {
	chain.hashToOrphanBlock[blockHash] = block
	// Add to previous hash lookup index for faster dependency lookups.
	chain.orphanBlockHashToChildren[prevHash] = append(chain.orphanBlockHashToChildren[prevHash], block)
}

// ProcessBlock is the main workhorse for handling insertion of new blocks into
// the block chain.  It includes functionality such as rejecting duplicate
// blocks, ensuring blocks follow all rules, orphan handling, and insertion into
// the block chain along with best chain selection and reorganization.
//
// The first return value indicates if the block is on the main chain.
// The second indicates if the block is an orphan.
func (chain *BlockChain) ProcessBlock(block *types.MsgBlock) (bool, bool, error) {
	blockHash, err := block.BlockHash()
	if err != nil {
		logger.Errorf("block hash calculation error")
		return false, false, err
	}
	logger.Infof("Processing block %v", blockHash)

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
		return false, false, err
	}

	// Handle orphan blocks.
	prevHash := block.Header.PrevBlockHash
	if prevHashExists := chain.blockExists(prevHash); !prevHashExists {
		logger.Infof("Adding orphan block %v with parent %v", blockHash, prevHash)
		chain.addOrphanBlock(block, *blockHash, prevHash)

		return false, true, nil
	}

	// // The block has passed all context independent checks and appears sane
	// // enough to potentially accept it into the block chain.
	// isMainChain, err := b.maybeAcceptBlock(block, flags)
	// if err != nil {
	// 	return false, false, err
	// }

	// // Accept any orphan blocks that depend on this block (they are
	// // no longer orphans) and repeat for those accepted blocks until
	// // there are no more.
	// err = b.processOrphans(blockHash, flags)
	// if err != nil {
	// 	return false, false, err
	// }

	// log.Debugf("Accepted block %v", blockHash)

	// return isMainChain, false, nil
	// TODO
	return true, false, nil
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

// IsCoinBase determines whether or not a transaction is a coinbase.  A coinbase
// is a special transaction created by miners that has no inputs.  This is
// represented in the block chain by a transaction with a single input that has
// a previous output transaction index set to the maximum value along with a zero hash.
//
// This function only differs from IsCoinBase in that it works with a raw wire
// transaction as opposed to a higher level util transaction.
func IsCoinBase(tx *types.MsgTx) bool {
	// A coin base must only have one transaction input.
	if len(tx.Vin) != 1 {
		return false
	}

	// The previous output of a coin base must have a max value index and a zero hash.
	return isNullOutPoint(&tx.Vin[0].PrevOutPoint)
}

// SanityCheckTransaction performs some preliminary checks on a transaction to
// ensure it is sane. These checks are context free.
func SanityCheckTransaction(tx *types.MsgTx) error {
	// A transaction must have at least one input.
	if len(tx.Vin) == 0 {
		return ErrNoTxInputs
	}

	// A transaction must have at least one output.
	if len(tx.Vout) == 0 {
		return ErrNoTxOutputs
	}

	// TOOD: check before deserialization
	// // A transaction must not exceed the maximum allowed block payload when
	// // serialized.
	// serializedTxSize := tx.MsgTx().SerializeSizeStripped()
	// if serializedTxSize > MaxBlockBaseSize {
	// 	str := fmt.Sprintf("serialized transaction is too big - got "+
	// 		"%d, max %d", serializedTxSize, MaxBlockBaseSize)
	// 	return ruleError(ErrTxTooBig, str)
	// }

	// Ensure the transaction amounts are in range. Each transaction
	// output must not be negative or more than the max allowed per
	// transaction. Also, the total of all outputs must abide by the same
	// restrictions.
	var totalValue int64
	for _, txOut := range tx.Vout {
		value := txOut.Value
		if value < 0 {
			logger.Errorf("transaction output has negative value of %v", value)
			return ErrBadTxOutValue
		}
		if value > totalSupply {
			logger.Errorf("transaction output value of %v is "+
				"higher than max allowed value of %v", totalSupply)
			return ErrBadTxOutValue
		}

		// Two's complement int64 overflow guarantees that any overflow
		// is detected and reported.
		totalValue += value
		if totalValue < 0 {
			logger.Errorf("total value of all transaction outputs overflows %v", totalValue)
			return ErrBadTxOutValue
		}
		if totalValue > totalSupply {
			logger.Errorf("total value of all transaction "+
				"outputs is %v which is higher than max "+
				"allowed value of %v", totalValue, totalSupply)
			return ErrBadTxOutValue
		}
	}

	// Check for duplicate transaction inputs.
	existingOutPoints := make(map[types.OutPoint]struct{})
	for _, txIn := range tx.Vin {
		if _, exists := existingOutPoints[txIn.PrevOutPoint]; exists {
			return ErrDuplicateTxInputs
		}
		existingOutPoints[txIn.PrevOutPoint] = struct{}{}
	}

	if IsCoinBase(tx) {
		// Coinbase script length must be between min and max length.
		slen := len(tx.Vin[0].ScriptSig)
		if slen < MinCoinbaseScriptLen || slen > MaxCoinbaseScriptLen {
			logger.Errorf("coinbase transaction script length "+
				"of %d is out of range (min: %d, max: %d)",
				slen, MinCoinbaseScriptLen, MaxCoinbaseScriptLen)
			return ErrBadCoinbaseScriptLen
		}
	} else {
		// Previous transaction outputs referenced by the inputs to this
		// transaction must not be null.
		for _, txIn := range tx.Vin {
			if isNullOutPoint(&txIn.PrevOutPoint) {
				return ErrBadTxInput
			}
		}
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
func countSigOps(tx *types.MsgTx) int {
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
func sanityCheckBlock(block *types.MsgBlock, timeSource util.MedianTimeSource) error {
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
	if !IsCoinBase(transactions[0]) {
		logger.Errorf("first transaction in block is not a coinbase")
		return ErrFirstTxNotCoinbase
	}

	// A block must not have more than one coinbase.
	for i, tx := range transactions[1:] {
		if IsCoinBase(tx) {
			logger.Errorf("block contains second coinbase at index %d", i+1)
			return ErrMultipleCoinbases
		}
	}

	// Do some preliminary checks on each transaction to ensure they are
	// sane before continuing.
	for _, tx := range transactions {
		err := SanityCheckTransaction(tx)
		if err != nil {
			return err
		}
	}

	// Build merkle tree and ensure the calculated merkle root matches the entry in the block header.
	// TODO: caching all of the transaction hashes in the block to speed up future hashing
	calculatedMerkleRoot := util.CalcTxsHash(block.Txs)
	if !header.TxsRoot.IsEqual(calculatedMerkleRoot) {
		logger.Errorf("block merkle root is invalid - block "+
			"header indicates %v, but calculated value is %v",
			header.TxsRoot, *calculatedMerkleRoot)
		return ErrBadMerkleRoot
	}

	// Check for duplicate transactions.
	existingTxHashes := make(map[*crypto.HashType]struct{})
	for _, tx := range transactions {
		transaction := types.Transaction{MsgTx: tx}
		hash, _ := transaction.TxHash()
		if _, exists := existingTxHashes[hash]; exists {
			logger.Errorf("block contains duplicate transaction %v", hash)
			return ErrDuplicateTx
		}
		existingTxHashes[hash] = struct{}{}
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
func (chain *BlockChain) LoadUnspentUtxo(tx *types.Transaction) (*UtxoUnspentCache, error) {

	outPointMap := make(map[types.OutPoint]struct{})
	prevOut := types.OutPoint{Hash: *tx.Hash}
	for txOutIdx := range tx.MsgTx.Vout {
		prevOut.Index = uint32(txOutIdx)
		outPointMap[prevOut] = struct{}{}
	}
	if !IsCoinBase(tx.MsgTx) {
		for _, txIn := range tx.MsgTx.Vin {
			outPointMap[txIn.PrevOutPoint] = struct{}{}
		}
	}

	// Request the utxos from the point of view of the end of the main
	// chain.
	// uup := NewUtxoUnspentCache()
	uup := UtxoUnspentCachePool.Get().(*UtxoUnspentCache)
	UtxoUnspentCachePool.Put(uup)
	// TODO: add mutex?
	err := uup.LoadUtxoFromDB(chain.db, outPointMap)

	return uup, err
}

// CheckTransactionInputs check transaction inputs.
func (chain *BlockChain) CheckTransactionInputs(tx *types.Transaction, unspentUtxo *UtxoUnspentCache) (int64, error) {
	// Coinbase transactions have no inputs.
	if IsCoinBase(tx.MsgTx) {
		return 0, nil
	}

	var totalFeeIn int64
	for _, txIn := range tx.MsgTx.Vin {
		// Ensure the referenced input transaction is available.
		utxo := unspentUtxo.FindByOutPoint(txIn.PrevOutPoint)
		if utxo == nil || utxo.IsPacked {
			return 0, errors.New("utxo is not exist or has already been spent")
		}
		// if utxo is coinbase
		if utxo.IsCoinbase {
			originHeight := utxo.BlockHeight
			blocksSincePrev := chain.tail.Height - originHeight
			coinbaseMaturity := CoinbaseLib
			if blocksSincePrev < coinbaseMaturity {
				return 0, errors.New("tried to spend coinbase tx at height before required lib blocks")
			}
		}

		txInFee := utxo.Value
		if txInFee < 0 || txInFee > totalSupply {
			return 0, errors.New("Invalid txIn fee")
		}
		totalFeeIn += txInFee
		if totalFeeIn > totalSupply {
			return 0, errors.New("Invalid totalFeesIn")
		}
	}

	var totalFeeOut int64
	for _, txOut := range tx.MsgTx.Vout {
		totalFeeOut += txOut.Value
	}

	if totalFeeIn < totalFeeOut {
		return 0, errors.New("total value of all transaction inputs is less than the amount spent")
	}

	txFee := totalFeeIn - totalFeeOut
	return txFee, nil
}

// ValidateTransactionScripts verify crypto signatures for each input
func (chain *BlockChain) ValidateTransactionScripts(tx *types.Transaction, unspentUtxo *UtxoUnspentCache) error {
	txIns := tx.MsgTx.Vin
	txValItems := make([]*txValidateItem, 0, len(txIns))
	for txInIdx, txIn := range txIns {
		// Skip coinbases.
		if txIn.PrevOutPoint.Index == math.MaxUint32 {
			continue
		}

		txVI := &txValidateItem{
			txInIndex: txInIdx,
			txIn:      txIn,
			tx:        tx,
			// sigHashes: cachedHashes,
		}
		txValItems = append(txValItems, txVI)
	}

	// Validate all of the inputs.
	// validator := NewTxValidator(unspentUtxo, flags, sigCache, hashCache)
	// return validator.Validate(txValItems)
	return nil
}

// PackTxs packed txs and add them to block.
func (chain *BlockChain) PackTxs(block *types.Block) {
	pool := chain.txpool.pool
	blockUtxos := NewUtxoUnspentCache()
	var blockTxns []*types.MsgTx
	for pool.Len() > 0 {
		txwrap := heap.Pop(pool).(*TxWrap)
		tx := txwrap.tx
		unspentUtxoCache, err := chain.LoadUnspentUtxo(tx)
		if err != nil {
			continue
		}
		mergeUtxoCache(blockUtxos, unspentUtxoCache)
		// spent tx
		chain.spendTransaction(blockUtxos, tx, chain.tail.Height)
		blockTxns = append(blockTxns, tx.MsgTx)
	}

	merkles := util.CalcTxsHash(blockTxns)
	block.MsgBlock.Header.TxsRoot = *merkles
	for _, tx := range blockTxns {
		block.MsgBlock.Txs = append(block.MsgBlock.Txs, tx)
	}
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

func (chain *BlockChain) spendTransaction(blockUtxos *UtxoUnspentCache, tx *types.Transaction, height int) error {
	for _, txIn := range tx.MsgTx.Vin {
		utxowrap := blockUtxos.FindByOutPoint(txIn.PrevOutPoint)
		if utxowrap != nil {
			utxowrap.IsPacked = true
		}
	}

	blockUtxos.AddTxOuts(tx, height)
	return nil
}
