// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package chain

import (
	"errors"
	"fmt"
	"sync"

	"github.com/BOXFoundation/boxd/core"
	"github.com/BOXFoundation/boxd/core/txlogic"
	"github.com/BOXFoundation/boxd/core/types"
	state "github.com/BOXFoundation/boxd/core/worldstate"
	"github.com/BOXFoundation/boxd/crypto"
	"github.com/BOXFoundation/boxd/script"
	"github.com/BOXFoundation/boxd/storage"
)

// BalanceChangeMap defines the balance changes of accounts (add or subtract)
type BalanceChangeMap map[types.AddressHash]uint64

// UtxoSet contains all utxos
type UtxoSet struct {
	utxoMap         types.UtxoMap
	normalTxUtxoSet map[types.OutPoint]struct{}
	contractUtxos   map[types.OutPoint]struct{}
}

// NewUtxoSet new utxo set
func NewUtxoSet() *UtxoSet {
	return &UtxoSet{
		utxoMap:         make(types.UtxoMap),
		normalTxUtxoSet: make(map[types.OutPoint]struct{}),
		contractUtxos:   make(map[types.OutPoint]struct{}),
	}
}

func (u *UtxoSet) String() string {
	s, sep := "{utxoMap: [", ""
	for k, v := range u.utxoMap {
		s += fmt.Sprintf("%s%s: %s", sep, k, v)
		sep = ", "
	}
	s += "], normal outpoint: ["
	sep = ""
	for k := range u.normalTxUtxoSet {
		s += sep + k.String()
		sep = ", "
	}
	s += "], contract outpoint: ["
	sep = ""
	for k := range u.contractUtxos {
		s += sep + k.String()
		sep = ", "
	}
	s += "]}"
	return s
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

// GetUtxo returns the utxo wrap relevant to OutPoint op
func (u *UtxoSet) GetUtxo(op *types.OutPoint) *types.UtxoWrap {
	w, ok := u.utxoMap[*op]
	if !ok {
		return nil
	}
	return w
}

// All returns all utxo contained including spent utxo
func (u *UtxoSet) All() types.UtxoMap {
	return u.utxoMap
}

// ContractUtxos returns contract utxos
func (u *UtxoSet) ContractUtxos() types.UtxoMap {
	utxoMaps := make(types.UtxoMap)
	for o := range u.contractUtxos {
		utxoMaps[o] = u.utxoMap[o]
	}
	return utxoMaps
}

// ImportUtxoMap imports utxos from a UtxoMap
func (u *UtxoSet) ImportUtxoMap(m types.UtxoMap) {
	for op, um := range m {
		u.utxoMap[op] = um
	}
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
	vout := tx.Vout[txOutIdx]
	sc := script.NewScriptFromBytes(vout.ScriptPubKey)
	if sc.IsOpReturnScript() {
		return fmt.Errorf("tx %s incluces a op_return vout", txHash)
	}
	if sc.IsContractPubkey() { // smart contract utxo
		return fmt.Errorf("tx %s incluces a contract vout", txHash)
	}

	// common tx
	var utxoWrap *types.UtxoWrap
	outPoint := types.OutPoint{Hash: *txHash, Index: txOutIdx}
	if utxoWrap = u.utxoMap[outPoint]; utxoWrap != nil {
		return core.ErrAddExistingUtxo
	}
	utxoWrap = types.NewUtxoWrap(vout.Value, vout.ScriptPubKey, blockHeight)
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
func GetExtendedTxUtxoSet(
	tx *types.Transaction, db storage.Reader, spendableTxs *sync.Map,
) (*UtxoSet, error) {

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
			if err := utxoSet.AddUtxo(spendableTxWrap.Tx, txIn.PrevOutPoint.Index,
				spendableTxWrap.Height); err != nil {
				logger.Error(err)
			}
		}
	}
	return utxoSet, nil
}

