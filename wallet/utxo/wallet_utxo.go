// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package utxo

import (
	"fmt"
	"strconv"

	"github.com/BOXFoundation/boxd/core/chain"
	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/crypto"
	"github.com/BOXFoundation/boxd/log"
	"github.com/BOXFoundation/boxd/storage"
	key2 "github.com/BOXFoundation/boxd/storage/key"
)

var logger = log.NewLogger("wallet-utxo")

// BalanceFor returns balance amount of an address using balance index
func BalanceFor(addr types.Address, db storage.Table) (uint64, error) {
	utxos, err := FetchUtxosOf(addr, db)
	if err != nil {
		return 0, err
	}
	var balance uint64
	for _, u := range utxos {
		if u != nil && !u.IsSpent {
			balance += u.Output.Value
		}
	}
	return balance, nil
}

// FetchUtxosOf fetches utxos from db
func FetchUtxosOf(addr types.Address, db storage.Table) (types.UtxoMap, error) {

	utxoKey := chain.AddrAllUtxoKey(addr.String())
	keys := db.KeysWithPrefix(utxoKey)
	utxoMap := make(map[types.OutPoint]*types.UtxoWrap)
	for _, keyBytes := range keys {
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
		k := key2.NewKeyFromBytes(keyBytes)
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

//func updateBalanceFor(addr types.Address, db storage.Table,
//	batch storage.Batch) error {
//	utxos, err := FetchUtxosOf(addr, db)
//	if err != nil {
//		return err
//	}
//	var balance uint64
//	for _, u := range utxos {
//		if u != nil && !u.IsSpent {
//			balance += u.Output.Value
//		}
//	}
//
//	return saveBalanceToDB(addr, balance, batch)
//}
//
//func fetchBalanceFromDB(addr types.Address, db storage.Table) (uint64, error) {
//	bKey := chain.AddrBalanceKey(addr.String())
//	buf, err := db.Get(bKey)
//	if err != nil {
//		return 0, err
//	}
//	if buf == nil {
//		return 0, nil
//	}
//	if len(buf) != 8 {
//		return 0, fmt.Errorf("invalid balance record")
//	}
//	return binary.LittleEndian.Uint64(buf), nil
//}
//
//func saveBalanceToDB(addr types.Address, balance uint64, batch storage.Batch) error {
//	buf := make([]byte, 8)
//	binary.LittleEndian.PutUint64(buf, balance)
//	key := chain.AddrBalanceKey(addr.String())
//	batch.Put(key, buf)
//	return nil
//}
