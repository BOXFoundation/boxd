// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package chain

import (
	"math"
	"sort"
	"time"

	"github.com/BOXFoundation/boxd/core"
	"github.com/BOXFoundation/boxd/core/pb"
	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/crypto"
	"github.com/BOXFoundation/boxd/script"
	"github.com/BOXFoundation/boxd/util"
)

var (
	// zeroHash is the zero value for a hash
	zeroHash crypto.HashType

	// TotalSupply is the total supply of box: 3 billion
	TotalSupply = (int64)(3e9 * math.Pow10(core.Decimals))

	// CoinbaseMaturity coinbase only spendable after this many blocks
	CoinbaseMaturity = (int32)(0)

	// BaseSubsidy is the starting subsidy amount for mined blocks.
	// This value is halved every SubsidyReductionInterval blocks.
	BaseSubsidy = (int64)(50 * math.Pow10(core.Decimals))
)

// isNullOutPoint determines whether or not a previous transaction output point is set.
func isNullOutPoint(outPoint *types.OutPoint) bool {
	return outPoint.Index == math.MaxUint32 && outPoint.Hash == zeroHash
}

// IsCoinBase determines whether or not a transaction is a coinbase.
func IsCoinBase(tx *types.Transaction) bool {
	// A coin base must only have one transaction input.
	if len(tx.Vin) != 1 {
		return false
	}

	// The previous output of a coin base must have a max value index and a zero hash.
	return isNullOutPoint(&tx.Vin[0].PrevOutPoint)
}

// CalcTxsHash calculate txsHash in block.
func CalcTxsHash(txs []*types.Transaction) *crypto.HashType {

	hashs := make([]*crypto.HashType, len(txs))
	for index := range txs {
		hash, _ := txs[index].TxHash()
		hashs[index] = hash
	}
	txsHash := util.BuildMerkleRoot(hashs)
	return txsHash[len(txsHash)-1]
}

// CalcBlockSubsidy returns the subsidy amount a block at the provided height should have.
func CalcBlockSubsidy(height int32) int64 {
	return BaseSubsidy >> uint(height/core.SubsidyReductionInterval)
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
		pkScript, err = scriptBuilder.AddOp(byte(script.OPTRUE)).Script()
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
		Vout: []*corepb.TxOut{
			{
				Value:        blockReward,
				ScriptPubKey: pkScript,
			},
		},
	}
	return tx, nil
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

func sequenceLockActive(timeLock *LockTime, blockHeight int32, medianTimePast time.Time) bool {
	return timeLock.Seconds < medianTimePast.Unix() && timeLock.BlockHeight < blockHeight
}

func (chain *BlockChain) calcLockTime(utxoSet *UtxoSet, block *types.Block, tx *types.Transaction) (*LockTime, error) {

	lockTime := &LockTime{Seconds: -1, BlockHeight: -1}

	// lock-time does not apply to coinbase tx.
	if IsCoinBase(tx) {
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
