// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package utils

import (
	"errors"
	"math"

	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/crypto"
	"github.com/BOXFoundation/boxd/log"
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

	// baseSubsidy is the starting subsidy amount for mined blocks.
	// This value is halved every SubsidyReductionInterval blocks.
	baseSubsidy = (int64)(50 * math.Pow10(decimals))
)

// error
var (
	ErrNoTxInputs           = errors.New("Transaction has no inputs")
	ErrNoTxOutputs          = errors.New("Transaction has no outputs")
	ErrBadTxOutValue        = errors.New("Invalid output value")
	ErrDuplicateTxInputs    = errors.New("Transaction contains duplicate inputs")
	ErrBadCoinbaseScriptLen = errors.New("Coinbase scriptSig out of range")
	ErrBadTxInput           = errors.New("Transaction input refers to null out point")
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
		if value > TotalSupply {
			logger.Errorf("transaction output value of %v is "+
				"higher than max allowed value of %v", TotalSupply)
			return ErrBadTxOutValue
		}

		// Two's complement int64 overflow guarantees that any overflow
		// is detected and reported.
		totalValue += value
		if totalValue < 0 {
			logger.Errorf("total value of all transaction outputs overflows %v", totalValue)
			return ErrBadTxOutValue
		}
		if totalValue > TotalSupply {
			logger.Errorf("total value of all transaction "+
				"outputs is %v which is higher than max "+
				"allowed value of %v", totalValue, TotalSupply)
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
