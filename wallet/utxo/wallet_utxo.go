// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package utxo

import (
	"encoding/binary"
	"fmt"
	"strconv"
	"sync"

	"github.com/BOXFoundation/boxd/core/chain"
	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/crypto"
	"github.com/BOXFoundation/boxd/log"
	"github.com/BOXFoundation/boxd/script"
	"github.com/BOXFoundation/boxd/storage"
	key2 "github.com/BOXFoundation/boxd/storage/key"
)

var logger = log.NewLogger("wallet-utxo")

// WalletUtxo manages utxos of a set of addresses
type WalletUtxo struct {
	f       filter
	utxoMap map[types.OutPoint]*types.UtxoWrap
	db      storage.Table
	mux     *sync.RWMutex
}

// NewWalletUtxoWithAddress creates a WalletUtxo using one address filter
func NewWalletUtxoWithAddress(scriptBytes []byte, db storage.Table) *WalletUtxo {
	return &WalletUtxo{
		f: &addressFilter{
			scriptPrefix: scriptBytes,
		},
		utxoMap: make(map[types.OutPoint]*types.UtxoWrap),
		db:      db,
		mux:     &sync.RWMutex{},
	}
}

// NewWalletUtxoForP2PKH creates a WalletUtxo using p2pkh script filter
func NewWalletUtxoForP2PKH(db storage.Table) *WalletUtxo {
	return &WalletUtxo{
		f:       &p2pkhFilter{},
		utxoMap: make(map[types.OutPoint]*types.UtxoWrap),
		db:      db,
		mux:     &sync.RWMutex{},
	}
}

// ApplyUtxoSet merges utxo change from an chain.UtxoSet struct
func (wu *WalletUtxo) ApplyUtxoSet(utxoSet *chain.UtxoSet) error {
	wu.mux.Lock()
	defer wu.mux.Unlock()
	for o, u := range utxoSet.All() {
		// logger.Infof("apply utxo :%+v", o)
		if u.IsSpent {
			wu.spendUtxo(o, u)
			continue
		}
		if wu.f.accept(u.Output.ScriptPubKey) {
			wu.utxoMap[o] = u
		}
	}
	return nil
}

func (wu *WalletUtxo) spendUtxo(outPoint types.OutPoint, wrap *types.UtxoWrap) {
	if u, ok := wu.utxoMap[outPoint]; ok {
		u.IsSpent = true
		u.IsModified = true
	} else {
		utxoWrap := &types.UtxoWrap{
			Output:      wrap.Output,
			BlockHeight: wrap.BlockHeight,
			IsCoinBase:  wrap.IsCoinBase,
		}
		utxoWrap.IsSpent = true
		utxoWrap.IsModified = true
		wu.utxoMap[outPoint] = utxoWrap
	}
}

func (wu *WalletUtxo) addUtxo(outPoint types.OutPoint, wrap *types.UtxoWrap) {
	wrapCopy := &types.UtxoWrap{
		Output:      wrap.Output,
		BlockHeight: wrap.BlockHeight,
		IsCoinBase:  wrap.IsCoinBase,
		IsSpent:     false,
		IsModified:  true,
	}
	wu.utxoMap[outPoint] = wrapCopy
}

// Save stores current utxo and balance change to db
func (wu *WalletUtxo) Save() error {
	batch := wu.db.NewBatch()
	defer batch.Close()
	wu.mux.Lock()
	defer wu.mux.Unlock()
	recalculateAddrs := make(map[string]bool)
	increAddrs := make(map[string]int64)
	for o, u := range wu.utxoMap {
		if u == nil || !u.IsModified {
			continue
		}
		if u.Output == nil || u.Output.ScriptPubKey == nil {
			logger.Errorf("invalid output %+v wrapper %+v", o, u)
			continue
		}
		sc := *script.NewScriptFromBytes(u.Output.ScriptPubKey)
		addr, err := sc.ExtractAddress()
		if err != nil {
			return err
		}
		recalculateAddrs[addr.String()] = true
		key := chain.AddrUtxoKey(addr.String(), o)
		oBalance, ok := increAddrs[addr.String()]
		if !ok {
			oBalance = 0
		}
		_, reCalculate := recalculateAddrs[addr.String()]
		if u.IsSpent {
			batch.Del(key)
			// logger.Infof("Addr :%s spent %d", addr, u.Output.Value)
			if !reCalculate {
				exist, err := wu.db.Has(key)
				if err != nil {
					recalculateAddrs[addr.String()] = true
				} else if exist {
					increAddrs[addr.String()] = oBalance - int64(u.Output.Value)
				}
			}
		} else {
			serialized, err := u.Marshal()
			if err != nil {
				return err
			}
			batch.Put(key, serialized)
			// logger.Infof("Addr: %s received %d", addr, u.Output.Value)
			if !reCalculate {
				increAddrs[addr.String()] = oBalance + int64(u.Output.Value)
			}
		}
		u.IsModified = false
	}
	if err := batch.Write(); err != nil {
		return err
	}
	balanceBatch := wu.db.NewBatch()
	defer balanceBatch.Close()
	for addrStr := range recalculateAddrs {
		addr := &types.AddressPubKeyHash{}
		addr.SetString(addrStr)
		if err := wu.updateBalanceFromUtxo(addr, balanceBatch); err != nil {
			return err
		}
	}
	for addrStr, balanceChange := range increAddrs {
		if _, ok := recalculateAddrs[addrStr]; ok {
			//ignore already existing addresses
			continue
		}
		addr := &types.AddressPubKeyHash{}
		addr.SetString(addrStr)
		oBalance := wu.Balance(addr)
		if err := wu.saveBalanceToDB(addr, uint64(int64(oBalance)+balanceChange), balanceBatch); err != nil {
			return err
		}
	}

	return balanceBatch.Write()
}

