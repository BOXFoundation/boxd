package utxo

import (
	"encoding/binary"
	"fmt"
	"strconv"
	"sync"

	"github.com/BOXFoundation/boxd/log"

	"github.com/BOXFoundation/boxd/crypto"
	key2 "github.com/BOXFoundation/boxd/storage/key"

	"github.com/BOXFoundation/boxd/core/chain"
	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/script"
	"github.com/BOXFoundation/boxd/storage"
)

var logger = log.NewLogger("wallet-utxo")

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

func (wu *WalletUtxo) Save() error {
	batch := wu.db.NewBatch()
	defer batch.Close()
	wu.mux.Lock()
	defer wu.mux.Unlock()
	balanceChange := make(map[types.Address]int64)
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
		oBalance, ok := balanceChange[addr]
		if !ok {
			oBalance = 0
		}
		if u.IsSpent {
			batch.Del(key)
			balanceChange[addr] = oBalance - int64(u.Output.Value)
		} else {
			serialized, err := u.Marshal()
			if err != nil {
				return err
			}
			batch.Put(key, serialized)
			balanceChange[addr] = oBalance + int64(u.Output.Value)
		}
		u.IsModified = false
	}
	for addr, c := range balanceChange {
		beforeChange, err := wu.fetchBalanceFromDB(addr)
		if err != nil {
			return err
		}
		balance := uint64(int64(beforeChange) + c)
		if err := wu.saveBalanceToDB(addr, balance, batch); err != nil {
			return err
		}
	}
	if err := batch.Write(); err != nil {
		return err
	}
	return nil
}

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
	} else {
		batch.Put(key, buf)
		return nil
	}
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
	//sc := script.PayToPubKeyHashScript(addr.Hash())
	//var balance uint64
	//for _, u := range wu.utxoMap {
	//	if !u.IsSpent && bytes.HasPrefix(u.Output.ScriptPubKey, *sc.P2PKHScriptPrefix()) {
	//		balance += u.Output.Value
	//	}
	//}
	if balance, err := wu.fetchBalanceFromDB(addr); err != nil {
		logger.Errorf("unable to get balance for addr: %s, err: %v", addr.String(), err)
		return 0
	} else {
		return balance
	}
}
