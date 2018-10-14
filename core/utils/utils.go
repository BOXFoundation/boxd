// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package utils

import (
	"math"

	"github.com/BOXFoundation/boxd/core"
	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/crypto"
	"github.com/BOXFoundation/boxd/log"
	"github.com/BOXFoundation/boxd/script"
)

// const defines constants
const (
	// subsidyReductionInterval is the interval of blocks before the subsidy is reduced.
	subsidyReductionInterval = 210000

	// decimals is the number of digits after decimal point of value/amount
	decimals = 8

	// MinCoinbaseScriptLen is the minimum length a coinbase script can be.
	MinCoinbaseScriptLen = 2

	// MaxCoinbaseScriptLen is the maximum length a coinbase script can be.
	MaxCoinbaseScriptLen = 1000
)

var (
	// zeroHash is the zero value for a hash
	zeroHash crypto.HashType

	// TotalSupply is the total supply of box: 3 billion
	TotalSupply = (int64)(3e9 * math.Pow10(decimals))

	// coinbase only spendable after this many blocks
	coinbaseMaturity = (int32)(0)

	// baseSubsidy is the starting subsidy amount for mined blocks.
	// This value is halved every SubsidyReductionInterval blocks.
	baseSubsidy = (int64)(50 * math.Pow10(decimals))
)

var logger = log.NewLogger("utils") // logger

// isNullOutPoint determines whether or not a previous transaction output point is set.
func isNullOutPoint(outPoint *types.OutPoint) bool {
	return outPoint.Index == math.MaxUint32 && outPoint.Hash == zeroHash
}

// IsCoinBase determines whether or not a transaction is a coinbase.  A coinbase
// is a special transaction created by miners that has no inputs.  This is
// represented in the block chain by a transaction with a single input that has
// a previous output transaction index set to the maximum value along with a zero hash.
func IsCoinBase(tx *types.Transaction) bool {
	// A coin base must only have one transaction input.
	if len(tx.Vin) != 1 {
		return false
	}

	// The previous output of a coin base must have a max value index and a zero hash.
	return isNullOutPoint(&tx.Vin[0].PrevOutPoint)
}

// CalcBlockSubsidy returns the subsidy amount a block at the provided height
// should have. This is mainly used for determining how much the coinbase for
// newly generated blocks awards as well as validating the coinbase for blocks
// has the expected value.
//
// The subsidy is halved every subsidyReductionInterval blocks.  Mathematically
// this is: baseSubsidy / 2^(height/subsidyReductionInterval)
func CalcBlockSubsidy(height int32) int64 {
	// Equivalent to: baseSubsidy / 2^(height/subsidyHalvingInterval)
	return baseSubsidy >> uint(height/subsidyReductionInterval)
}

// SanityCheckTransaction performs some preliminary checks on a transaction to
// ensure it is sane. These checks are context free.
func SanityCheckTransaction(tx *types.Transaction) error {
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
		if slen < MinCoinbaseScriptLen || slen > MaxCoinbaseScriptLen {
			logger.Errorf("coinbase transaction script length "+
				"of %d is out of range (min: %d, max: %d)",
				slen, MinCoinbaseScriptLen, MaxCoinbaseScriptLen)
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

// ValidateTransactionScripts verify crypto signatures for each input
func ValidateTransactionScripts(tx *types.Transaction) error {
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
			if blocksSincePrev < coinbaseMaturity {
				logger.Errorf("tried to spend coinbase transaction output %v from height %v "+
					"at height %v before required maturity of %v blocks", txIn.PrevOutPoint,
					originHeight, txHeight, coinbaseMaturity)
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

// CreateCoinbaseTx creates a coinbase give miner address and block height
func CreateCoinbaseTx(addr types.Address, blockHeight int32) (*types.Transaction, error) {
	var pkScript []byte
	var err error
	blockReward := CalcBlockSubsidy(blockHeight)
	coinbaseScript, err := script.StandardCoinbaseScript(blockHeight)
	if err != nil {
		return nil, err
	}
	if addr != nil {
		pkScript, err = script.PayToPubKeyHashScript(addr.ScriptAddress())
		if err != nil {
			return nil, err
		}
	} else {
		scriptBuilder := script.NewBuilder()
		pkScript, err = scriptBuilder.AddOp(script.OPTRUE).Script()
		if err != nil {
			return nil, err
		}
	}

	tx := &types.Transaction{
		Version: 1,
		Vin: []*types.TxIn{
			{
				PrevOutPoint: types.OutPoint{
					Hash:  crypto.HashType{},
					Index: 0xffffffff,
				},
				ScriptSig: coinbaseScript,
				Sequence:  0xffffffff,
			},
		},
		Vout: []*types.TxOut{
			{
				Value:        blockReward,
				ScriptPubKey: pkScript,
			},
		},
	}
	return tx, nil
}
