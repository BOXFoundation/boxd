// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package chain

import (
	"sync"

	"github.com/BOXFoundation/boxd/core"
	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/crypto"
	"github.com/BOXFoundation/boxd/script"
	"github.com/BOXFoundation/boxd/storage"
	"github.com/BOXFoundation/boxd/util"
)

// UtxoSet contains all utxos
type UtxoSet struct {
	utxoMap types.UtxoMap
}

// NewUtxoSet new utxo set
func NewUtxoSet() *UtxoSet {
	return &UtxoSet{
		utxoMap: make(types.UtxoMap),
	}
}

// NewUtxoSetFromMap returns the underlying utxos as a map
func NewUtxoSetFromMap(utxoMap types.UtxoMap) *UtxoSet {
	return &UtxoSet{
		utxoMap: utxoMap,
	}
}

// GetUtxos returns the unspent utxos
func (u *UtxoSet) GetUtxos() types.UtxoMap {
	result := make(types.UtxoMap)
	for outPoint, utxoWrap := range u.utxoMap {
		if !utxoWrap.IsSpent {
			result[outPoint] = utxoWrap
		}
	}
	return result
}

// All returns all utxo contained including spent utxo
func (u *UtxoSet) All() types.UtxoMap {
	return u.utxoMap
}

// FindUtxo returns information about an outpoint.
func (u *UtxoSet) FindUtxo(outPoint types.OutPoint) *types.UtxoWrap {
	return u.utxoMap[outPoint]
}

// AddUtxo adds a tx's outputs as utxos
func (u *UtxoSet) AddUtxo(tx *types.Transaction, txOutIdx uint32, blockHeight uint32) error {
	// Index out of bound
	if txOutIdx >= uint32(len(tx.Vout)) {
		return core.ErrTxOutIndexOob
	}

	txHash, _ := tx.TxHash()
	outPoint := types.OutPoint{Hash: *txHash, Index: txOutIdx}
	if utxoWrap := u.utxoMap[outPoint]; utxoWrap != nil {
		return core.ErrAddExistingUtxo
	}
	utxoWrap := types.UtxoWrap{
		Output:      tx.Vout[txOutIdx],
		BlockHeight: blockHeight,
		IsCoinBase:  IsCoinBase(tx),
		IsModified:  true,
		IsSpent:     false,
	}
	u.utxoMap[outPoint] = &utxoWrap
	return nil
}

// SpendUtxo mark a utxo as the spent state.
func (u *UtxoSet) SpendUtxo(outPoint types.OutPoint) {
	utxoWrap := u.utxoMap[outPoint]
	if utxoWrap == nil {
		// Mark the utxo entry as spent so it could be deleted from db later
		utxoWrap = &types.UtxoWrap{}
		u.utxoMap[outPoint] = utxoWrap
	}
	utxoWrap.IsSpent = true
	utxoWrap.IsModified = true
}

// TxInputAmount returns total amount from tx's inputs
// Return 0 if a tx is not fully funded, i.e., if not all of its spending utxos exist
func (u *UtxoSet) TxInputAmount(tx *types.Transaction) uint64 {
	totalInputAmount := uint64(0)
	for _, txIn := range tx.Vin {
		utxo := u.FindUtxo(txIn.PrevOutPoint)
		if utxo == nil || utxo.IsSpent {
			return uint64(0)
		}
		totalInputAmount += utxo.Value()
	}

	return totalInputAmount
}

// TxWrap wrap transaction
type TxWrap struct {
	Tx             *types.Transaction
	AddedTimestamp int64
	Height         uint32
	FeePerKB       uint64
	IsScriptValid  bool
}

// GetExtendedTxUtxoSet returns tx's utxo set from both db & txs in spendableTxs
func GetExtendedTxUtxoSet(tx *types.Transaction, db storage.Table,
	spendableTxs *sync.Map) (*UtxoSet, error) {

	utxoSet := NewUtxoSet()
	if err := utxoSet.LoadTxUtxos(tx, db); err != nil {
		return nil, err
	}

	// Outputs of existing txs in spendableTxs can also be spent
	for _, txIn := range tx.Vin {
		utxo := utxoSet.FindUtxo(txIn.PrevOutPoint)
		if utxo != nil && !utxo.IsSpent {
			continue
		}
		if v, exists := spendableTxs.Load(txIn.PrevOutPoint.Hash); exists {
			spendableTxWrap := v.(*TxWrap)
			utxoSet.AddUtxo(spendableTxWrap.Tx, txIn.PrevOutPoint.Index, spendableTxWrap.Height)
		}
	}
	return utxoSet, nil
}

