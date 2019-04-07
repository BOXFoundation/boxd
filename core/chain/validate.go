// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package chain

import (
	"math"
	"reflect"
	"runtime"
	"strings"
	"time"

	"github.com/BOXFoundation/boxd/core"
	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/crypto"
	"github.com/BOXFoundation/boxd/script"
)

// LockTime represents the relative lock-time in seconds
type LockTime struct {
	Seconds     int64
	BlockHeight uint32
}

// VerifyBlockTimeOut refuse to accept a block with wrong timestamp.
func VerifyBlockTimeOut(block *types.Block) error {
	now := time.Now().Unix()
	if now-block.Header.TimeStamp > core.MaxBlockTimeOut {
		logger.Warnf("The block is timeout. Now: %d BlockTimeStamp: %d Timeout: %d Hash: %v", now, block.Header.TimeStamp, (now - block.Header.TimeStamp), block.Hash.String())
		return core.ErrBlockTimeOut
	} else if now < block.Header.TimeStamp {
		logger.Warnf("The block is a future block. Now: %d BlockTimeStamp: %d Timeout: %d Hash: %v", now, block.Header.TimeStamp, (now - block.Header.TimeStamp), block.Hash.String())
		return core.ErrFutureBlock
	}
	return nil
}

func validateBlock(block *types.Block) error {
	header := block.Header

	// Can't have no tx
	numTx := len(block.Txs)
	if numTx == 0 {
		logger.Errorf("block does not contain any transactions")
		return core.ErrNoTransactions
	}

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
	existingOutPoints := make(map[types.OutPoint]struct{})
	for _, tx := range transactions {
		if !IsTxFinalized(tx, block.Header.Height, blockTime) {
			txHash, _ := tx.TxHash()
			logger.Errorf("block contains unfinalized transaction %v", txHash)
			return core.ErrUnfinalizedTx
		}
		if err := ValidateTransactionPreliminary(tx); err != nil {
			return err
		}

		// Check for double spent in transactions in the same block.
		for _, txIn := range tx.Vin {
			if _, exists := existingOutPoints[txIn.PrevOutPoint]; exists {
				return core.ErrDoubleSpendTx
			}
			existingOutPoints[txIn.PrevOutPoint] = struct{}{}
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

// IsTxFinalized checks if a transaction is finalized.
func IsTxFinalized(tx *types.Transaction, blockHeight uint32, blockTime int64) bool {
	// The tx is finalized if lock time is 0.
	lockTime := tx.LockTime
	if lockTime == 0 {
		return true
	}

	// When lock time field is less than the threshold, it is a block height.
	// Otherwise it is a timestamp.
	blockTimeOrHeight := int64(0)
	if lockTime < script.LockTimeThreshold {
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
	var scriptItems []*ScriptItem

	// Skip coinbases.
	for _, tx := range block.Txs[1:] {
		txScriptItems, err := CheckTxScripts(utxoSet, tx, true /* do not validate script here */)
		if err != nil {
			return err
		}
		scriptItems = append(scriptItems, txScriptItems...)
	}

	return newScriptValidator().validate(scriptItems)
}

type scriptValidator struct {
	scriptItemCh chan *ScriptItem
	resultCh     chan error
	quitCh       chan struct{}
}

func newScriptValidator() *scriptValidator {
	return &scriptValidator{
		scriptItemCh: make(chan *ScriptItem),
		resultCh:     make(chan error),
		quitCh:       make(chan struct{}),
	}
}

func (sv *scriptValidator) validate(scriptItems []*ScriptItem) error {
	totalValidatorWorkers := runtime.NumCPU()

	for i := 0; i < totalValidatorWorkers; i++ {
		go sv.doValidate()
	}

	totalItems := len(scriptItems)
	nextItemIdx := 0
	validatedItems := 0

	for validatedItems < totalItems {
		var scriptItemCh chan *ScriptItem
		var scriptItem *ScriptItem
		if nextItemIdx < totalItems {
			scriptItem = scriptItems[nextItemIdx]
			scriptItemCh = sv.scriptItemCh
		}

		select {
		// This will be ignored by "select" if scriptItemCh is nil
		case scriptItemCh <- scriptItem:
			nextItemIdx++

		case err := <-sv.resultCh:
			validatedItems++
			if err != nil {
				close(sv.quitCh)
				return err
			}
		}
	}

	close(sv.quitCh)
	return nil
}

// Worker
func (sv *scriptValidator) doValidate() {
	for {
		select {
		case it := <-sv.scriptItemCh:
			if err := script.Validate(it.scriptSig, it.prevScriptPubKey, it.tx, it.txInIdx); err != nil {
				sv.resultCh <- err
				return
			}
			sv.resultCh <- nil
		case <-sv.quitCh:
			return
		}
	}
}

// ScriptItem saves scripts for parallel validation
type ScriptItem struct {
	tx               *types.Transaction
	txInIdx          int
	scriptSig        *script.Script
	prevScriptPubKey *script.Script
}

// CheckTxScripts verifies unlocking script for each input to ensure it is authorized to spend the utxo
// Coinbase tx will not reach here
func CheckTxScripts(utxoSet *UtxoSet, tx *types.Transaction, skipValidation bool) ([]*ScriptItem, error) {
	var scriptItems []*ScriptItem
	txHash, _ := tx.TxHash()
	for txInIdx, txIn := range tx.Vin {
		// Ensure the referenced input transaction exists and is not spent.
		utxo := utxoSet.FindUtxo(txIn.PrevOutPoint)
		if utxo == nil {
			logger.Errorf("output %v referenced from transaction %s:%d does not exist", txIn.PrevOutPoint, txHash, txInIdx)
			return nil, core.ErrMissingTxOut
		}
		if utxo.IsSpent() {
			logger.Errorf("output %v referenced from transaction %s:%d has already been spent", txIn.PrevOutPoint, txHash, txInIdx)
			return nil, core.ErrMissingTxOut
		}

		prevScriptPubKey := script.NewScriptFromBytes(utxo.Script())
		scriptSig := script.NewScriptFromBytes(txIn.ScriptSig)

		if skipValidation {
			scriptItems = append(scriptItems, &ScriptItem{tx, txInIdx, scriptSig, prevScriptPubKey})
			continue
		}

		if err := script.Validate(scriptSig, prevScriptPubKey, tx, txInIdx); err != nil {
			return nil, err
		}
	}

	return scriptItems, nil
}

// ValidateTxInputs validates the inputs of a tx.
// Returns the total tx fee.
func ValidateTxInputs(utxoSet *UtxoSet, tx *types.Transaction, txHeight uint32) (uint64, error) {
	// Coinbase tx needs no inputs.
	if IsCoinBase(tx) {
		return 0, nil
	}

	txHash, _ := tx.TxHash()
	var totalInputAmount uint64
	tokenInputAmounts := make(map[script.TokenID]uint64)
	for txInIndex, txIn := range tx.Vin {
		// Ensure the referenced input transaction exists and is not spent.
		utxo := utxoSet.FindUtxo(txIn.PrevOutPoint)
		if utxo == nil || utxo.IsSpent() {
			logger.Errorf("output %v referenced from transaction %s:%d does not exist or "+
				"has already been spent", txIn.PrevOutPoint, txHash, txInIndex)
			return 0, core.ErrMissingTxOut
		}

		// Immature coinbase coins cannot be spent.
		if utxo.IsCoinBase() {
			originHeight := utxo.Height()
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

		// token tx input amount
		scriptPubKey := script.NewScriptFromBytes(utxo.Script())
		if scriptPubKey.IsTokenIssue() {
			tokenID := script.NewTokenID(txIn.PrevOutPoint.Hash, txIn.PrevOutPoint.Index)
			// no need to check error since it will not err
			params, _ := scriptPubKey.GetIssueParams()
			// case insensitive comparison
			if strings.EqualFold(params.Name, "box") {
				logger.Errorf("Token name %s cannot be box", params.Name)
				return 0, core.ErrTokenInvalidName
			}
			tokenInputAmounts[tokenID] += params.TotalSupply * uint64(math.Pow10(int(params.Decimals)))
		} else if scriptPubKey.IsTokenTransfer() {
			// no need to check error since it will not err
			params, _ := scriptPubKey.GetTransferParams()
			tokenID := script.NewTokenID(params.Hash, params.Index)
			tokenInputAmounts[tokenID] += params.Amount
		}
	}

	// Sum the total output amount.
	var totalOutputAmount uint64
	tokenOutputAmounts := make(map[script.TokenID]uint64)
	for _, txOut := range tx.Vout {
		totalOutputAmount += txOut.Value
		// token tx output amount
		scriptPubKey := script.NewScriptFromBytes(txOut.GetScriptPubKey())
		// do not count token issued
		if scriptPubKey.IsTokenTransfer() {
			// no need to check error since it will not err
			params, _ := scriptPubKey.GetTransferParams()
			tokenID := script.NewTokenID(params.Hash, params.Index)
			tokenOutputAmounts[tokenID] += params.Amount
		}
	}

	// Tx total outputs must not exceed total inputs.
	if totalInputAmount < totalOutputAmount {
		logger.Errorf("total value of all transaction outputs for "+
			"transaction %v is %v, which exceeds the input amount "+
			"of %v", txHash, totalOutputAmount, totalInputAmount)
		return 0, core.ErrSpendTooHigh
	}

	if !reflect.DeepEqual(tokenOutputAmounts, tokenInputAmounts) {
		logger.Errorf("total value of all token outputs for "+
			"transaction %v is %v, differs from the input amount "+
			"of %v", txHash, tokenOutputAmounts, tokenInputAmounts)
		return 0, core.ErrTokenInputsOutputNotEqual
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
	var totalValue uint64
	for _, txOut := range tx.Vout {
		value := txOut.Value
		if value > TotalSupply {
			logger.Errorf("transaction output value of %v is "+
				"higher than max allowed value of %v", TotalSupply)
			return core.ErrBadTxOutValue
		}

		totalValue += value
		if totalValue < value {
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
