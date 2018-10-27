// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package chain

import (
	"math"
	"time"

	"github.com/BOXFoundation/boxd/core"
	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/crypto"
	"github.com/BOXFoundation/boxd/script"
	"github.com/BOXFoundation/boxd/util"
)

// LockTime represents the relative lock-time in seconds
type LockTime struct {
	Seconds     int64
	BlockHeight int32
}

func validateBlock(block *types.Block, timeSource util.MedianTimeSource) error {
	header := block.Header

	if err := validateBlockHeader(header, timeSource); err != nil {
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
	if !IsCoinBase(transactions[0]) {
		logger.Errorf("first transaction in block is not a coinbase")
		return core.ErrFirstTxNotCoinbase
	}

	// There should be only one coinbase.
	for i, tx := range transactions[1:] {
		if IsCoinBase(tx) {
			logger.Errorf("block contains second coinbase at index %d", i+1)
			return core.ErrMultipleCoinbases
		}
	}

	// checks each transaction.
	blockTime := block.Header.TimeStamp
	for _, tx := range transactions {
		if !IsTxFinalized(tx, block.Height, blockTime) {
			txHash, _ := tx.TxHash()
			logger.Errorf("block contains unfinalized transaction %v", txHash)
			return core.ErrUnfinalizedTx
		}
		if err := ValidateTransactionPreliminary(tx); err != nil {
			return err
		}
	}

	// Calculate merkle tree root and ensure it matches with the block header.
	// TODO: caching all of the transaction hashes in the block to speed up future hashing
	calculatedMerkleRoot := CalcTxsHash(transactions)
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

func validateBlockHeader(header *types.BlockHeader, timeSource util.MedianTimeSource) error {

	timestamp := time.Unix(0, header.TimeStamp)

	// Ensure the block time is not too far in the future.
	maxTimestamp := timeSource.AdjustedTime().Add(time.Second * MaxTimeOffsetSeconds)
	if timestamp.After(maxTimestamp) {
		logger.Errorf("block timestamp of %v is too far in the future", header.TimeStamp)
		return core.ErrTimeTooNew
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

func validateBlockScripts(utxoSet *UtxoSet, block *types.Block) error {
	// Skip coinbases.
	for _, tx := range block.Txs[1:] {
		if err := ValidateTxScripts(utxoSet, tx); err != nil {
			return err
		}
	}

	return nil
}

// ValidateTxScripts verifies unlocking script for each input to ensure it is authorized to spend the utxo
// Coinbase tx will not reach here
func ValidateTxScripts(utxoSet *UtxoSet, tx *types.Transaction) error {
	txHash, _ := tx.TxHash()
	for txInIdx, txIn := range tx.Vin {
		// Ensure the referenced input transaction exists and is not spent.
		utxo := utxoSet.FindUtxo(txIn.PrevOutPoint)
		if utxo == nil {
			logger.Errorf("output %v referenced from transaction %s:%d does not exist", txIn.PrevOutPoint, txHash, txInIdx)
			return core.ErrMissingTxOut
		}
		if utxo.IsSpent {
			logger.Errorf("output %v referenced from transaction %s:%d has already been spent", txIn.PrevOutPoint, txHash, txInIdx)
			return core.ErrMissingTxOut
		}

		prevScriptPubKey := script.NewScriptFromBytes(utxo.Output.ScriptPubKey)
		scriptSig := script.NewScriptFromBytes(txIn.ScriptSig)

		if err := script.Validate(scriptSig, prevScriptPubKey, tx, txInIdx); err != nil {
			return err
		}
	}

	return nil
}

// ValidateTxInputs validates the inputs of a tx.
// Returns the total tx fee.
func ValidateTxInputs(utxoSet *UtxoSet, tx *types.Transaction, txHeight int32) (int64, error) {
	// Coinbase tx needs no inputs.
	if IsCoinBase(tx) {
		return 0, nil
	}

	txHash, _ := tx.TxHash()
	var totalInputAmount int64
	for txInIndex, txIn := range tx.Vin {
		// Ensure the referenced input transaction exists and is not spent.
		utxo := utxoSet.FindUtxo(txIn.PrevOutPoint)
		if utxo == nil || utxo.IsSpent {
			logger.Errorf("output %v referenced from transaction %s:%d does not exist or"+
				"has already been spent", txIn.PrevOutPoint, txHash, txInIndex)
			return 0, core.ErrMissingTxOut
		}

		// Immature coinbase coins cannot be spent.
		if utxo.IsCoinBase {
			originHeight := utxo.BlockHeight
			blocksSincePrev := txHeight - originHeight
			if blocksSincePrev < CoinbaseMaturity {
				logger.Errorf("tried to spend coinbase transaction output %v from height %v "+
					"at height %v before required maturity of %v blocks", txIn.PrevOutPoint,
					originHeight, txHeight, CoinbaseMaturity)
				return 0, core.ErrImmatureSpend
			}
		}

		// Tx amount must be in range.
		utxoAmount := utxo.Value()
		if utxoAmount < 0 {
			logger.Errorf("transaction output has negative value of %v", utxoAmount)
			return 0, core.ErrBadTxOutValue
		}
		if utxoAmount > TotalSupply {
			logger.Errorf("transaction output value of %v is higher than max allowed value of %v", utxoAmount, TotalSupply)
			return 0, core.ErrBadTxOutValue
		}

		// Total tx amount must also be in range. Also, we check for overflow.
		lastAmount := totalInputAmount
		totalInputAmount += utxoAmount
		if totalInputAmount < lastAmount || totalInputAmount > TotalSupply {
			logger.Errorf("total value of all transaction inputs is %v which is higher than max "+
				"allowed value of %v", totalInputAmount, TotalSupply)
			return 0, core.ErrBadTxOutValue
		}
	}

	// Sum the total output amount.
	var totalOutputAmount int64
	for _, txOut := range tx.Vout {
		totalOutputAmount += txOut.Value
	}

	// Tx total outputs must not exceed total inputs.
	if totalInputAmount < totalOutputAmount {
		logger.Errorf("total value of all transaction outputs for "+
			"transaction %v is %v, which exceeds the input amount "+
			"of %v", txHash, totalOutputAmount, totalInputAmount)
		return 0, core.ErrSpendTooHigh
	}

	txFee := totalInputAmount - totalOutputAmount
	return txFee, nil
}

// ValidateTransactionPreliminary performs some preliminary checks on a transaction to
// ensure it is sane. These checks are context free.
func ValidateTransactionPreliminary(tx *types.Transaction) error {
	// A transaction must have at least one input.
	if len(tx.Vin) == 0 {
		return core.ErrNoTxInputs
	}

	// A transaction must have at least one output.
	if len(tx.Vout) == 0 {
		return core.ErrNoTxOutputs
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
			return core.ErrBadTxOutValue
		}
		if value > TotalSupply {
			logger.Errorf("transaction output value of %v is "+
				"higher than max allowed value of %v", TotalSupply)
			return core.ErrBadTxOutValue
		}

		// Two's complement int64 overflow guarantees that any overflow
		// is detected and reported.
		totalValue += value
		if totalValue < 0 {
			logger.Errorf("total value of all transaction outputs overflows %v", totalValue)
			return core.ErrBadTxOutValue
		}
		if totalValue > TotalSupply {
			logger.Errorf("total value of all transaction "+
				"outputs is %v which is higher than max "+
				"allowed value of %v", totalValue, TotalSupply)
			return core.ErrBadTxOutValue
		}
	}

	// Check for duplicate transaction inputs.
	existingOutPoints := make(map[types.OutPoint]struct{})
	for _, txIn := range tx.Vin {
		if _, exists := existingOutPoints[txIn.PrevOutPoint]; exists {
			return core.ErrDuplicateTxInputs
		}
		existingOutPoints[txIn.PrevOutPoint] = struct{}{}
	}

	if IsCoinBase(tx) {
		// Coinbase script length must be between min and max length.
		slen := len(tx.Vin[0].ScriptSig)
		if slen < core.MinCoinbaseScriptLen || slen > core.MaxCoinbaseScriptLen {
			logger.Errorf("coinbase transaction script length "+
				"of %d is out of range (min: %d, max: %d)",
				slen, core.MinCoinbaseScriptLen, core.MaxCoinbaseScriptLen)
			return core.ErrBadCoinbaseScriptLen
		}
	} else {
		// Previous transaction outputs referenced by the inputs to this
		// transaction must not be null.
		for _, txIn := range tx.Vin {
			if isNullOutPoint(&txIn.PrevOutPoint) {
				return core.ErrBadTxInput
			}
		}
	}

	return nil
}
