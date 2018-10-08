// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package core

import (
	"errors"

	"github.com/BOXFoundation/boxd/core/types"
)

// error
var (
	ErrTxOutIndexOob   = errors.New("Transaction output index out of bound")
	ErrAddExistingUtxo = errors.New("Trying to add utxo already existed")
)

// UtxoEntry contains info about utxo
type UtxoEntry struct {
	Output types.TxOut
	// height of block containing the tx output
	BlockHeight int32
	// is this utxo inside a coinbase tx
	IsCoinBase bool
	// is this utxo spent
	IsSpent bool
}

// Value returns utxo amount
func (u *UtxoEntry) Value() int64 {
	return u.Output.Value
}

// UtxoSet contains all utxos
type UtxoSet struct {
	utxoMap map[types.OutPoint]*UtxoEntry
}

// NewUtxoSet new utxo set
func NewUtxoSet() *UtxoSet {
	return &UtxoSet{
		utxoMap: make(map[types.OutPoint]*UtxoEntry),
	}
}

// FindUtxo returns information about an outpoint.
func (u *UtxoSet) FindUtxo(outPoint types.OutPoint) *UtxoEntry {
	logger.Debugf("Find utxo: %+v", outPoint)
	return u.utxoMap[outPoint]
}

// AddUtxo adds a utxo
func (u *UtxoSet) AddUtxo(tx *types.Transaction, txOutIdx uint32, blockHeight int32) error {
	logger.Debugf("Add utxo tx info: %+v, index: %d", tx, txOutIdx)
	// Index out of bound
	if txOutIdx >= uint32(len(tx.Vout)) {
		return ErrTxOutIndexOob
	}

	txHash, _ := tx.TxHash()
	outPoint := types.OutPoint{Hash: *txHash, Index: txOutIdx}
	if utxoEntry := u.utxoMap[outPoint]; utxoEntry != nil {
		return ErrAddExistingUtxo
	}
	utxoEntry := UtxoEntry{*tx.Vout[txOutIdx], blockHeight, IsCoinBase(tx), false}
	u.utxoMap[outPoint] = &utxoEntry
	return nil
}

// RemoveUtxo removes a utxo. We do not actually remove the entry in case it has to be
// recovered later and we do not have all info, such as block height
func (u *UtxoSet) RemoveUtxo(outPoint types.OutPoint) {
	logger.Debugf("Remove utxo: %+v", outPoint)
	utxoEntry := u.utxoMap[outPoint]
	if utxoEntry == nil {
		return
	}
	utxoEntry.IsSpent = true
}

// ApplyTx updates utxos with the passed tx: adds all utxos in outputs and delete all utxos in inputs.
func (u *UtxoSet) ApplyTx(tx *types.Transaction, blockHeight int32) error {
	// Add new utxos
	for txOutIdx := range tx.Vout {
		if err := u.AddUtxo(tx, (uint32)(txOutIdx), blockHeight); err != nil {
			return err
		}
	}

	// Coinbase transaction doesn't spend any utxo.
	if IsCoinBase(tx) {
		return nil
	}

	// Spend the referenced utxos
	for _, txIn := range tx.Vin {
		u.RemoveUtxo(txIn.PrevOutPoint)
	}
	return nil
}

// ApplyBlock updates utxos with all transactions in the passed block
func (u *UtxoSet) ApplyBlock(block *types.Block) error {
	txs := block.Txs
	for _, tx := range txs {
		if err := u.ApplyTx(tx, block.Height); err != nil {
			return err
		}
	}
	return nil
}

// RevertTx updates utxos with the passed tx: delete all utxos in outputs and add all utxos in inputs.
// It undoes the effect of ApplyTx on utxo set
func (u *UtxoSet) RevertTx(tx *types.Transaction, blockHeight int32) error {
	txHash, _ := tx.TxHash()

	// Remove added utxos
	for txOutIdx := range tx.Vout {
		u.RemoveUtxo(types.OutPoint{Hash: *txHash, Index: (uint32)(txOutIdx)})
	}

	// Coinbase transaction doesn't spend any utxo.
	if IsCoinBase(tx) {
		return nil
	}

	// "Unspend" the referenced utxos
	for _, txIn := range tx.Vin {
		utxoEntry := u.utxoMap[txIn.PrevOutPoint]
		if utxoEntry == nil {
			logger.Panicf("Trying to unspend non-existing spent output %v", txIn.PrevOutPoint)
		}
		utxoEntry.IsSpent = false
	}
	return nil
}

// RevertBlock undoes utxo changes made with all the transactions in the passed block
// It undoes the effect of ApplyBlock on utxo set
func (u *UtxoSet) RevertBlock(block *types.Block) error {
	// Loop backwards through all transactions so everything is unspent in reverse order.
	// This is necessary since transactions later in a block can spend from previous ones.
	txs := block.Txs
	for txIdx := len(txs) - 1; txIdx >= 0; txIdx-- {
		tx := txs[txIdx]
		if err := u.RevertTx(tx, block.Height); err != nil {
			return err
		}
	}
	return nil
}
