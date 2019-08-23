// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package wallet

import (
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"io/ioutil"
	"log"
	"math"
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
	rpcpb "github.com/BOXFoundation/boxd/rpc/pb"
	"github.com/BOXFoundation/boxd/script"
	"github.com/BOXFoundation/boxd/storage"
	"github.com/BOXFoundation/boxd/storage/rocksdb"
)

type testFlag string

const (
	boxTest   testFlag = "boxTest"
	tokenTest testFlag = "tokenTest"
)

func init() {
	log.SetFlags(log.Ldate | log.Lmicroseconds | log.Lshortfile)
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
	t.Run("t1", walletUtxosSaveGetTest(db, boxTest, 2, addr1))
	t.Run("t2", walletUtxosSaveGetTest(db, boxTest, 300, addr2))
	t.Run("t3", walletUtxosSaveGetTest(db, boxTest, 400, addrs1...))
}

func TestTokenSaveUtxos(t *testing.T) {
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
	t.Run("t1", walletUtxosSaveGetTest(db, tokenTest, 1, addr1))
	t.Run("t2", walletUtxosSaveGetTest(db, tokenTest, 300, addr2))
	t.Run("t3", walletUtxosSaveGetTest(db, tokenTest, 400, addrs1...))
}

func TestSelectUtxos(t *testing.T) {

	utxos := make([]*rpcpb.Utxo, 0)
	values := []uint64{1, 2, 3, 4, 5, 10, 9, 8, 7, 6, 6, 7, 8, 9, 10, 5, 4, 3, 2, 1}
	address, _ := types.NewAddress("b1ndoQmEd83y4Fza5PzbUQDYpT3mV772J5o")
	for _, v := range values {
		utxos = append(utxos, &rpcpb.Utxo{TxOut: (*corepb.TxOut)(txlogic.MakeVout(address.Hash160(), v))})
	}
	t.Run("t1", selUtxosTest(utxos, 10, []uint64{1, 1, 2, 2, 3, 3}))
	t.Run("t2", selUtxosTest(utxos, 0, []uint64{}))
	t.Run("t3", selUtxosTest(utxos, 1000, []uint64{1, 1, 2, 2, 3, 3, 4, 4, 5, 5,
		6, 6, 7, 7, 8, 8, 9, 9, 10, 10}))
	t.Run("t4", selUtxosTest(utxos, 110, []uint64{1, 1, 2, 2, 3, 3, 4, 4, 5, 5,
		6, 6, 7, 7, 8, 8, 9, 9, 10, 10}))
	t.Run("t5", selUtxosTest(utxos, 60, []uint64{1, 1, 2, 2, 3, 3, 4, 4, 5, 5,
		6, 6, 7, 7, 8}))

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

func newTestUtxoSet(
	n int, addrs ...string,
) (types.UtxoMap, map[types.AddressHash]uint64, map[types.AddressHash]types.UtxoMap) {

	utxoMap := make(types.UtxoMap, n)
	addrBalance := make(map[types.AddressHash]uint64)
	addrUtxos := make(map[types.AddressHash]types.UtxoMap)
	addrHashes := make([]*types.AddressHash, 0)
	for _, addr := range addrs {
		address, _ := types.ParseAddress(addr)
		addrHashes = append(addrHashes, address.Hash160())
		addrUtxos[*address.Hash160()] = make(types.UtxoMap)
	}
	for h := uint32(0); h < uint32(n); h++ {
		// outpoint
		hash := hashFromUint64(uint64(time.Now().UnixNano()))
		outpoint := types.NewOutPoint(&hash, h%10)
		// utxo wrap
		addr := addrHashes[int(h)%len(addrs)]
		value := 1 + uint64(rand.Intn(10000))
		utxoWrap := txlogic.NewUtxoWrap(addr, h, value)
		utxoMap[*outpoint] = utxoWrap
		// update balance
		addrBalance[*addr] += value
		// update utxo
		addrUtxos[*addr][*outpoint] = utxoWrap
	}
	// log
	log.Printf("newTestUtxoSet: balances: %+v, utxos count: %d\n",
		addrBalance, len(utxoMap))
	//
	return utxoMap, addrBalance, addrUtxos
}

func newTestTokenUtxoSet(
	n int, addrs ...string,
) (types.UtxoMap, map[types.AddressHash]uint64, map[types.AddressHash]types.UtxoMap, *types.TokenID) {

	// initial variables
	utxoMap := make(types.UtxoMap, n)
	addrBalance := make(map[types.AddressHash]uint64)
	addrUtxos := make(map[types.AddressHash]types.UtxoMap)
	addrHashes := make([]*types.AddressHash, 0)
	for _, addr := range addrs {
		address, _ := types.ParseAddress(addr)
		addrHashes = append(addrHashes, address.Hash160())
		addrUtxos[*address.Hash160()] = make(types.UtxoMap)
	}

	// generate issue utxo
	hash := hashFromUint64(uint64(time.Now().UnixNano()))
	tid := txlogic.NewTokenID(&hash, 0)
	addr, supply := addrHashes[0], uint64(1000000)
	tag := txlogic.NewTokenTag("box token", "BOX", 8, supply)
	issueUtxo, _ := txlogic.NewIssueTokenUtxoWrap(addr, tag, 0)
	outpoint := types.NewOutPoint(&hash, 0)

	utxoMap[*outpoint] = issueUtxo
	addrBalance[*addr] += supply * uint64(math.Pow10(int(tag.Decimal)))
	addrUtxos[*addr][*outpoint] = issueUtxo

	for h := uint32(0); h < uint32(n); h++ {
		// outpoint
		hash := hashFromUint64(uint64(time.Now().UnixNano()))
		outpoint := types.NewOutPoint(&hash, h%10)
		// utxo wrap
		addr := addrHashes[int(h)%len(addrs)]
		value := 1 + uint64(rand.Intn(10000))
		utxoWrap, _ := txlogic.NewTokenUtxoWrap(addr, tid, h, value)
		utxoMap[*outpoint] = utxoWrap
		// update balance
		addrBalance[*addr] += value
		// update utxo
		addrUtxos[*addr][*outpoint] = utxoWrap
	}
	// log
	log.Printf("newTestTokenUtxoSet: balances: %+v, utxos count: %d\n",
		addrBalance, len(utxoMap))
	//
	return utxoMap, addrBalance, addrUtxos, tid
}

func hashFromUint64(n uint64) crypto.HashType {
	bs := make([]byte, 8)
	binary.LittleEndian.PutUint64(bs, n)
	return crypto.DoubleHashH(bs)
}

func makeOutpoint(hash *crypto.HashType, i uint32) *types.OutPoint {
	return &types.OutPoint{Hash: *hash, Index: i}
}

func walletUtxosSaveGetTest(
	db storage.Table, flag testFlag, n int, addrs ...string,
) func(*testing.T) {
	return func(t *testing.T) {
		//t.Parallel()
		// gen utxos
		var (
			utxoMap  types.UtxoMap
			balances map[types.AddressHash]uint64
			utxos    map[types.AddressHash]types.UtxoMap
			tid      *types.TokenID
		)
		if flag == tokenTest {
			utxoMap, balances, utxos, tid = newTestTokenUtxoSet(n, addrs...)
		} else {
			utxoMap, balances, utxos = newTestUtxoSet(n, addrs...)
		}
		// apply utxos
		if err := applyUtxosTest(utxoMap, db); err != nil {
			t.Fatal(err)
		}
		addrHashes := make([]*types.AddressHash, 0, len(addrs))
		for _, addr := range addrs {
			address, _ := types.ParseAddress(addr)
			addrHashes = append(addrHashes, address.Hash160())
		}
		// check balance and utxos
		for _, addr := range addrHashes {
			// check balance
			balanceGot, err := BalanceFor(addr, tid, db)
			if err != nil {
				t.Error(err)
			}
			if balances[*addr] != balanceGot {
				t.Errorf("for balance of addr %s, want: %d, got: %d", addr, balances[*addr],
					balanceGot)
			}
			// check utxos
			utxosGot, err := FetchUtxosOf(addr, tid, 0, true, db)
			if err != nil {
				t.Error(err)
			}
			utxosWantC := comparableUtxoWrapMap(utxos[*addr])
			utxosGotC := comparableUtxoWrapMap(makeUtxoMapFromPbUtxos(utxosGot))
			if !reflect.DeepEqual(utxosWantC, utxosGotC) {
				t.Errorf("for utxos of addr %s, want map: %+v, got map: %+v", addr,
					utxosWantC, utxosGotC)
			}
			// check fetching partial utxos
			t.Logf("fetch utxos for %d", balanceGot/2)
			utxosGot, err = FetchUtxosOf(addr, tid, balanceGot/2, false, db)
			if err != nil {
				t.Error(err)
			}
			total := uint64(0)
			for _, u := range utxosGot {
				amount, tidR, err := txlogic.ParseUtxoAmount(u)
				if err != nil {
					t.Fatal(err)
				}
				if (tid != nil && (tidR == nil || *tid != *tidR)) ||
					(tid == nil && tidR != nil) {
					t.Fatalf("token id want %+v, got %+v", tid, tidR)
				}
				total += amount
			}
			if total < balanceGot/2 || total > balanceGot {
				t.Errorf("want value of utxos %d, got %d", balanceGot/2, total)
			}
		}
	}
}

func selUtxosTest(
	utxos []*rpcpb.Utxo, amount uint64, wantUtxos []uint64,
) func(*testing.T) {
	return func(t *testing.T) {
		wantAmount := uint64(0)
		for _, u := range wantUtxos {
			wantAmount += u
		}
		selUtxos, gotAmount := selectUtxos(utxos, nil, amount)
		gotUtxos := make([]uint64, 0)
		gotCalcAmount := uint64(0)
		for _, u := range selUtxos {
			gotUtxos = append(gotUtxos, u.TxOut.Value)
			gotCalcAmount += u.TxOut.Value
		}
		if !reflect.DeepEqual(wantUtxos, gotUtxos) {
			t.Errorf("for utxos, want: %v, got: %v", wantUtxos, gotUtxos)
		}
		if gotCalcAmount != gotAmount || wantAmount != gotAmount {
			t.Errorf("for amount, want: %d, got: %d, calc amount: %d",
				wantAmount, gotAmount, gotCalcAmount)
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
		Output: corepb.TxOut{
			Value:        uw.Value(),
			ScriptPubKey: uw.Script(),
		},
		BlockHeight: uw.Height(),
		// IsCoinBase:  uw.IsCoinBase(),
		IsSpent:    uw.IsSpent(),
		IsModified: uw.IsModified(),
	}
}

func (u ComparableUtxoWrap) String() string {
	return fmt.Sprintf("{Output:{Value: %d, ScriptPubKey: %s}, BlockHeight: %d, "+
		" IsSpent: %t, IsModified: %t}", u.Output.GetValue(),
		base64.StdEncoding.EncodeToString(u.Output.GetScriptPubKey()),
		u.BlockHeight, u.IsSpent,
		u.IsModified)
}

func makeUtxoMapFromPbUtxos(utxos []*rpcpb.Utxo) types.UtxoMap {
	m := make(types.UtxoMap)
	for _, u := range utxos {
		hash := crypto.HashType{}
		copy(hash[:], u.OutPoint.Hash[:])
		op := types.NewOutPoint(&hash, u.OutPoint.Index)
		m[*op] = types.NewUtxoWrap(u.TxOut.Value, u.TxOut.ScriptPubKey, u.BlockHeight)
		// if u.IsCoinbase {
		// 	m[*op].SetCoinBase()
		// }
	}
	return m
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
	// db.EnableBatch()
	batch := db.NewBatch()
	defer batch.Close()
	for o, u := range utxos {
		if u == nil {
			logger.Warnf("invalid utxo, outpoint: %s, utxoWrap: %+v", o, u)
			continue
		}
		if !isTxUtxo(u.Script()) {
			logger.Warnf("utxo[%s, %+v] is not tx utxo", o, u)
			continue
		}
		if !u.IsModified() {
			logger.Warnf("utxo[%s, %+v] unmodified", o, u)
			continue
		}
		//
		sc := *script.NewScriptFromBytes(u.Script())
		addr, err := sc.ExtractAddress()
		if err != nil {
			logger.Warnf("apply utxo[%s, %+v] error: %s", o, u, err)
			continue
		}
		// store utxo with key consisting of addr and outpoint
		var utxoKey []byte
		if sc.IsTokenTransfer() {
			param, err := sc.GetTransferParams()
			if err != nil {
				logger.Warnf("get token utxo ScriptPubKey %+v transfer params error: %s",
					u.Script(), err)
			}
			tid := types.TokenID(param.TokenID.OutPoint)
			utxoKey = chain.AddrTokenUtxoKey(addr.Hash160(), tid, o)
		} else if sc.IsTokenIssue() {
			utxoKey = chain.AddrTokenUtxoKey(addr.Hash160(), types.TokenID(o), o)
		} else if sc.IsPayToPubKeyHash() {
			utxoKey = chain.AddrUtxoKey(addr.Hash160(), o)
		} else {
			logger.Warnf("utxo[%s, %+v] is not tx utxo", o, u)
			continue
		}
		if u.IsSpent() {
			batch.Del(utxoKey)
		} else {
			serialized, err := chain.SerializeUtxoWrap(u)
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
