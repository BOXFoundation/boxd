// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package chain

import (
	"bytes"
	"math"
	"os"

	"github.com/BOXFoundation/boxd/core"
	"github.com/BOXFoundation/boxd/core/abi"
	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/crypto"
	"github.com/BOXFoundation/boxd/script"
	"github.com/BOXFoundation/boxd/util"
)

var (
	// zeroHash is the zero value for a hash
	zeroHash crypto.HashType

	// TotalSupply is the total supply of box: 3 billion
	TotalSupply = (uint64)(3e9 * core.DuPerBox)

	// CoinbaseMaturity coinbase only spendable after this many blocks
	CoinbaseMaturity = (uint32)(0)

	// BaseSubsidy is the starting subsidy amount for mined blocks.
	// This value is halved every SubsidyReductionInterval blocks.
	BaseSubsidy = (uint64)(3.17 * core.DuPerBox)
)

// isNullOutPoint determines whether or not a previous transaction output point is set.
func isNullOutPoint(outPoint *types.OutPoint) bool {
	return outPoint.Index == math.MaxUint32 && outPoint.Hash == zeroHash
}

// IsInternalContract determines whether or not a transaction is a internal contract tx.
func IsInternalContract(tx *types.Transaction) bool {
	if len(tx.Vin) != 1 {
		return false
	}
	outPoint := &tx.Vin[0].PrevOutPoint
	return outPoint.Index == math.MaxUint32-1 && outPoint.Hash == zeroHash
}

// IsCoinBase determines whether or not a transaction is a coinbase.
func IsCoinBase(tx *types.Transaction) bool {
	// A coin base must only have one transaction input.
	if len(tx.Vin) != 1 {
		return false
	}
	outPoint := &tx.Vin[0].PrevOutPoint
	return outPoint.Index == math.MaxUint32 && outPoint.Hash == zeroHash

	// The previous output of a coin base must have a max value index and a zero hash.
	// return isNullOutPoint(&tx.Vin[0].PrevOutPoint)
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
func CalcBlockSubsidy(height uint32) uint64 {
	return BaseSubsidy >> uint(height/core.SubsidyReductionInterval)
}

// CreateCoinbaseTx creates a coinbase give bookkeeper address and block height
func CreateCoinbaseTx(addr []byte, blockHeight uint32) (*types.Transaction, error) {
	var pkScript []byte
	blockReward := CalcBlockSubsidy(blockHeight)
	coinbaseScriptSig := script.StandardCoinbaseSignatureScript(blockHeight)
	pkScript = *script.PayToPubKeyHashScript(addr)

	tx := &types.Transaction{
		Version: 1,
		Vin: []*types.TxIn{
			{
				PrevOutPoint: *types.NewNullOutPoint(),
				ScriptSig:    *coinbaseScriptSig,
				Sequence:     math.MaxUint32,
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

// ReadAbi read genesis abi file.
func ReadAbi(filename string) (*abi.ABI, error) {

	abiFile, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer abiFile.Close()
	abiObj, err := abi.JSON(abiFile)
	if err != nil {
		return nil, err
	}
	return &abiObj, err
}

// MarshalTxIndex writes Tx height and index to bytes
func MarshalTxIndex(height, index uint32) (data []byte, err error) {
	var buf bytes.Buffer
	if err := util.WriteUint32(&buf, height); err != nil {
		return nil, err
	}
	if err := util.WriteUint32(&buf, index); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// UnmarshalTxIndex return tx index from bytes
func UnmarshalTxIndex(data []byte) (height uint32, index uint32, err error) {
	buf := bytes.NewBuffer(data)
	if height, err = util.ReadUint32(buf); err != nil {
		return
	}
	if index, err = util.ReadUint32(buf); err != nil {
		return
	}
	return
}

// MarshalMissData writes miss rate data to bytes
func MarshalMissData(height, miss uint32, ts int64) (data []byte, err error) {
	var buf bytes.Buffer
	if err := util.WriteUint32(&buf, height); err != nil {
		return nil, err
	}
	if err := util.WriteUint32(&buf, miss); err != nil {
		return nil, err
	}
	if err := util.WriteInt64(&buf, ts); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// UnmarshalMissData return tx index from bytes
func UnmarshalMissData(data []byte) (height uint32, miss uint32, ts int64, err error) {
	buf := bytes.NewBuffer(data)
	if height, err = util.ReadUint32(buf); err != nil {
		return
	}
	if miss, err = util.ReadUint32(buf); err != nil {
		return
	}
	ts, err = util.ReadInt64(buf)
	return
}

// FetchOutPointOwner fetchs owner of outpoint in utxo set
func FetchOutPointOwner(op *types.OutPoint, utxoSet *UtxoSet) (types.Address, error) {
	utxoWrap := utxoSet.FindUtxo(*op)
	if utxoWrap == nil {
		return nil, core.ErrUtxoNotFound
	}
	address, err := script.NewScriptFromBytes(utxoWrap.Script()).ExtractAddress()
	if err != nil {
		return nil, err
	}
	return address, nil
}
