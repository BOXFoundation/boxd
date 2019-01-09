// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package walletserver

import (
	"container/list"
	"fmt"
	"sync"
	"time"

	"github.com/BOXFoundation/boxd/boxd/eventbus"
	"github.com/BOXFoundation/boxd/core/chain"
	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/log"
	"github.com/BOXFoundation/boxd/storage"
	"github.com/BOXFoundation/boxd/wallet/utxo"
	"github.com/jbenet/goprocess"
)

var logger = log.NewLogger("wallet-server")

// WalletServer is the struct type of an wallet service
type WalletServer struct {
	proc  goprocess.Process
	bus   eventbus.Bus
	table storage.Table
	cfg   *Config
	wu    *utxo.WalletUtxo
	mux   *sync.Mutex
	queue *list.List
}

// NewWalletServer creates an WalletServer instance using config and storage
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
		mux:   &sync.Mutex{},
		queue: list.New(),
	}
	return wServer, nil
}

// Run starts WalletServer main loop
func (w *WalletServer) Run() error {
	logger.Info("Wallet Server Start Running")
	if err := w.initListener(); err != nil {
		logger.Error("fail to subscribe utxo change")
	}
	w.proc.Go(w.loop)
	return nil
}

func (w *WalletServer) loop(p goprocess.Process) {
	ticker := time.NewTicker(100 * time.Millisecond)
	for {
		select {
		case <-ticker.C:
			err := w.process()
			if err != nil {
				logger.Error(err)
			}
		case <-p.Closing():
			ticker.Stop()
			logger.Infof("Quit Wallet Server")
			return
		}
	}
}

func (w *WalletServer) process() error {
	w.mux.Lock()
	if w.queue.Len() == 0 {
		w.mux.Unlock()
		return nil
	}
	tmpList := list.New()
	tmpList.PushBackList(w.queue)
	w.queue.Init()
	w.mux.Unlock()
	elem := tmpList.Front()
	for elem != nil {
		err := w.wu.ApplyUtxoSet(elem.Value.(*chain.UtxoSet))
		if err != nil {
			logger.Error("fail to apply utxo set", err)
			return err
		}
		if err := w.wu.Save(); err != nil {
			logger.Error("fail to save utxo set", err)
			return err
		}
		w.wu.ClearSaved()
		elem = elem.Next()
	}
	return nil
}

// Proc returns then go process of the wallet server
func (w *WalletServer) Proc() goprocess.Process {
	return w.proc
}

// Stop terminate the WalletServer process
func (w *WalletServer) Stop() {
	logger.Info("Wallet Server Stop Running")
	w.bus.Unsubscribe(eventbus.TopicUtxoUpdate, w.onUtxoChange)
	w.proc.Close()
}

func (w *WalletServer) initListener() error {
	return w.bus.SubscribeAsync(eventbus.TopicUtxoUpdate, w.onUtxoChange, true)
}

func (w *WalletServer) onUtxoChange(utxoSet *chain.UtxoSet) {
	w.mux.Lock()
	defer w.mux.Unlock()
	w.queue.PushBack(utxoSet)
	//err := w.wu.ApplyUtxoSet(utxoSet)
	//if err != nil {
	//	logger.Error("fail to apply utxo set", err)
	//}
	//if err := w.wu.Save(); err != nil {
	//	logger.Error("fail to save utxo set", err)
	//}
	//w.wu.ClearSaved()
}

// Balance returns the total balance of an address
func (w *WalletServer) Balance(addr types.Address) (uint64, error) {
	//sc := script.PayToPubKeyHashScript(addr.Hash())
	//s := utxo.NewWalletUtxoWithAddress(*sc.P2PKHScriptPrefix(), w.table)
	//if err := s.FetchUtxoForAddress(addr); err != nil {
	//	return 0, err
	//}
	if w == nil || w.cfg == nil || !w.cfg.Enable {
		return 0, fmt.Errorf("not supported")
	}
	return w.wu.Balance(addr), nil
}

// Utxos returns all utxos of an address
func (w *WalletServer) Utxos(addr types.Address) (map[types.OutPoint]*types.UtxoWrap, error) {
	//sc := script.PayToPubKeyHashScript(addr.Hash())
	//s := utxo.NewWalletUtxoWithAddress(*sc.P2PKHScriptPrefix(), w.table)
	//if err := s.FetchUtxoForAddress(addr); err != nil {
	//
	//}
	if w == nil || w.cfg == nil || !w.cfg.Enable {
		return nil, fmt.Errorf("not supported")
	}
	return w.wu.Utxos(addr)
}
