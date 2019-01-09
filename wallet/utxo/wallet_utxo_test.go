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

func newTestUtxoSet(n int, addrs ...string) (*chain.UtxoSet, map[string]uint64,
	map[string]txlogic.UtxoWrapMap) {

	utxoSetMap := make(map[types.OutPoint]*types.UtxoWrap, n)
	balanceMap := make(map[string]uint64)
	utxoMap := make(map[string]txlogic.UtxoWrapMap)
	for _, addr := range addrs {
		utxoMap[addr] = make(txlogic.UtxoWrapMap)
	}
	for h := uint32(0); h < uint32(n); h++ {
		// outpoint
		hash := hashFromUint64(uint64(time.Now().UnixNano()))
		outpoint := txlogic.NewOutPoint(&hash, h%10)
		// utxo wrap
		addr := addrs[int(h)%len(addrs)]
		value := 1 + uint64(rand.Intn(10000))
		utxoWrap := txlogic.NewUtxoWrap(addr, h, value)
		utxoSetMap[*outpoint] = utxoWrap
		// update balance
		balanceMap[addr] += value
		// update utxo
		utxoMap[addr][*outpoint] = utxoWrap
	}
	// log
	log.Printf("newTestUtxoSet: balances: %+v, utxos count: %d\n",
		balanceMap, len(utxoSetMap))
	//
	return chain.NewUtxoSetFromMap(utxoSetMap), balanceMap, utxoMap
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
	// new WalletUtxo
	wu := NewWalletUtxoForP2PKH(db)
	//
	addr1 := "b1ndoQmEd83y4Fza5PzbUQDYpT3mV772J5o"
	t.Run("test1", walletUtxosSaveGetTest(wu, 100, addr1))
	addr2 := "b1b8bzyci5VYUJVKRU2HRMMQiUXnoULkKAJ"
	t.Run("test2", walletUtxosSaveGetTest(wu, 300, addr2))
	addrs1 := []string{
		"b1jh8DSdB6kB7N7RanrudV1hzzMCCcoX6L7",
		"b1UP5pbfJgZrF1ezoSHLdvkxvgF2BYLtGva",
	}
	t.Run("test3", walletUtxosSaveGetTest(wu, 400, addrs1...))
}

func walletUtxosSaveGetTest(wu *WalletUtxo, n int, addrs ...string) func(*testing.T) {
	return func(t *testing.T) {
		// utxo set
		utxoSet, balances, utxos := newTestUtxoSet(n, addrs...)
		// new wallet utxo
		if err := wu.ApplyUtxoSet(utxoSet); err != nil {
			t.Fatal(err)
		}
		if err := wu.Save(); err != nil {
			t.Fatal(err)
		}
		// check balance and utxos
		for _, addr := range addrs {
			address, _ := types.NewAddress(addr)
			balanceGot := wu.Balance(address)
			if balances[addr] != balanceGot {
				t.Errorf("for balance of addr %s, want: %d, got: %d", addr, balances[addr],
					balanceGot)
			}
			// check utxos
			utxoGot, err := wu.Utxos(address)
			if err != nil {
				t.Error(err)
			}
			if !compareUtxoWrapMap(utxos[addr], utxoGot) {
				t.Errorf("for utxos of addr %s, want map len: %d, got map len: %d", addr,
					len(utxos[addr]), len(utxoGot))
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

func compareUtxoWrapMap(x, y txlogic.UtxoWrapMap) bool {
	x1 := make(map[types.OutPoint]ComparableUtxoWrap)
	y1 := make(map[types.OutPoint]ComparableUtxoWrap)
	for k, v := range x {
		x1[k] = *newComparableUtxoWrap(v)
	}
	for k, v := range y {
		y1[k] = *newComparableUtxoWrap(v)
	}
	return reflect.DeepEqual(x1, y1)
}