func (u *UtxoSet) applyUtxo(
	tx *types.Transaction, txOutIdx uint32, blockHeight uint32, statedb *state.StateDB,
) error {
	if txOutIdx >= uint32(len(tx.Vout)) {
		return core.ErrTxOutIndexOob
	}
	txHash, _ := tx.TxHash()
	vout := tx.Vout[txOutIdx]
	sc := script.NewScriptFromBytes(vout.ScriptPubKey)

	// common vout
	if !sc.IsContractPubkey() {
		address, err := sc.ExtractAddress()
		if err != nil {
			return fmt.Errorf("apply utxo with error: %s", err)
		}
		if !statedb.IsContractAddr(*address.Hash160()) {
			outPoint := types.NewOutPoint(txHash, txOutIdx)
			utxoWrap, exists := u.utxoMap[*outPoint]
			if exists {
				return core.ErrAddExistingUtxo
			}
			utxoWrap = types.NewUtxoWrap(vout.Value, vout.ScriptPubKey, blockHeight)
			u.utxoMap[*outPoint] = utxoWrap
			if txlogic.GetTxType(tx, statedb) != types.ContractTx {
				u.normalTxUtxoSet[*outPoint] = struct{}{}
			}
		} else {
			// for case to transfer box to contract address
			if vout.Value == 0 {
				return nil
			}
			addressHash := types.NormalizeAddressHash(address.Hash160())
			outPoint := types.NewOutPoint(addressHash, 0)
			var exists bool
			utxoWrap, exists := u.utxoMap[*outPoint]
			if !exists {
				utxoWrap = GetContractUtxoFromStateDB(statedb, address.Hash160())
				if utxoWrap == nil {
					return fmt.Errorf("%s for contract address: %x",
						core.ErrUtxoNotFound, address.Hash160()[:])
				}
			}
			value := utxoWrap.Value() + vout.Value
			logger.Infof("modify contract utxo in normal tx, outpoint: %+v, value: %d,"+
				" previous value: %d", outPoint, value, utxoWrap.Value())
			utxoWrap.SetValue(value)
			u.utxoMap[*outPoint] = utxoWrap
		}
		return nil
	}

	// smart contract vout
	from, contractAddr, nonce, err := sc.ParseContractInfo()
	if err != nil {
		return err
	}
	deploy := contractAddr == nil
	var outPoint *types.OutPoint
	if deploy {
		// deploy smart contract
		contractAddr, _ := types.MakeContractAddress(from, nonce)
		addressHash := types.NormalizeAddressHash(contractAddr.Hash160())
		outPoint = types.NewOutPoint(addressHash, 0)
		// value is set in ContractCreated function
		utxoWrap := types.NewUtxoWrap(0, vout.ScriptPubKey, blockHeight)
		u.utxoMap[*outPoint] = utxoWrap
	} else {
		// call smart contract
		addressHash := types.NormalizeAddressHash(contractAddr.Hash160())
		outPoint = types.NewOutPoint(addressHash, 0)
		var exists bool
		_, exists = u.utxoMap[*outPoint]
		if !exists {
			return fmt.Errorf("contract utxo[%v] not found in utxoset", outPoint)
		}
		logger.Infof("modify contract utxo, outpoint: %+v, value: %d, previous value: %d",
			outPoint, vout.Value, u.utxoMap[*outPoint].Value())
		u.utxoMap[*outPoint].AddValue(vout.Value)
	}
	u.contractUtxos[*outPoint] = struct{}{}

	return nil
}

// applyTx updates utxos with the passed tx: adds all utxos in outputs and delete all utxos in inputs.
func (u *UtxoSet) applyTx(tx *types.Transaction, blockHeight uint32, statedb *state.StateDB) error {
	// Add new utxos
	for txOutIdx, txOut := range tx.Vout {
		if sc := script.NewScriptFromBytes(txOut.ScriptPubKey); sc.IsOpReturnScript() {
			continue
		}
		if err := u.applyUtxo(tx, (uint32)(txOutIdx), blockHeight, statedb); err != nil {
			if err == core.ErrAddExistingUtxo {
				// This can occur when a tx spends from another tx in front of it in the same block
				continue
			}
			return err
		}
	}

	// Coinbase transaction doesn't spend any utxo.
	if IsCoinBase(tx) || IsInternalContract(tx) {
		return nil
	}

	// Spend the referenced utxos
	for _, txIn := range tx.Vin {
		u.SpendUtxo(txIn.PrevOutPoint)
		if txlogic.GetTxType(tx, statedb) != types.ContractTx {
			u.normalTxUtxoSet[txIn.PrevOutPoint] = struct{}{}
		}
	}
	return nil
}

