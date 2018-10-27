// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package chain

import (
	"sync"

	"github.com/BOXFoundation/boxd/core"
	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/crypto"
	"github.com/BOXFoundation/boxd/storage"
)

// UtxoSet contains all utxos
type UtxoSet struct {
	utxoMap map[types.OutPoint]*types.UtxoWrap
}

// NewUtxoSet new utxo set
func NewUtxoSet() *UtxoSet {
	return &UtxoSet{
		utxoMap: make(map[types.OutPoint]*types.UtxoWrap),
	}
}

// FindUtxo returns information about an outpoint.
func (u *UtxoSet) FindUtxo(outPoint types.OutPoint) *types.UtxoWrap {
	logger.Debugf("Find utxo: %+v", outPoint)
	return u.utxoMap[outPoint]
}

// AddUtxo adds a utxo
func (u *UtxoSet) AddUtxo(tx *types.Transaction, txOutIdx uint32, blockHeight uint32) error {
	logger.Debugf("Add utxo tx info: %+v, index: %d", tx, txOutIdx)
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
	logger.Debugf("Spend utxo: %+v", outPoint)
	utxoWrap := u.utxoMap[outPoint]
	if utxoWrap == nil {
		return
	}
	utxoWrap.IsSpent = true
}

// ApplyTx updates utxos with the passed tx: adds all utxos in outputs and delete all utxos in inputs.
func (u *UtxoSet) ApplyTx(tx *types.Transaction, blockHeight uint32) error {
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
func (u *UtxoSet) RevertTx(tx *types.Transaction, blockHeight uint32) error {
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
		if utxoWrap == nil {
			logger.Panicf("Trying to unspend non-existing spent output %v", txIn.PrevOutPoint)
		}
		utxoWrap.IsSpent = false
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

// WriteUtxoSetToDB store utxo set to database.
func (u *UtxoSet) WriteUtxoSetToDB(db storage.Table) error {
	for outpoint, utxoWrap := range u.utxoMap {
		if utxoWrap == nil || !utxoWrap.IsModified {
			continue
		}
		utxoKey := UtxoKey(&outpoint)
		// Remove the utxo entry if it is spent.
		if utxoWrap.IsSpent {
			// key := generateKey(outpoint)
			//err := db.Del(*key)
			// keyPool.Put(key)
			err := db.Del(utxoKey)
			if err != nil {
				return err
			}
			utxoWrap.IsModified = false
			continue
		}
		// Serialize and store the utxo entry.
		serialized, err := utxoWrap.Marshal()
		if err != nil {
			return err
		}
		// key := generateKey(outpoint)
		// err = db.Put(*key, serialized)
		err = db.Put(utxoKey, serialized)
		if err != nil {
			return err
		}
		utxoWrap.IsModified = false
	}
	return nil
}

// LoadTxUtxos loads the unspent transaction outputs related to tx
func (u *UtxoSet) LoadTxUtxos(tx *types.Transaction, db storage.Table) error {

	emptySet := make(map[types.OutPoint]struct{})

	prevOut := types.OutPoint{Hash: *tx.Hash}
	for idx := range tx.Vout {
		prevOut.Index = uint32(idx)
		emptySet[prevOut] = struct{}{}
	}
	if !IsCoinBase(tx) {
		for _, txIn := range tx.Vin {
			emptySet[txIn.PrevOutPoint] = struct{}{}
		}
	}

	if len(emptySet) > 0 {
		if err := u.fetchUtxosFromOutPointSet(emptySet, db); err != nil {
			return err
		}
	}
	return nil
}

// LoadBlockUtxos loads the unspent transaction outputs related to block
func (u *UtxoSet) LoadBlockUtxos(block *types.Block, db storage.Table) error {

	// utxoset := NewUtxoSet()
	txs := map[crypto.HashType]int{}
	emptySet := make(map[types.OutPoint]struct{})

	for index, tx := range block.Txs {
		txs[*tx.Hash] = index
	}

	for i, tx := range block.Txs[1:] {
		for _, txIn := range tx.Vin {
			preHash := &txIn.PrevOutPoint.Hash
			if index, ok := txs[*preHash]; ok && i >= index {
				originTx := block.Txs[index]
				for idx := range tx.Vout {
					u.AddUtxo(originTx, uint32(idx), block.Height)
				}
				continue
			}
			if _, ok := u.utxoMap[txIn.PrevOutPoint]; ok {
				continue
			}
			emptySet[txIn.PrevOutPoint] = struct{}{}
		}
	}

	if len(emptySet) > 0 {
		if err := u.fetchUtxosFromOutPointSet(emptySet, db); err != nil {
			return err
		}
	}
	return nil

}

func (u *UtxoSet) fetchUtxosFromOutPointSet(outPoints map[types.OutPoint]struct{}, db storage.Table) error {
	for outpoint := range outPoints {
		entry, err := u.fetchUtxoWrapFromDB(db, outpoint)
		if err != nil {
			return err
		}
		u.utxoMap[outpoint] = entry
	}
	return nil
}

func (u *UtxoSet) fetchUtxoWrapFromDB(db storage.Table, outpoint types.OutPoint) (*types.UtxoWrap, error) {
	// key := generateKey(outpoint)
	// serializedUtxoWrap, err := db.Get(*key)
	// keyPool.Put(key)
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

var maxUint32VLQSerializeSize = serializeSizeVLQ(1<<32 - 1)

var keyPool = sync.Pool{
	New: func() interface{} {
		b := make([]byte, crypto.HashSize+maxUint32VLQSerializeSize)
		return &b
	},
}

func serializeSizeVLQ(n uint64) int {
	size := 1
	for ; n > 0x7f; n = (n >> 7) - 1 {
		size++
	}
	return size
}

func putVLQ(target []byte, n uint64) int {
	offset := 0
	for ; ; offset++ {
		highBitMask := byte(0x80)
		if offset == 0 {
			highBitMask = 0x00
		}

		target[offset] = byte(n&0x7f) | highBitMask
		if n <= 0x7f {
			break
		}
		n = (n >> 7) - 1
	}
	// Reverse the bytes so it is MSB-encoded.
	for i, j := 0, offset; i < j; i, j = i+1, j-1 {
		target[i], target[j] = target[j], target[i]
	}

	return offset + 1
}

// func generateKey(outpoint *types.OutPoint) *[]byte {
// 	key := keyPool.Get().(*[]byte)
// 	idx := uint64(outpoint.Index)
// 	*key = (*key)[:crypto.HashSize+serializeSizeVLQ(idx)]
// 	copy(*key, outpoint.Hash[:])
// 	putVLQ((*key)[crypto.HashSize:], idx)
// 	return key
// }
