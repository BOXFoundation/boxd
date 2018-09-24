// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package core

import (
	"errors"

	"github.com/BOXFoundation/Quicksilver/core/types"
)

// error
var (
	ErrTxOutIndexOob   = errors.New("Transaction output index out of bound")
	ErrAddExistingUtxo = errors.New("Trying to add utxo already existed")
)

// UtxoEntry contains info about utxo
type UtxoEntry struct {
	output types.TxOut
	// height of block containing the tx output
	BlockHeight int32
	// is this utxo inside a coinbase tx
	IsCoinBase bool
}

// Value returns utxo amount
func (u *UtxoEntry) Value() int64 {
	return u.output.Value
}

// UtxoSet contains all utxos
type UtxoSet struct {
	utxoMap map[types.OutPoint]*UtxoEntry
}

// FindUtxo returns information about an outpoint.
// It returns nil if the outpoint does not exist, i.e, the output has been spent.
func (u *UtxoSet) FindUtxo(outPoint types.OutPoint) *UtxoEntry {
	return u.utxoMap[outPoint]
}

// AddUtxo adds a utxo
func (u *UtxoSet) AddUtxo(tx *types.MsgTx, txOutIdx uint32, blockHeight int32) error {
	// Index out of bound
	if txOutIdx >= uint32(len(tx.Vout)) {
		return ErrTxOutIndexOob
	}

	txHash, _ := tx.MsgTxHash()
	outPoint := types.OutPoint{Hash: *txHash, Index: txOutIdx}
	if utxoEntry := u.utxoMap[outPoint]; utxoEntry != nil {
		return ErrAddExistingUtxo
	}
	utxoEntry := UtxoEntry{*tx.Vout[txOutIdx], blockHeight, IsCoinBase(tx)}
	u.utxoMap[outPoint] = &utxoEntry
	return nil
}

// RemoveUtxo removes a utxo
func (u *UtxoSet) RemoveUtxo(outPoint types.OutPoint) {
	delete(u.utxoMap, outPoint)
}

// ApplyTx updates utxos with the passed tx: adds all utxos in outputs and delete all utxos in inputs.
func (u *UtxoSet) ApplyTx(tx *types.MsgTx, blockHeight int32) error {
	// Add new utxos
	for txOutIdx := range tx.Vout {
		if err := u.AddUtxo(tx, (uint32)(txOutIdx), blockHeight); err != nil {
			return err
		}
	}

	// Coinbase transaction doesn't spend any utxo.
	if !IsCoinBase(tx) {
		// Spend the referenced utxos
		for _, txIn := range tx.Vin {
			u.RemoveUtxo(txIn.PrevOutPoint)
		}
	}

	return nil
}
