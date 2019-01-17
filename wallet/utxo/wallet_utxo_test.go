// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package utxo

import (
	"encoding/binary"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/BOXFoundation/boxd/core/chain"
	corepb "github.com/BOXFoundation/boxd/core/pb"
	"github.com/BOXFoundation/boxd/core/txlogic"
	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/crypto"
	"github.com/BOXFoundation/boxd/script"
	"github.com/BOXFoundation/boxd/storage"
	"github.com/BOXFoundation/boxd/storage/rocksdb"
)

var (
	testAddrs = []string{
		"b1ndoQmEd83y4Fza5PzbUQDYpT3mV772J5o",
		"b1b8bzyci5VYUJVKRU2HRMMQiUXnoULkKAJ",
		"b1jh8DSdB6kB7N7RanrudV1hzzMCCcoX6L7",
		"b1UP5pbfJgZrF1ezoSHLdvkxvgF2BYLtGva",
		"b1ZWSdrg48g145VdcmBwMPVuDFdaxDLoktk",
		"b1fRtRnKF4qhQG7bSwqbgR2BMw9VfM2XpT4",
	}
)

func init() {
	log.SetFlags(log.Ldate | log.Lmicroseconds | log.Lshortfile)
}

func getDatabase() (string, storage.Storage, error) {
	dbpath, err := ioutil.TempDir("", fmt.Sprintf("%d", rand.Int()))
	if err != nil {
		return "", nil, err
	}

	db, err := rocksdb.NewRocksDB(dbpath, &storage.Options{})
	if err != nil {
		return dbpath, nil, err
	}
	return dbpath, db, nil
}

func releaseDatabase(dbpath string, db storage.Storage) {
	db.Close()
	os.RemoveAll(dbpath)
}

func newTestUtxoSet(n int, addrs ...string) (types.UtxoMap, map[string]uint64,
	map[string]types.UtxoMap) {

	utxoMap := make(types.UtxoMap, n)
	addrBalance := make(map[string]uint64)
	addrUtxos := make(map[string]types.UtxoMap)
	for _, addr := range addrs {
		addrUtxos[addr] = make(types.UtxoMap)
	}
	for h := uint32(0); h < uint32(n); h++ {
		// outpoint
		hash := hashFromUint64(uint64(time.Now().UnixNano()))
		outpoint := txlogic.NewOutPoint(&hash, h%10)
		// utxo wrap
		addr := addrs[int(h)%len(addrs)]
		value := 1 + uint64(rand.Intn(10000))
		utxoWrap := txlogic.NewUtxoWrap(addr, h, value)
		utxoMap[*outpoint] = utxoWrap
		// update balance
		addrBalance[addr] += value
		// update utxo
		addrUtxos[addr][*outpoint] = utxoWrap
	}
	// log
	log.Printf("newTestUtxoSet: balances: %+v, utxos count: %d\n",
		addrBalance, len(utxoMap))
	//
	return utxoMap, addrBalance, addrUtxos
}

func hashFromUint64(n uint64) crypto.HashType {
	bs := make([]byte, 8)
	binary.LittleEndian.PutUint64(bs, n)
	return crypto.DoubleHashH(bs)
}

func makeOutpoint(hash *crypto.HashType, i uint32) *types.OutPoint {
	return &types.OutPoint{Hash: *hash, Index: i}
}

func TestSaveUtxos(t *testing.T) {
	// db
	dbpath, db, err := getDatabase()
	if err != nil {
		t.Fatal(err)
	}
	defer releaseDatabase(dbpath, db)
	//
	addr1 := "b1ndoQmEd83y4Fza5PzbUQDYpT3mV772J5o"
	addr2 := "b1b8bzyci5VYUJVKRU2HRMMQiUXnoULkKAJ"
	addrs1 := []string{
		"b1jh8DSdB6kB7N7RanrudV1hzzMCCcoX6L7",
		"b1UP5pbfJgZrF1ezoSHLdvkxvgF2BYLtGva",
	}
	t.Run("test1", walletUtxosSaveGetTest(db, 2, addr1))
	t.Run("test2", walletUtxosSaveGetTest(db, 300, addr2))
	t.Run("test3", walletUtxosSaveGetTest(db, 400, addrs1...))
}

func walletUtxosSaveGetTest(db storage.Table, n int, addrs ...string) func(*testing.T) {
	return func(t *testing.T) {
		//t.Parallel()
		// gen utxos
		utxoMap, balances, utxos := newTestUtxoSet(n, addrs...)
		// apply utxos
		if err := applyUtxosTest(utxoMap, db); err != nil {
			t.Fatal(err)
		}
		// check balance and utxos
		for _, addr := range addrs {
			address, _ := types.NewAddress(addr)
			// check balance
			balanceGot, err := BalanceFor(address, db)
			if err != nil {
				t.Error(err)
			}
			if balances[addr] != balanceGot {
				t.Errorf("for balance of addr %s, want: %d, got: %d", addr, balances[addr],
					balanceGot)
			}
			// check utxos
			utxosGot, err := FetchUtxosOf(address, db)
			if err != nil {
				t.Error(err)
			}
			utxosWantC := comparableUtxoWrapMap(utxos[addr])
			utxosGotC := comparableUtxoWrapMap(utxosGot)
			if !reflect.DeepEqual(utxosWantC, utxosGotC) {
				//t.Errorf("for utxos of addr %s, want map len: %d, got map len: %d",
				//	addr, len(utxos[addr]), len(utxosGot))
				t.Errorf("for utxos of addr %s, want map: %+v, got map: %+v", addr,
					utxosWantC, utxosGotC)
			}
		}
	}
}

type ComparableUtxoWrap struct {
	Output      corepb.TxOut
	BlockHeight uint32
	IsCoinBase  bool
	IsSpent     bool
	IsModified  bool
}

func newComparableUtxoWrap(uw *types.UtxoWrap) *ComparableUtxoWrap {
	return &ComparableUtxoWrap{
		Output:      *uw.Output,
		BlockHeight: uw.BlockHeight,
		IsCoinBase:  uw.IsCoinBase,
		IsSpent:     uw.IsSpent,
		IsModified:  uw.IsModified,
	}
}

func comparableUtxoWrapMap(um types.UtxoMap) map[types.OutPoint]ComparableUtxoWrap {
	x := make(map[types.OutPoint]ComparableUtxoWrap)
	for k, v := range um {
		u := *newComparableUtxoWrap(v)
		u.IsModified = false
		x[k] = u
	}
	return x
}

func applyUtxosTest(utxos types.UtxoMap, db storage.Table) error {
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
	return batch.Write()
}

func isTxUtxo(scriptBytes []byte) bool {
	sc := *script.NewScriptFromBytes(scriptBytes)
	if sc != nil && (sc.IsPayToPubKeyHash() || sc.IsTokenIssue() || sc.IsTokenTransfer()) {
		return true
	}
	return false
}
