// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package chain

import (
	"encoding/hex"
	"errors"
	"fmt"
	"sync"

	"github.com/BOXFoundation/boxd/core"
	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/crypto"
	"github.com/BOXFoundation/boxd/script"
	"github.com/BOXFoundation/boxd/storage"
)

// UtxoSet contains all utxos
type UtxoSet struct {
	utxoMap types.UtxoMap
}

// BalanceChangeMap defines the balance changes of accounts (add or subtract)
type BalanceChangeMap map[types.AddressHash]uint64

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
		if !utxoWrap.IsSpent() {
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
func (u *UtxoSet) AddUtxo(tx *types.Transaction, txOutIdx uint32, blockHeight uint32, reader storage.Reader) error {
	// Index out of bound
	if txOutIdx >= uint32(len(tx.Vout)) {
		return core.ErrTxOutIndexOob
	}

	txHash, _ := tx.TxHash()
	sc := script.NewScriptFromBytes(tx.Vout[txOutIdx].ScriptPubKey)
	var utxoWrap *types.UtxoWrap
	if sc.IsContractPubkey() && tx.Vout[txOutIdx].Value > 0 { // smart contract utxo
		address, err := sc.ExtractAddress()
		if err != nil {
			return err
		}
		hash := types.NormalizeAddressHash(address.Hash160())
		if hash.IsEqual(&zeroHash) { // deploy smart contract
			sender, err := fetchOwnerOfOutPoint(&tx.Vin[0].PrevOutPoint, reader)
			if err != nil {
				return err
			}
			senderPubkeyHash, err := types.NewAddressPubKeyHash(sender.Hash())
			if err != nil {
				return err
			}
			contractAddress, err := types.MakeContractAddress(senderPubkeyHash, txHash, txOutIdx)
			if err != nil {
				return err
			}
			logger.Infof("contract address created by sender %s tx hash: %s vout idx: %d %s",
				address.Hash160(), txHash, txOutIdx, contractAddress)
			hash = types.NormalizeAddressHash(contractAddress.Hash160())
		}

		outPoint := types.OutPoint{Hash: *hash, Index: 0}
		if utxoWrap = u.utxoMap[outPoint]; utxoWrap == nil {
			utxoWrap, err = fetchUtxoWrapFromDB(reader, &outPoint)
			if err != nil {
				return err
			}
			if utxoWrap == nil {
				utxoWrap = types.NewUtxoWrap(tx.Vout[txOutIdx].Value, address.Hash(), 0)
				if IsCoinBase(tx) {
					return errors.New("Invalid smart contract tx")
				}
				utxoWrap.SetScript(tx.Vout[txOutIdx].ScriptPubKey)
				u.utxoMap[outPoint] = utxoWrap
				return nil
			}
		}
		value := utxoWrap.Value() + tx.Vout[txOutIdx].Value
		utxoWrap.SetValue(value)
		u.utxoMap[outPoint] = utxoWrap
		return nil
	}

	// common tx
	outPoint := types.OutPoint{Hash: *txHash, Index: txOutIdx}
	if utxoWrap = u.utxoMap[outPoint]; utxoWrap != nil {
		return core.ErrAddExistingUtxo
	}
	utxoWrap = types.NewUtxoWrap(tx.Vout[txOutIdx].Value, tx.Vout[txOutIdx].ScriptPubKey, blockHeight)
	if IsCoinBase(tx) {
		utxoWrap.SetCoinBase()
	}
	u.utxoMap[outPoint] = utxoWrap
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
	utxoWrap.Spend()
	// utxoWrap.IsSpent = true
	// utxoWrap.IsModified = true
}

// TxInputAmount returns total amount from tx's inputs
// Return 0 if a tx is not fully funded, i.e., if not all of its spending utxos exist
func (u *UtxoSet) TxInputAmount(tx *types.Transaction) uint64 {
	totalInputAmount := uint64(0)
	for _, txIn := range tx.Vin {
		utxo := u.FindUtxo(txIn.PrevOutPoint)
		if utxo == nil || utxo.IsSpent() {
			return uint64(0)
		}
		totalInputAmount += utxo.Value()
	}

	return totalInputAmount
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
		if utxo != nil && !utxo.IsSpent() {
			continue
		}
		if v, exists := spendableTxs.Load(txIn.PrevOutPoint.Hash); exists {
			spendableTxWrap := v.(*types.TxWrap)
			utxoSet.AddUtxo(spendableTxWrap.Tx, txIn.PrevOutPoint.Index, spendableTxWrap.Height, db)
		}
	}
	return utxoSet, nil
}

