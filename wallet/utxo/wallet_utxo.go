// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package utxo

import (
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/BOXFoundation/boxd/core/chain"
	"github.com/BOXFoundation/boxd/core/txlogic"
	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/crypto"
	"github.com/BOXFoundation/boxd/log"
	"github.com/BOXFoundation/boxd/rpc/pb"
	"github.com/BOXFoundation/boxd/storage"
	sk "github.com/BOXFoundation/boxd/storage/key"
	"github.com/jbenet/goprocess"
)

const (
	utxoSelUnitCnt = 64
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
func BalanceFor(addr string, db storage.Table) (uint64, error) {
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
		balance += u.TxOut.Value
	}
	return balance, nil
}

// FetchUtxosOf fetches utxos from db
// if total is 0, fetch all utxos
func FetchUtxosOf(addr string, total uint64, db storage.Table) ([]*rpcpb.Utxo, error) {
	utxoKey := chain.AddrAllUtxoKey(addr)
	//
	start := time.Now()
	keys := db.KeysWithPrefix(utxoKey)
	logger.Infof("get utxos keys[%d] for %s cost %v", len(keys), addr, time.Since(start))
	//
	if total == 0 {
		utxos, err := makeUtxosFromDB(keys, db)
		if err != nil {
			return nil, err
		}
		return utxos, nil
	}
	//
	utxos, err := fetchModerateUtxos(keys, total, db)
	if err != nil {
		return nil, err
	}

	return utxos, nil
}

func fetchModerateUtxos(keys [][]byte, total uint64, db storage.Table) (
	[]*rpcpb.Utxo, error) {

	result := make([]*rpcpb.Utxo, 0)
	remain := total
	for start := 0; start < len(keys) && remain <= total; start += utxoSelUnitCnt {
		// calc start and end keys
		end := start + utxoSelUnitCnt
		if end > len(keys) {
			end = len(keys)
		}
		// fetch utxo from db
		utxos, err := makeUtxosFromDB(keys[start:end], db)
		if err != nil {
			return nil, err
		}
		// select utxos
		selUtxos, amount := selectUtxos(utxos, remain)
		remain -= amount
		result = append(result, selUtxos...)
	}

	return result, nil
}

func makeUtxosFromDB(keys [][]byte, db storage.Table) ([]*rpcpb.Utxo, error) {
	ts := time.Now()
	values, err := db.MultiGet(keys...)
	logger.Infof("get utxos values[count=%d] cost %v", len(keys), time.Since(ts))
	if err != nil {
		return nil, err
	}
	// make rpcpb.Utxo array
	utxos := make([]*rpcpb.Utxo, 0, len(values))
	for i, value := range values {
		if value == nil {
			logger.Warnf("utxo not found for key = %s", string(keys[i]))
			continue
		}
		utxoWrap := new(types.UtxoWrap)
		if err := utxoWrap.Unmarshal(value); err != nil {
			logger.Warnf("unmarshal error %s, key = %s, body = %v",
				err, string(keys[i]), string(value))
			continue
		}
		if utxoWrap.Output == nil {
			logger.Warnf("invalid utxo in db, key: %s, value: %+v", keys[i], utxoWrap)
			continue
		}
		op, err := parseOutPointFromDbKey(keys[i])
		if err != nil {
			logger.Warn(err)
			continue
		}
		utxos = append(utxos, makePbUtxo(op, utxoWrap))
	}
	return utxos, nil
}

func selectUtxos(utxos []*rpcpb.Utxo, amount uint64) ([]*rpcpb.Utxo, uint64) {
	total := uint64(0)
	for _, u := range utxos {
		total += u.TxOut.Value
	}
	if total <= amount {
		return utxos, total
	}
	// sort
	sort.Sort(sort.Interface(txlogic.SortByUTXOValue(utxos)))
	// select
	i, total := 0, uint64(0)
	for i < len(utxos) && total < amount {
		total += utxos[i].TxOut.Value
		i++
	}
	return utxos[:i], total
}

func parseOutPointFromDbKey(key []byte) (*types.OutPoint, error) {
	segs := sk.NewKeyFromBytes(key).List()
	if len(segs) < 4 {
		return nil, fmt.Errorf("invalid address utxo storage key %s", string(key))
	}
	hash := new(crypto.HashType)
	if err := hash.SetString(segs[2]); err != nil {
		return nil, err
	}
	index, err := strconv.ParseUint(segs[3], 16, 32)
	if err != nil {
		return nil, err
	}
	return txlogic.NewOutPoint(hash, uint32(index)), nil
}

func makePbUtxo(op *types.OutPoint, uw *types.UtxoWrap) *rpcpb.Utxo {
	return &rpcpb.Utxo{
		BlockHeight: uw.BlockHeight,
		IsCoinbase:  uw.IsCoinBase,
		IsSpent:     uw.IsSpent,
		OutPoint:    txlogic.NewPbOutPoint(&op.Hash, op.Index),
		TxOut:       uw.Output,
	}
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
