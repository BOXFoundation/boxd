// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package walletserver

import (
	"github.com/BOXFoundation/boxd/boxd/eventbus"
	"github.com/BOXFoundation/boxd/core/chain"
	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/log"
	"github.com/BOXFoundation/boxd/script"
	"github.com/BOXFoundation/boxd/storage"
	"github.com/BOXFoundation/boxd/wallet/utxo"
	"github.com/jbenet/goprocess"
)

var logger = log.NewLogger("wallet-server")

type WalletServer struct {
	proc  goprocess.Process
	bus   eventbus.Bus
	table storage.Table
	cfg   *Config
	wu    *utxo.WalletUtxo
}

func NewWalletServer(parent goprocess.Process, config *Config, s storage.Storage, bus eventbus.Bus) (*WalletServer, error) {
	proc := goprocess.WithParent(parent)
	table, err := s.Table(chain.WalletTableName)
	if err != nil {
		return nil, err
	}
	wServer := &WalletServer{
		proc:  proc,
		bus:   bus,
		table: table,
		cfg:   config,
		wu:    utxo.NewWalletUtxoForP2PKH(table),
	}
	return wServer, nil
}

func (w *WalletServer) Run() error {
	logger.Info("Wallet Server Start Running")
	if err := w.initListener(); err != nil {
		logger.Error("fail to subscribe utxo change")
	}
	return nil
}

func (w *WalletServer) Proc() goprocess.Process {
	return w.proc
}

func (w *WalletServer) Stop() {
	logger.Info("Wallet Server Stop Running")
	w.bus.Unsubscribe(eventbus.TopicUtxoUpdate, w.onUtxoChange)
	w.proc.Close()
}

func (w *WalletServer) initListener() error {
	return w.bus.Subscribe(eventbus.TopicUtxoUpdate, w.onUtxoChange)
}

func (w *WalletServer) onUtxoChange(utxoSet *chain.UtxoSet) {
	err := w.wu.ApplyUtxoSet(utxoSet)
	if err != nil {
		logger.Error("fail to apply utxo set", err)
	}
	if err := w.wu.Save(); err != nil {
		logger.Error("fail to save utxo set", err)
	}
}

func (w *WalletServer) Balance(addr types.Address) (uint64, error) {
	sc := script.PayToPubKeyHashScript(addr.Hash())
	s := utxo.NewWalletUtxoWithAddress(*sc.P2PKHScriptPrefix(), w.table)
	if err := s.FetchUtxoForAddress(addr); err != nil {
		return 0, err
	}
	return s.Balance(addr), nil
}