// applyTx updates utxos with the passed tx: adds all utxos in outputs and delete all utxos in inputs.
func (u *UtxoSet) applyTx(tx *types.Transaction, blockHeight uint32, db storage.Table) error {
	// Add new utxos
	for txOutIdx := range tx.Vout {
		if err := u.AddUtxo(tx, (uint32)(txOutIdx), blockHeight, db); err != nil {
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

func (u *UtxoSet) applyInternalTx(tx *types.Transaction, blockHeight uint32, db storage.Table) error {
	for txOutIdx := range tx.Vout {
		if err := u.AddUtxo(tx, (uint32)(txOutIdx), blockHeight, db); err != nil {
			if err == core.ErrAddExistingUtxo {
				continue
			}
			return err
		}
	}
	return nil
}

func (u *UtxoSet) applyInternalTxs(block *types.Block, db storage.Table) error {
	for _, tx := range block.InternalTxs {
		if err := u.applyTx(tx, block.Header.Height, db); err != nil {
			return err
		}
	}
	return nil
}

// ApplyBlock updates utxos with all transactions in the passed block
func (u *UtxoSet) ApplyBlock(block *types.Block, db storage.Table) error {
	txs := block.Txs
	for _, tx := range txs {
		if err := u.applyTx(tx, block.Header.Height, db); err != nil {
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
			// utxoWrap.IsSpent = false
			// utxoWrap.IsModified = true
			utxoWrap.UnSpend()
			continue
		}
		// This can happen when the block the tx is in is being reverted
		// The UTXO txIn spends have been deleted from UTXO set, so we load it from tx index
		block, prevTx, err := chain.LoadBlockInfoByTxHash(txIn.PrevOutPoint.Hash)
		if err != nil {
			logger.Error("Failed to load block info by txhash. Err: %v", err)
			logger.Panicf("Trying to unspend non-existing spent output %v", txIn.PrevOutPoint)
		}
		// prevTx := block.Txs[txIdx]
		utxoWrap = types.NewUtxoWrap(prevTx.Vout[txIn.PrevOutPoint.Index].Value,
			prevTx.Vout[txIn.PrevOutPoint.Index].ScriptPubKey, block.Header.Height)
		if IsCoinBase(prevTx) {
			utxoWrap.SetCoinBase()
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

var maxUint32VLQSerializeSize = serializeSizeVLQ(1<<32 - 1)

// outpointKeyPool defines a concurrent safe free list of byte slices used to
// provide temporary buffers for outpoint database keys.
var outpointKeyPool = sync.Pool{
	New: func() interface{} {
		b := make([]byte, crypto.HashSize+maxUint32VLQSerializeSize)
		return &b // Pointer to slice to avoid boxing alloc.
	},
}

func utxoKey(outpoint types.OutPoint) *[]byte {
	key := outpointKeyPool.Get().(*[]byte)
	idx := uint64(outpoint.Index)
	*key = (*key)[:crypto.HashSize+serializeSizeVLQ(idx)]
	copy(*key, outpoint.Hash[:])
	putVLQ((*key)[crypto.HashSize:], idx)
	return key
}

func recycleOutpointKey(key *[]byte) {
	outpointKeyPool.Put(key)
}

func utxoWrapHeaderCode(utxoWrap *types.UtxoWrap) (uint64, error) {
	if utxoWrap.IsSpent() {
		return 0, errors.New("attempt to serialize spent utxo header")
	}

	// As described in the serialization format comments, the header code
	// encodes the height shifted over one bit and the coinbase flag in the
	// lowest bit.
	headerCode := uint64(utxoWrap.Height()) << 1
	if utxoWrap.IsCoinBase() {
		headerCode |= 0x01
	}

	return headerCode, nil
}

// SerializeUtxoWrap returns the utxoWrap serialized to a format that is suitable
// for long-term storage.
func SerializeUtxoWrap(utxoWrap *types.UtxoWrap) ([]byte, error) {
	// Spent outputs have no serialization.
	if utxoWrap.IsSpent() {
		return nil, nil
	}

	// Encode the header code.
	headerCode, err := utxoWrapHeaderCode(utxoWrap)
	if err != nil {
		return nil, err
	}

	size := serializeSizeVLQ(headerCode) +
		compressedTxOutSize(utxoWrap.Value(), utxoWrap.Script())

	serialized := make([]byte, size)
	offset := putVLQ(serialized, headerCode)
	offset += compressedTxOut(serialized[offset:], uint64(utxoWrap.Value()),
		utxoWrap.Script())

	return serialized, nil
}

// DeserializeUtxoWrap returns UtxoWrap form serialized bytes.
func DeserializeUtxoWrap(serialized []byte) (*types.UtxoWrap, error) {

	// Deserialize the header code.
	code, offset := deserializeVLQ(serialized)
	if offset >= len(serialized) {
		return nil, errors.New("unexpected end of data after header")
	}

	isCoinBase := code&0x01 != 0
	blockHeight := uint32(code >> 1)

	// Decode the compressed unspent transaction output.
	value, pkScript, _, err := decodeCompressedTxOut(serialized[offset:])
	if err != nil {
		return nil, fmt.Errorf(fmt.Sprintf("unable to decode "+
			"utxo: %v", err))
	}

	utxoWrap := types.NewUtxoWrap(value, pkScript, blockHeight)
	if isCoinBase {
		utxoWrap.SetCoinBase()
	}

	return utxoWrap, nil
}

// WriteUtxoSetToDB store utxo set to database.
func (u *UtxoSet) WriteUtxoSetToDB(db storage.Table) error {

	for outpoint, utxoWrap := range u.utxoMap {
		if utxoWrap == nil || !utxoWrap.IsModified() {
			continue
		}
		utxoKey := utxoKey(outpoint)
		var addrUtxoKey []byte
		sc := *script.NewScriptFromBytes(utxoWrap.Script())
		addr, err := sc.ExtractAddress()
		if err != nil {
			logger.Warnf("Failed to extract address. utxoWrap: %+v, sc: %s %s, Err: %v",
				utxoWrap, sc.Disasm(), hex.EncodeToString(utxoWrap.Script()), err)
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
			addrUtxoKey = AddrTokenUtxoKey(addr.String(), types.TokenID(tokenID), outpoint)
		} else if sc.IsContractPubkey() {
			contractAddress := types.BytesToAddressHash(outpoint.Hash[:])
			//if len(addr.Hash()) > 0 && contractAddress.String() != addr.Hash160().String() {
			if len(addr.Hash()) > 0 {
				if contractAddress.String() != addr.Hash160().String() {
					return errors.New("Invalid contract address")
				}
			}
			addrUtxoKey = AddrUtxoKey(addr.String(), outpoint)
		} else {
			addrUtxoKey = AddrUtxoKey(addr.String(), outpoint)
		}

		// Remove the utxo entry if it is spent.
		if utxoWrap.IsSpent() {
			db.Del(*utxoKey)
			db.Del(addrUtxoKey)
			// recycleOutpointKey(utxoKey)
			continue
		} else if utxoWrap.IsModified() {
			// Serialize and store the utxo entry.
			serialized, err := SerializeUtxoWrap(utxoWrap)
			if err != nil {
				return err
			}
			db.Put(*utxoKey, serialized)
			db.Put(addrUtxoKey, serialized)
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
				u.AddUtxo(originTx, txIn.PrevOutPoint.Index, block.Header.Height, db)
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
				u.AddUtxo(originTx, txIn.PrevOutPoint.Index, block.Header.Height, db)
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
		entry, err := fetchUtxoWrapFromDB(db, &outPoint)
		if err != nil {
			return err
		}
		if entry != nil {
			u.utxoMap[outPoint] = entry
		}
	}
	return nil
}

func fetchUtxoWrapFromDB(reader storage.Reader, outpoint *types.OutPoint) (*types.UtxoWrap, error) {
	utxoKey := utxoKey(*outpoint)
	serializedUtxoWrap, err := reader.Get(*utxoKey)
	recycleOutpointKey(utxoKey)
	if err != nil {
		return nil, err
	}
	if serializedUtxoWrap == nil {
		return nil, nil
	}
	var utxoWrap *types.UtxoWrap
	if utxoWrap, err = DeserializeUtxoWrap(serializedUtxoWrap); err != nil {
		return nil, err
	}
	return utxoWrap, nil
}

func (u *UtxoSet) calcBalanceChanges() (add, sub BalanceChangeMap) {
	add = make(BalanceChangeMap)
	sub = make(BalanceChangeMap)
	for _, w := range u.utxoMap {
		sc := script.NewScriptFromBytes(w.Script())
		// calc balance for account state, here only EOA (external owned account)
		// have balance state
		if !sc.IsPayToPubKeyHash() {
			continue
		}
		address, _ := sc.ExtractAddress()
		addr := address.Hash160()
		if w.IsSpent() {
			sub[*addr] += w.Value()
			continue
		}
		add[*addr] += w.Value()
	}
	return
}