func (u *UtxoSet) applyInternalTx(
	tx *types.Transaction, height uint32, statedb *state.StateDB,
) {
	txHash, _ := tx.TxHash()
	for idx, vout := range tx.Vout {
		sc := script.NewScriptFromBytes(vout.ScriptPubKey)
		if !sc.IsPayToPubKeyHash() {
			continue
		}
		outPoint := types.NewOutPoint(txHash, uint32(idx))
		u.utxoMap[*outPoint] = types.NewUtxoWrap(vout.Value, vout.ScriptPubKey, height)
	}
}

// ApplyInternalTxs applies internal txs in block
func (u *UtxoSet) ApplyInternalTxs(block *types.Block, statedb *state.StateDB) {
	for _, tx := range block.InternalTxs {
		u.applyInternalTx(tx, block.Header.Height, statedb)
	}
}

// ApplyBlock updates utxos with all transactions in the passed block
func (u *UtxoSet) ApplyBlock(block *types.Block, statedb *state.StateDB) error {
	txs := block.Txs
	for _, tx := range txs {
		if err := u.applyTx(tx, block.Header.Height, statedb); err != nil {
			return err
		}
	}
	logger.Infof("UTXO: apply block %s with %d transactions", block.BlockHash(), len(block.Txs))
	return nil
}

