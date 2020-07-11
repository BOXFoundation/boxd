// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package wallet

import (
	"github.com/BOXFoundation/boxd/boxd/service"
	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/log"
	rpcpb "github.com/BOXFoundation/boxd/rpc/pb"
	"github.com/BOXFoundation/boxd/storage"
)

var logger = log.NewLogger("wallet")

var (
	utxoLiveCache = NewLiveUtxoCache()
)

type txPoolAPI interface {
	GetAllTxs() []*types.TxWrap
	FindTransaction(outpoint *types.OutPoint) *types.Transaction
}

type walletAgent struct {
	table  storage.Table
	txpool txPoolAPI
}

// Config contains config information for wallet server
type Config struct {
	Enable        bool `mapstructure:"enable"`
	UtxoCacheTime int  `mapstructure:"utxo_cache_time"`
}

// NewWalletAgent news a wallet agent instance
func NewWalletAgent(
	table storage.Table, txpool txPoolAPI, cacheTime int,
) service.WalletAgent {
	utxoLiveCache.SetLiveDuration(cacheTime)
	return &walletAgent{
		table:  table,
		txpool: txpool,
	}
}

// Balance returns the total balance of an address
func (w *walletAgent) Balance(addrHash *types.AddressHash, tokenID *types.TokenID) (uint64, error) {
	return BalanceFor(addrHash, tokenID, w.table, w.txpool)
}

// Utxos returns all utxos of an address
// NOTE: return all utxos of addr if amount is equal to 0
func (w *walletAgent) Utxos(
	addrHash *types.AddressHash, tokenID *types.TokenID, amount uint64,
) ([]*rpcpb.Utxo, error) {
	return FetchUtxosOf(addrHash, tokenID, amount, false, w.table, w.txpool)
}
