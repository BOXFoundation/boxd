// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package utxo

import (
	"strconv"
	"time"

	"github.com/BOXFoundation/boxd/core/chain"
	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/crypto"
	"github.com/BOXFoundation/boxd/log"
	"github.com/BOXFoundation/boxd/storage"
	key2 "github.com/BOXFoundation/boxd/storage/key"
	"github.com/jbenet/goprocess"
)

var logger = log.NewLogger("wallet-utxo")

// LiveCache defines a cache in that utxos keep alive
type LiveCache struct {
	liveDuration time.Duration
	ts2Op        map[int64]types.OutPoint
	Op2ts        map[types.OutPoint]int64
}

// NewLiveCache new a LiveCache instance with expired seconds
func NewLiveCache(expired int) *LiveCache {
	return &LiveCache{
		liveDuration: time.Duration(expired) * time.Second,
		ts2Op:        make(map[int64]types.OutPoint),
		Op2ts:        make(map[types.OutPoint]int64),
	}
}

// Server defines a utxo server
type Server struct {
	proc      goprocess.Process
	db        storage.Table
	liveCache *LiveCache
}

// NewServer new a Server instance
func NewServer(db storage.Table, expired int, parent goprocess.Process) *Server {
	return &Server{
		proc:      goprocess.WithParent(parent),
		db:        db,
		liveCache: NewLiveCache(expired),
	}
}

// Run starts utxo server
func (svr *Server) Run() error {
	logger.Info("Utxo Server Start Running")
	svr.proc.Go(svr.loop)
	return nil
}

func (svr *Server) loop(p goprocess.Process) {
	for {
		select {
		case <-p.Closing():
			logger.Infof("Quit Utxo Server")
			return
		default:
		}
	}
}

// BalanceFor returns balance amount of an address using balance index
func BalanceFor(addr types.Address, db storage.Table) (uint64, error) {
	utxos, err := FetchUtxosOf(addr, 0, db)
	if err != nil {
		return 0, err
	}
	var balance uint64
	for _, u := range utxos {
		if u == nil || u.IsSpent {
			logger.Warnf("fetch utxos for %s error, utxo: %+v", addr, u)
			continue
		}
		balance += u.Output.Value
	}
	return balance, nil
}

// FetchUtxosOf fetches utxos from db
func FetchUtxosOf(addr types.Address, amount uint64, db storage.Table) (types.UtxoMap, error) {
	utxoMap := make(types.UtxoMap)
	utxoKey := chain.AddrAllUtxoKey(addr.String())
	keys := db.KeysWithPrefix(utxoKey)
	values, err := db.MultiGet(keys...)
	if err != nil {
		return nil, err
	}

	for i, value := range values {
		if value == nil {
			logger.Warnf("utxo not found for %s key = %s", addr, string(keys[i]))
			continue
		}
		utxoWrap := new(types.UtxoWrap)
		if err := utxoWrap.Unmarshal(value); err != nil {
			logger.Warnf("unmarshal error for %s %s, key = %s, body = %v", addr, err,
				string(keys[i]), string(value))
			continue
		}
		k := key2.NewKeyFromBytes(keys[i])
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
