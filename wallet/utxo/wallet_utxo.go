// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package utxo

import (
	"encoding/binary"
	"fmt"
	"strconv"

	"github.com/BOXFoundation/boxd/core/chain"
	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/crypto"
	"github.com/BOXFoundation/boxd/log"
	"github.com/BOXFoundation/boxd/script"
	"github.com/BOXFoundation/boxd/storage"
	key2 "github.com/BOXFoundation/boxd/storage/key"
)

var logger = log.NewLogger("wallet-utxo")

func isTxUtxo(scriptBytes []byte) bool {
	sc := *script.NewScriptFromBytes(scriptBytes)
	if sc != nil && (sc.IsPayToPubKeyHash() || sc.IsTokenIssue() || sc.IsTokenTransfer()) {
		return true
	}
	return false
}

// ApplyUtxos apply utxos from chain
func ApplyUtxos(utxos types.UtxoMap, db storage.Table) error {
	if len(utxos) == 0 {
		return fmt.Errorf("no utxo to apply")
	}
	batch := db.NewBatch()
	addrsChanged := make(map[types.Address]struct{})
	for o, u := range utxos {
		if u == nil || u.Output == nil || u.Output.ScriptPubKey == nil {
			logger.Warnf("invalid utxo, outpoint: %s, utxoWrap: %+v", o, u)
			continue
		}
		if !isTxUtxo(u.Output.ScriptPubKey) {
			logger.Warnf("utxo[%s, %+v] is not tx utxo", o, u)
			continue
		}
		if !u.IsModified {
			logger.Warnf("utxo[%s, %+v] unmodified", o, u)
			continue
		}
		//
		sc := *script.NewScriptFromBytes(u.Output.ScriptPubKey)
		addr, err := sc.ExtractAddress()
		if err != nil {
			logger.Warnf("apply utxo[%s, %+v] error: %s", o, u, err)
			continue
		}
		addrsChanged[addr] = struct{}{}
		// store utxo with key consisting of addr and outpoint
		utxoKey := chain.AddrUtxoKey(addr.String(), o)
		if u.IsSpent {
			batch.Del(utxoKey)
		} else {
			serialized, err := u.Marshal()
			if err != nil {
				return err
			}
			batch.Put(utxoKey, serialized)
		}
	}
	// write storage
	if err := batch.Write(); err != nil {
		return err
	}

	// update balance
	for addr := range addrsChanged {
		updateBalanceFor(addr, db, batch)
	}
	// write storage
	return batch.Write()
}

func updateBalanceFor(addr types.Address, db storage.Table,
	batch storage.Batch) error {
	utxos, err := FetchUtxosOf(addr, db)
	if err != nil {
		return err
	}
	var balance uint64
	for _, u := range utxos {
		if u != nil && !u.IsSpent {
			balance += u.Output.Value
		}
	}

	return saveBalanceToDB(addr, balance, batch)
}

func fetchBalanceFromDB(addr types.Address, db storage.Table) (uint64, error) {
	bKey := chain.AddrBalanceKey(addr.String())
	buf, err := db.Get(bKey)
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

func saveBalanceToDB(addr types.Address, balance uint64, batch storage.Batch) error {
	buf := make([]byte, 8)
	binary.LittleEndian.PutUint64(buf, balance)
	key := chain.AddrBalanceKey(addr.String())
	batch.Put(key, buf)
	return nil
}

// BalanceFor returns balance amount of an address using balance index
func BalanceFor(addr types.Address, db storage.Table) uint64 {
	balance, err := fetchBalanceFromDB(addr, db)
	if err != nil {
		logger.Errorf("unable to get balance for addr: %s, err: %v", addr.String(), err)
		return 0
	}
	return balance
}

// FetchUtxosOf fetches utxos from db
func FetchUtxosOf(addr types.Address, db storage.Table) (map[types.OutPoint]*types.UtxoWrap, error) {
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