func (wu *WalletUtxo) updateBalanceFromUtxo(addr types.Address, batch storage.Batch) error {
	utxos, err := fetchUtxoFromDB(addr, wu.db)
	if err != nil {
		return err
	}
	var balance uint64
	for _, u := range utxos {
		if u != nil && !u.IsSpent {
			balance += u.Output.Value
		}
	}

	return wu.saveBalanceToDB(addr, balance, batch)
}

// ClearSaved removes all utxos
func (wu *WalletUtxo) ClearSaved() {
	wu.mux.Lock()
	defer wu.mux.Unlock()
	wu.utxoMap = make(map[types.OutPoint]*types.UtxoWrap)
	//clearKeys := make([]types.OutPoint, 0, len(wu.utxoMap))
	//for o, u := range wu.utxoMap {
	//	if !u.IsModified {
	//		clearKeys = append(clearKeys, o)
	//	}
	//}
	//for _, o := range clearKeys {
	//	delete(wu.utxoMap, o)
	//}
}

func (wu *WalletUtxo) fetchBalanceFromDB(addr types.Address) (uint64, error) {
	bKey := chain.AddrBalanceKey(addr.String())
	buf, err := wu.db.Get(bKey)
	if err != nil {
		return 0, err
	}
	if buf == nil {
		return 0, nil
	}
	if len(buf) != 8 {
		return 0, fmt.Errorf("invalid balance record")
	}
	return binary.LittleEndian.Uint64(buf), nil
}

func (wu *WalletUtxo) saveBalanceToDB(addr types.Address, balance uint64, batch storage.Batch) error {
	buf := make([]byte, 8)
	binary.LittleEndian.PutUint64(buf, balance)
	key := chain.AddrBalanceKey(addr.String())
	if batch == nil {
		return wu.db.Put(key, buf)
	}
	batch.Put(key, buf)
	return nil
}

// FetchUtxoForAddress fetches all utxo from db into struct internal member utxoMap
func (wu *WalletUtxo) FetchUtxoForAddress(addr types.Address) error {
	utxoMap, err := fetchUtxoFromDB(addr, wu.db)
	if err != nil {
		return err
	}
	for k, v := range utxoMap {
		wu.utxoMap[k] = v
	}
	return nil
}

func fetchUtxoFromDB(addr types.Address, db storage.Table) (map[types.OutPoint]*types.UtxoWrap, error) {
	utxoKey := chain.AddrAllUtxoKey(addr.String())
	// logger.Errorf("utxokey = %v", string(utxoKey))
	keys := db.KeysWithPrefix(utxoKey)
	// logger.Errorf("keys = %v", len(keys))
	utxoMap := make(map[types.OutPoint]*types.UtxoWrap)
	for _, keyBytes := range keys {
		// logger.Errorf("keyBytes = %v", string(keyBytes))
		serialized, err := db.Get(keyBytes)
		if err != nil {
			return nil, err
		}
		if serialized == nil {
			return nil, fmt.Errorf("utxo not found for address: %s", addr.String())
		}
		utxoWrap := new(types.UtxoWrap)
		if err := utxoWrap.Unmarshal(serialized); err != nil {
			return nil, err
		}
		// logger.Errorf("utxoWrap = %v", utxoWrap)
		k := key2.NewKeyFromBytes(keyBytes)
		// logger.Errorf("k = %v", k)
		segs := k.List()
		if len(segs) >= 4 {
			hash := new(crypto.HashType)
			if err := hash.SetString(segs[2]); err != nil {
				return nil, err
			}
			index, err := strconv.ParseInt(segs[3], 16, 32)
			if err != nil {
				return nil, err
			}
			op := types.OutPoint{
				Hash:  *hash,
				Index: uint32(index),
			}
			utxoMap[op] = utxoWrap
		}
	}
	return utxoMap, nil
}

// Balance returns balance amount of an address using balance index
func (wu *WalletUtxo) Balance(addr types.Address) uint64 {
	//sc := script.PayToPubKeyHashScript(addr.Hash())
	//var balance uint64
	//for _, u := range wu.utxoMap {
	//	if !u.IsSpent && bytes.HasPrefix(u.Output.ScriptPubKey, *sc.P2PKHScriptPrefix()) {
	//		balance += u.Output.Value
	//	}
	//}
	balance, err := wu.fetchBalanceFromDB(addr)
	if err != nil {
		logger.Errorf("unable to get balance for addr: %s, err: %v", addr.String(), err)
		return 0
	}
	return balance
}

// Utxos returns all utxo of an address from db
func (wu *WalletUtxo) Utxos(addr types.Address) (map[types.OutPoint]*types.UtxoWrap, error) {
	return fetchUtxoFromDB(addr, wu.db)
}
