package utxo

import (
	"bytes"
	"fmt"
	"strconv"
	"sync"

	"github.com/BOXFoundation/boxd/crypto"
	key2 "github.com/BOXFoundation/boxd/storage/key"

	"github.com/BOXFoundation/boxd/core/chain"
	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/script"
	"github.com/BOXFoundation/boxd/storage"
)

type WalletUtxo struct {
	f       filter
	utxoMap map[types.OutPoint]*types.UtxoWrap
	db      storage.Table
	mux     *sync.RWMutex
}

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

func NewWalletUtxoForP2PKH(db storage.Table) *WalletUtxo {
	return &WalletUtxo{
		f:       &p2pkhFilter{},
		utxoMap: make(map[types.OutPoint]*types.UtxoWrap),
		db:      db,
		mux:     &sync.RWMutex{},
	}
}

func (wu *WalletUtxo) ApplyUtxoSet(utxoSet *chain.UtxoSet) error {
	wu.mux.Lock()
	defer wu.mux.Unlock()
	for o, u := range utxoSet.All() {
		if u.IsSpent {
			wu.spendUtxo(o)
			continue
		}
		if wu.f.accept(u.Output.ScriptPubKey) {
			wu.utxoMap[o] = u
		}
	}
	return nil
}

func (wu *WalletUtxo) spendUtxo(outPoint types.OutPoint) {
	if u, ok := wu.utxoMap[outPoint]; ok {
		u.IsSpent = true
		u.IsModified = true
	} else {
		utxoWrap := &types.UtxoWrap{}
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

func (wu *WalletUtxo) Save() error {
	batch := wu.db.NewBatch()
	defer batch.Close()
	for o, u := range wu.utxoMap {
		if u == nil || !u.IsModified {
			continue
		}
		sc := *script.NewScriptFromBytes(u.Output.ScriptPubKey)
		addr, err := sc.ExtractAddress()
		if err != nil {
			return err
		}
		key := chain.AddrUtxoKey(addr.String(), o)
		if u.IsSpent {
			batch.Del(key)
		} else if u.IsModified {
			serialized, err := u.Marshal()
			if err != nil {
				return err
			}
			batch.Put(key, serialized)
		}
	}
	if err := batch.Write(); err != nil {
		return err
	}
	return nil
}

func (wu *WalletUtxo) FetchUtxoForAddress(addr types.Address) error {
	utxoKey := chain.AddrAllUtxoKey(addr.String())
	keys := wu.db.KeysWithPrefix(utxoKey)
	for _, keyBytes := range keys {
		serialized, err := wu.db.Get(keyBytes)
		if err != nil {
			return err
		}
		if serialized == nil {
			return fmt.Errorf("utxo not found for address: %s", addr.String())
		}
		utxoWrap := new(types.UtxoWrap)
		if err := utxoWrap.Unmarshal(serialized); err != nil {
			return err
		}
		k := key2.NewKeyFromBytes(keyBytes)
		segs := k.List()
		if len(segs) >= 4 {
			hash := new(crypto.HashType)
			if err := hash.SetString(segs[2]); err != nil {
				return err
			}
			index, err := strconv.ParseInt(segs[3], 16, 32)
			if err != nil {
				return err
			}
			op := types.OutPoint{
				Hash:  *hash,
				Index: uint32(index),
			}
			wu.utxoMap[op] = utxoWrap
		}
	}
	return nil
}

func (wu *WalletUtxo) Balance(addr types.Address) uint64 {
	wu.mux.RLock()
	defer wu.mux.RUnlock()
	sc := script.PayToPubKeyHashScript(addr.Hash())
	var balance uint64
	for _, u := range wu.utxoMap {
		if !u.IsSpent && bytes.HasPrefix(u.Output.ScriptPubKey, *sc.P2PKHScriptPrefix()) {
			balance += u.Output.Value
		}
	}
	return balance
}