// RevertTx updates utxos with the passed tx: delete all utxos in outputs and add all utxos in inputs.
// It undoes the effect of ApplyTx on utxo set
func (u *UtxoSet) RevertTx(tx *types.Transaction, chain *BlockChain) error {
	txHash, _ := tx.TxHash()

	// Remove added utxos
	for i, o := range tx.Vout {
		sc := script.NewScriptFromBytes(o.ScriptPubKey)
		if sc.IsOpReturnScript() || sc.IsContractPubkey() {
			continue
		}
		u.SpendUtxo(*types.NewOutPoint(txHash, uint32(i)))
	}

	// Coinbase transaction doesn't spend any utxo.
	if IsCoinBase(tx) || IsInternalContract(tx) {
		return nil
	}

	// "Unspend" the referenced utxos
	for _, txIn := range tx.Vin {
		if txIn.PrevOutPoint.IsContractType() {
			continue
		}
		utxoWrap := u.utxoMap[txIn.PrevOutPoint]
		if utxoWrap != nil {
			utxoWrap.UnSpend()
			continue
		}
		// This can happen when the block the tx is in is being reverted
		// The UTXO txIn spends have been deleted from UTXO set, so we load it from tx index
		block, prevTx, _, err := chain.LoadBlockInfoByTxHash(txIn.PrevOutPoint.Hash)
		if err != nil {
			logger.Errorf("Failed to load block info by txhash %s: %+v. Err: %s",
				txHash, tx, err)
			logger.Panicf("Trying to unspend non-existing output %v", txIn.PrevOutPoint)
		}
		// prevTx := block.Txs[txIdx]
		prevOut := prevTx.Vout[txIn.PrevOutPoint.Index]
		if script.NewScriptFromBytes(prevOut.ScriptPubKey).IsOpReturnScript() {
			continue
		}
		utxoWrap = types.NewUtxoWrap(prevOut.Value, prevOut.ScriptPubKey, block.Header.Height)
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
	if len(block.InternalTxs) > 0 {
		txs = append(txs, block.InternalTxs...)
	}
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

	// isCoinBase := code&0x01 != 0
	blockHeight := uint32(code >> 1)

	// Decode the compressed unspent transaction output.
	value, pkScript, _, err := decodeCompressedTxOut(serialized[offset:])
	if err != nil {
		return nil, fmt.Errorf(fmt.Sprintf("unable to decode "+
			"utxo: %v", err))
	}

	utxoWrap := types.NewUtxoWrap(value, pkScript, blockHeight)

	return utxoWrap, nil
}

// WriteUtxoSetToDB store utxo set to database.
func (u *UtxoSet) WriteUtxoSetToDB(db storage.Writer) error {

	for outpoint, utxoWrap := range u.utxoMap {
		if utxoWrap == nil || !utxoWrap.IsModified() {
			continue
		}
		utxoKey := utxoKey(outpoint)
		var addrUtxoKey []byte

		if len(utxoWrap.Script()) == 0 { // for the utxo deleted from db later
			if outpoint.IsContractType() {
				addrHash := types.BytesToAddressHash(outpoint.Hash[:])
				addrUtxoKey = AddrUtxoKey(&addrHash, outpoint)
			}
		} else {
			sc := script.NewScriptFromBytes(utxoWrap.Script())
			if sc.IsOpReturnScript() {
				return fmt.Errorf("utxo set contains a op_return script vout %s", string(*sc))
			}
			addr, err := sc.ExtractAddress()
			if err != nil {
				logger.Warnf("Failed to extract address. utxoWrap: %+v, sc disasm: %s, hex: %x, Err: %s",
					utxoWrap, sc.Disasm(), utxoWrap.Script(), err)
				return err
			}
			if sc.IsTokenTransfer() || sc.IsTokenIssue() {
				var tokenID types.OutPoint
				if sc.IsTokenTransfer() {
					v, err := sc.GetTransferParams()
					if err != nil {
						logger.Warnf("Failed to get token transfer param when write utxo to db. Err: %v", err)
						return err
					}
					tokenID = v.OutPoint
				} else {
					tokenID = outpoint
				}
				addrUtxoKey = AddrTokenUtxoKey(addr.Hash160(), types.TokenID(tokenID), outpoint)
			} else if !sc.IsContractPubkey() {
				addrUtxoKey = AddrUtxoKey(addr.Hash160(), outpoint)
			} else {
				addrHash := types.BytesToAddressHash(outpoint.Hash[:])
				addrUtxoKey = AddrUtxoKey(&addrHash, outpoint)
			}
		}

		// Remove the utxo entry if it is spent.
		if utxoWrap.IsSpent() {
			db.Del(*utxoKey)
			if len(addrUtxoKey) > 0 {
				db.Del(addrUtxoKey)
			}
			logger.Debugf("delete utxo key: %x, utxo wrap: %+v", *utxoKey, utxoWrap)
			// recycleOutpointKey(utxoKey)
			continue
		} else if utxoWrap.IsModified() {
			// Serialize and store the utxo entry.
			serialized, err := SerializeUtxoWrap(utxoWrap)
			if err != nil {
				return err
			}
			logger.Debugf("put utxo to db: %x, utxo wrap: %d", *utxoKey, utxoWrap.Value())
			db.Put(*utxoKey, serialized)
			if len(addrUtxoKey) > 0 {
				db.Put(addrUtxoKey, serialized)
			}
		}
	}
	return nil
}

// LoadTxUtxos loads the unspent transaction outputs related to tx
func (u *UtxoSet) LoadTxUtxos(tx *types.Transaction, db storage.Reader) error {

	if IsCoinBase(tx) || IsInternalContract(tx) {
		return nil
	}

	outPointsToFetch := make(map[types.OutPoint]struct{})
	for _, txIn := range tx.Vin {
		outPointsToFetch[txIn.PrevOutPoint] = struct{}{}
	}

	return u.fetchUtxosFromOutPointSet(outPointsToFetch, db)
}

// LoadBlockUtxos loads UTXOs txs in the block spend
func (u *UtxoSet) LoadBlockUtxos(block *types.Block, needContract bool, db storage.Table, statedb *state.StateDB) error {
	outPointsToFetch := make(map[types.OutPoint]struct{})
	txs := map[crypto.HashType]int{}
	for index, tx := range block.Txs {
		hash, _ := tx.TxHash()
		txs[*hash] = index
	}
	for i, tx := range block.Txs {
		if i > 0 {
			for _, txIn := range tx.Vin {
				if !needContract && txIn.PrevOutPoint.IsContractType() {
					continue
				}
				preHash := &txIn.PrevOutPoint.Hash
				// i points to txs[i + 1], which should be after txs[index]
				// Thus (i + 1) > index, equavalently, i >= index
				if index, ok := txs[*preHash]; ok && i >= index {
					originTx := block.Txs[index]
					if err := u.AddUtxo(originTx, txIn.PrevOutPoint.Index,
						block.Header.Height); err != nil {
						logger.Error(err)
					}
					continue
				}
				if _, ok := u.utxoMap[txIn.PrevOutPoint]; ok {
					continue
				}
				outPointsToFetch[txIn.PrevOutPoint] = struct{}{}
			}
			if !needContract {
				continue
			}
		}

		// add utxo for contract vout script pubkey
		for _, txOut := range tx.Vout {
			sc := script.NewScriptFromBytes(txOut.ScriptPubKey)
			var addr *types.AddressHash
			if sc.IsContractPubkey() {
				contractAddr, err := sc.ParseContractAddr()
				if err != nil {
					logger.Warn(err)
					return err
				}
				addr = contractAddr.Hash160()
			} else if sc.IsPayToPubKeyHash() {
				addrPkh, _ := sc.ExtractAddress()
				if statedb.IsContractAddr(*addrPkh.Hash160()) {
					addr = addrPkh.Hash160()
				}
			}
			if addr != nil {
				hash := types.NormalizeAddressHash(addr)
				outPoint := types.NewOutPoint(hash, 0)
				outPointsToFetch[*outPoint] = struct{}{}
			}
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
func (u *UtxoSet) LoadBlockAllUtxos(block *types.Block, needContract bool, db storage.Table, statedb *state.StateDB) error {
	// for vin
	u.LoadBlockUtxos(block, needContract, db, statedb)

	// for vout
	outPointsToFetch := make(map[types.OutPoint]struct{})
	txs := block.Txs
	if len(block.InternalTxs) > 0 {
		txs = append(txs, block.InternalTxs...)
	}
	for _, tx := range txs {
		hash, _ := tx.TxHash()
		outPoint := types.OutPoint{Hash: *hash}
		for idx, out := range tx.Vout {
			sc := script.NewScriptFromBytes(out.ScriptPubKey)
			if sc.IsOpReturnScript() {
				continue
			}
			if !needContract && sc.IsContractPubkey() {
				continue
			}
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

func (u *UtxoSet) fetchUtxosFromOutPointSet(
	outPoints map[types.OutPoint]struct{}, db storage.Reader,
) error {
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

func (u *UtxoSet) calcNormalTxBalanceChanges(block *types.Block) (add, sub BalanceChangeMap) {
	add = make(BalanceChangeMap)
	sub = make(BalanceChangeMap)
	for _, tx := range block.Txs {
		if tx.Type == types.ContractTx {
			continue
		}
		for _, vout := range tx.Vout {
			sc := script.NewScriptFromBytes(vout.ScriptPubKey)
			// calc balance for account state, here only EOA (external owned account)
			// have balance state
			if !sc.IsPayToPubKeyHash() {
				continue
			}
			address, _ := sc.ExtractAddress()
			addr := address.Hash160()
			add[*addr] += vout.Value
		}
	}

	for o, w := range u.utxoMap {
		_, exists := u.normalTxUtxoSet[o]
		if !exists {
			continue
		}
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
		}
	}
	return
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

// MakeRollbackContractUtxos makes finally contract utxos from block
func MakeRollbackContractUtxos(
	block *types.Block, db storage.Table,
) (types.UtxoMap, error) {
	header := block.Header
	stateDB, _ := state.New(&header.RootHash, &header.UtxoRoot, db)
	// prevStateDB
	prevBlock, err := LoadBlockByHash(header.PrevBlockHash, db)
	if err != nil {
		return nil, err
	}
	prevHeader := prevBlock.Header
	prevStateDB, err := state.New(&prevHeader.RootHash, &prevHeader.UtxoRoot, db)
	if err != nil {
		return nil, err
	}
	// balances added and subtracted
	bAdd, bSub, err := calcContractAddrBalanceChanges(block, stateDB)
	if err != nil {
		logger.Errorf("calc contract addr balance changes error: %s", err)
		return nil, err
	}
	balances := make(map[types.AddressHash]uint64, len(bAdd)+len(bSub))
	um := make(types.UtxoMap)
	height := header.Height
	tempLimit := uint64(core.TransferGasLimit)
	for a, v := range bAdd {
		ah := types.NormalizeAddressHash(&a)
		op := types.NewOutPoint(ah, 0)
		if _, ok := balances[a]; !ok {
			balances[a] = stateDB.GetBalance(a).Uint64()
			sc, _ := script.MakeContractScriptPubkey(&types.ZeroAddressHash, &a,
				tempLimit, prevStateDB.GetNonce(a), types.VMVersion)
			um[*op] = types.NewUtxoWrap(balances[a], []byte(*sc), height)
		}
		utxo := um[*op]
		utxo.SetValue(utxo.Value() - v)
	}
	for a, v := range bSub {
		ah := types.NormalizeAddressHash(&a)
		op := types.NewOutPoint(ah, 0)
		if _, ok := balances[a]; !ok {
			balances[a] = stateDB.GetBalance(a).Uint64()
			sc, _ := script.MakeContractScriptPubkey(&types.ZeroAddressHash, &a,
				tempLimit, prevStateDB.GetNonce(a), types.VMVersion)
			um[*op] = types.NewUtxoWrap(balances[a], []byte(*sc), height)
		}
		utxo := um[*op]
		utxo.SetValue(utxo.Value() + v)
	}
	//
	// to update ContractAddr utxo via the value stored in previous state.
	// because gas used of txs that create or call contract
	// are directly added to ContractAddr, and to which there are not utxos corresponding
	//
	for op, u := range um {
		ah := new(types.AddressHash)
		ah.SetBytes(op.Hash[:])
		b := prevStateDB.GetBalance(*ah)
		checkBalance := u.Value()
		if b.Uint64() != checkBalance {
			addr, _ := types.NewContractAddressFromHash(ah[:])
			return nil, fmt.Errorf("block %s:%d contract addr %s rollback to incorrect "+
				"balance: %d, need: %d", block.BlockHash(), header.Height, addr, checkBalance, b)
		}
		utxoBytes, err := prevStateDB.GetUtxo(*ah)
		if err != nil {
			return nil, err
		}
		if utxoBytes == nil {
			u.Spend()
			continue
		}
		utxoWrap, err := DeserializeUtxoWrap(utxoBytes)
		if err != nil {
			return nil, err
		}
		um[op] = utxoWrap
	}
	return um, nil
}

func calcContractAddrBalanceChanges(
	block *types.Block, stateDB *state.StateDB,
) (add, sub BalanceChangeMap, err error) {

	add = make(BalanceChangeMap)
	sub = make(BalanceChangeMap)
	for _, v := range block.Txs {
		// calc contract transaction vout value
		for _, vout := range v.Vout {
			sc := script.NewScriptFromBytes(vout.ScriptPubKey)
			if !sc.IsContractPubkey() {
				// calc pay2pubkey with contract address vout value
				if !sc.IsPayToPubKeyHash() {
					continue
				}
				addr, _ := sc.ExtractAddress()
				if stateDB.GetCode(*addr.Hash160()) != nil {
					add[*addr.Hash160()] += vout.Value
				}
				continue
			}
			param, t, err := sc.ParseContractParams()
			if err != nil {
				logger.Error(err)
				return nil, nil, err
			}
			var addr *types.AddressHash
			if t == types.ContractCreationType {
				from, _ := types.NewAddressPubKeyHash(param.From[:])
				contractAddr, _ := types.MakeContractAddress(from, param.Nonce)
				addr = contractAddr.Hash160()
			} else {
				addr = param.To
			}
			add[*addr] += vout.Value
		}
	}

	for _, tx := range block.InternalTxs {
		if !tx.Vin[0].PrevOutPoint.IsContractType() {
			continue
		}
		inAddr := new(types.AddressHash)
		inAddr.SetBytes(tx.Vin[0].PrevOutPoint.Hash[:])
		spend := uint64(0)
		for _, out := range tx.Vout {
			spend += out.Value
			// TODO: if vout points to a contract address
		}
		sub[*inAddr] += spend
	}
	return
}