// ApplyTx updates utxos with the passed tx: adds all utxos in outputs and delete all utxos in inputs.
func (u *UtxoSet) ApplyTx(tx *types.Transaction, blockHeight uint32) error {
	// Add new utxos
	for txOutIdx := range tx.Vout {
		if err := u.AddUtxo(tx, (uint32)(txOutIdx), blockHeight); err != nil {
			if err == core.ErrAddExistingUtxo {
				// This can occur when a tx spends from another tx in front of it in the same block
				continue
			}
			return err
		}
	}

	// Coinbase transaction doesn't spend any utxo.
	if IsCoinBase(tx) {
		return nil
	}

	// Spend the referenced utxos
	for _, txIn := range tx.Vin {
		u.SpendUtxo(txIn.PrevOutPoint)
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
	logger.Debugf("UTXO: apply block with %d transactions", len(block.Txs))
	return nil
}

// RevertTx updates utxos with the passed tx: delete all utxos in outputs and add all utxos in inputs.
// It undoes the effect of ApplyTx on utxo set
func (u *UtxoSet) RevertTx(tx *types.Transaction, chain *BlockChain) error {
	txHash, _ := tx.TxHash()

	// Remove added utxos
	for txOutIdx := range tx.Vout {
		u.SpendUtxo(types.OutPoint{Hash: *txHash, Index: (uint32)(txOutIdx)})
	}

	// Coinbase transaction doesn't spend any utxo.
	if IsCoinBase(tx) {
		return nil
	}

	// "Unspend" the referenced utxos
	for _, txIn := range tx.Vin {
		utxoWrap := u.utxoMap[txIn.PrevOutPoint]
		if utxoWrap != nil {
			utxoWrap.IsSpent = false
			utxoWrap.IsModified = true
			continue
		}
		// This can happen when the block the tx is in is being reverted
		// The UTXO txIn spends have been deleted from UTXO set, so we load it from tx index
		block, txIdx, err := chain.LoadBlockInfoByTxHash(txIn.PrevOutPoint.Hash)
		if err != nil {
			logger.Panicf("Trying to unspend non-existing spent output %v", txIn.PrevOutPoint)
		}
		prevTx := block.Txs[txIdx]
		utxoWrap = &types.UtxoWrap{
			Output:      prevTx.Vout[txIn.PrevOutPoint.Index],
			BlockHeight: block.Height,
			IsCoinBase:  IsCoinBase(prevTx),
			IsModified:  true,
			IsSpent:     false,
		}
		u.utxoMap[txIn.PrevOutPoint] = utxoWrap
	}
	return nil
}

// RevertBlock undoes utxo changes made with all the transactions in the passed block
// It undoes the effect of ApplyBlock on utxo set
func (u *UtxoSet) RevertBlock(block *types.Block, chain *BlockChain) error {
	// Loop backwards through all transactions so everything is unspent in reverse order.
	// This is necessary since transactions later in a block can spend from previous ones.
	txs := block.Txs
	for txIdx := len(txs) - 1; txIdx >= 0; txIdx-- {
		tx := txs[txIdx]
		if err := u.RevertTx(tx, chain); err != nil {
			return err
		}
	}
	return nil
}

// ApplyBlockWithScriptFilter adds or remove all utxos that transactions use or generate
// with the specified script bytes
func (u *UtxoSet) ApplyBlockWithScriptFilter(block *types.Block, targetScript []byte) error {
	txs := block.Txs
	for _, tx := range txs {
		if err := u.ApplyTxWithScriptFilter(tx, block.Height, targetScript); err != nil {
			return err
		}
	}
	return nil
}

// ApplyTxWithScriptFilter adds or remove an utxo if the transaction uses or generates an utxo
// with the specified script bytes
func (u *UtxoSet) ApplyTxWithScriptFilter(tx *types.Transaction, blockHeight uint32, targetScript []byte) error {
	// Add new utxos
	for txOutIdx := range tx.Vout {
		if util.IsPrefixed(tx.Vout[txOutIdx].ScriptPubKey, targetScript) {
			if err := u.AddUtxo(tx, (uint32)(txOutIdx), blockHeight); err != nil {
				return err
			}
		}
	}

	// Coinbase transaction doesn't spend any utxo.
	if IsCoinBase(tx) {
		return nil
	}

	// Spend the referenced utxos
	for _, txIn := range tx.Vin {
		delete(u.utxoMap, txIn.PrevOutPoint)
	}
	return nil
}

// WriteUtxoSetToDB store utxo set to database.
func (u *UtxoSet) WriteUtxoSetToDB(batch storage.Batch) error {

	for outpoint, utxoWrap := range u.utxoMap {
		if utxoWrap == nil || !utxoWrap.IsModified {
			continue
		}
		utxoKey := UtxoKey(&outpoint)
		var addrUtxoKey []byte
		sc := *script.NewScriptFromBytes(utxoWrap.Output.ScriptPubKey)
		addr, err := sc.ExtractAddress()
		if err != nil {
			logger.Warnf("Failed to extract address. Err: %v", err)
			return err
		}
		if sc.IsTokenTransfer() || sc.IsTokenIssue() {
			var tokenID types.OutPoint
			if sc.IsTokenTransfer() {
				v, err := sc.GetTransferParams()
				if err != nil {
					logger.Warnf("Failed to get transfer param when write utxo to db. Err: %v", err)
				}
				tokenID = v.OutPoint
			} else {
				tokenID = outpoint
			}
			addrUtxoKey = AddrTokenUtxoKey(addr.String(), tokenID, outpoint)
		} else {
			addrUtxoKey = AddrUtxoKey(addr.String(), outpoint)
		}

		// Remove the utxo entry if it is spent.
		if utxoWrap.IsSpent {
			batch.Del(utxoKey)
			batch.Del(addrUtxoKey)
			continue
		} else if utxoWrap.IsModified {
			// Serialize and store the utxo entry.
			serialized, err := utxoWrap.Marshal()
			if err != nil {
				return err
			}
			batch.Put(utxoKey, serialized)
			batch.Put(addrUtxoKey, serialized)
		}
	}
	return nil
}

// LoadTxUtxos loads the unspent transaction outputs related to tx
func (u *UtxoSet) LoadTxUtxos(tx *types.Transaction, db storage.Table) error {

	if IsCoinBase(tx) {
		return nil
	}

	outPointsToFetch := make(map[types.OutPoint]struct{})
	for _, txIn := range tx.Vin {
		outPointsToFetch[txIn.PrevOutPoint] = struct{}{}
	}

	return u.fetchUtxosFromOutPointSet(outPointsToFetch, db)
}

// LoadBlockUtxos loads UTXOs txs in the block spend
func (u *UtxoSet) LoadBlockUtxos(block *types.Block, db storage.Table) error {

	txs := map[crypto.HashType]int{}
	outPointsToFetch := make(map[types.OutPoint]struct{})

	for index, tx := range block.Txs {
		hash, _ := tx.TxHash()
		txs[*hash] = index
	}
	for i, tx := range block.Txs[1:] {
		for _, txIn := range tx.Vin {
			preHash := &txIn.PrevOutPoint.Hash
			// i points to txs[i + 1], which should be after txs[index]
			// Thus (i + 1) > index, equavalently, i >= index
			if index, ok := txs[*preHash]; ok && i >= index {
				originTx := block.Txs[index]
				u.AddUtxo(originTx, txIn.PrevOutPoint.Index, block.Height)
				continue
			}
			if _, ok := u.utxoMap[txIn.PrevOutPoint]; ok {
				continue
			}
			outPointsToFetch[txIn.PrevOutPoint] = struct{}{}
		}
	}

	if len(outPointsToFetch) > 0 {
		if err := u.fetchUtxosFromOutPointSet(outPointsToFetch, db); err != nil {
			return err
		}
	}
	return nil

}

// LoadBlockAllUtxos loads all UTXOs txs in the block
func (u *UtxoSet) LoadBlockAllUtxos(block *types.Block, db storage.Table) error {

	txs := map[crypto.HashType]int{}
	outPointsToFetch := make(map[types.OutPoint]struct{})

	for index, tx := range block.Txs {
		hash, _ := tx.TxHash()
		txs[*hash] = index
	}
	for i, tx := range block.Txs[1:] {
		for _, txIn := range tx.Vin {
			preHash := &txIn.PrevOutPoint.Hash
			// i points to txs[i + 1], which should be after txs[index]
			// Thus (i + 1) > index, equavalently, i >= index
			if index, ok := txs[*preHash]; ok && i >= index {
				originTx := block.Txs[index]
				u.AddUtxo(originTx, txIn.PrevOutPoint.Index, block.Height)
				continue
			}
			if _, ok := u.utxoMap[txIn.PrevOutPoint]; ok {
				continue
			}
			outPointsToFetch[txIn.PrevOutPoint] = struct{}{}
		}

	}

	for _, tx := range block.Txs {
		hash, _ := tx.TxHash()
		outPoint := types.OutPoint{Hash: *hash}
		for idx := range tx.Vout {
			outPoint.Index = uint32(idx)
			outPointsToFetch[outPoint] = struct{}{}
		}
	}

	if len(outPointsToFetch) > 0 {
		if err := u.fetchUtxosFromOutPointSet(outPointsToFetch, db); err != nil {
			return err
		}
	}
	return nil

}

func (u *UtxoSet) fetchUtxosFromOutPointSet(outPoints map[types.OutPoint]struct{}, db storage.Table) error {
	for outPoint := range outPoints {
		entry, err := u.fetchUtxoWrapFromDB(db, outPoint)
		if err != nil {
			return err
		}
		if entry != nil {
			u.utxoMap[outPoint] = entry
		}
	}
	return nil
}

func (u *UtxoSet) fetchUtxoWrapFromDB(db storage.Table, outpoint types.OutPoint) (*types.UtxoWrap, error) {
	utxoKey := UtxoKey(&outpoint)
	serializedUtxoWrap, err := db.Get(utxoKey)
	if err != nil {
		return nil, err
	}
	if serializedUtxoWrap == nil {
		return nil, nil
	}
	utxoWrap := new(types.UtxoWrap)
	if err := utxoWrap.Unmarshal(serializedUtxoWrap); err != nil {
		return nil, err
	}
	return utxoWrap, nil
}
