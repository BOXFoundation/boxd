// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package wallet

import (
	"errors"
	"fmt"

	"github.com/BOXFoundation/boxd/boxd/eventbus"
	"github.com/BOXFoundation/boxd/core/chain"
	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/log"
	rpcpb "github.com/BOXFoundation/boxd/rpc/pb"
	"github.com/BOXFoundation/boxd/storage"
	"github.com/jbenet/goprocess"
)

var logger = log.NewLogger("wallet")

var (
	utxoLiveCache = NewLiveUtxoCache()
)

// Server is the struct type of an wallet service
type Server struct {
	proc  goprocess.Process
	bus   eventbus.Bus
	table storage.Table
	cfg   *Config
	//sync.Mutex
	//utxosQueue *list.List
}

// Config contains config information for wallet server
type Config struct {
	Enable        bool `mapstructure:"enable"`
	CacheSize     int  `mapstructure:"cache_size"`
	UtxoCacheTime int  `mapstructure:"utxo_cache_time"`
}

// NewServer creates an Server instance using config and storage
func NewServer(parent goprocess.Process, config *Config, s storage.Storage,
	bus eventbus.Bus) (*Server, error) {
	proc := goprocess.WithParent(parent)
	table, err := s.Table(chain.BlockTableName)
	if err != nil {
		return nil, err
	}
	wServer := &Server{
		proc:  proc,
		bus:   bus,
		table: table,
		cfg:   config,
		//utxosQueue: list.New(),
	}
	utxoLiveCache.SetLiveDuration(config.UtxoCacheTime)
	return wServer, nil
}

// Run starts Server main loop
func (w *Server) Run() error {
	logger.Info("Wallet Server Start Running")
	//if err := w.initListener(); err != nil {
	//	return fmt.Errorf("fail to subscribe utxo change")
	//}
	//w.proc.Go(w.loop)
	return nil
}

func (w *Server) loop(p goprocess.Process) {
	for {
		// check whether to exit
		select {
		case <-p.Closing():
			logger.Infof("Quit Wallet Server")
			return
		default:
		}
		// process
		//elem := w.utxosQueue.Front()
		//if elem == nil {
		//	continue
		//}
		//value := w.utxosQueue.Remove(elem)
		//utxoSet := value.(*chain.UtxoSet)

		//allUtxos := utxoSet.All()
	}
}

// Proc returns then go process of the wallet server
func (w *Server) Proc() goprocess.Process {
	return w.proc
}

// Stop terminate the Server process
func (w *Server) Stop() {
	logger.Info("Wallet Server Stop Running")
	w.bus.Unsubscribe(eventbus.TopicUtxoUpdate, w.onUtxoChange)
	w.proc.Close()
}

func (w *Server) initListener() error {
	return w.bus.SubscribeAsync(eventbus.TopicUtxoUpdate, w.onUtxoChange, true)
}

func (w *Server) onUtxoChange(utxoSet *chain.UtxoSet) {
	//w.Lock()
	//defer w.Unlock()
	//w.utxosQueue.PushBack(utxoSet)
}

// Balance returns the total balance of an address
func (w *Server) Balance(addr string, tokenID *types.TokenID) (uint64, error) {
	if w.cfg == nil || !w.cfg.Enable {
		err := errors.New("not supported for non-wallet node")
		logger.Warn(err)
		return 0, err
	}
	balance, err := BalanceFor(addr, tokenID, w.table)
	if err != nil {
		logger.Warnf("BalanceFor %s token id: %+v error: %s", addr, tokenID, err)
	}
	return balance, err
}

// Utxos returns all utxos of an address
// NOTE: return all utxos of addr if amount is equal to 0
func (w *Server) Utxos(addr string, tokenID *types.TokenID, amount uint64) (
	[]*rpcpb.Utxo, error) {

	if w.cfg == nil || !w.cfg.Enable {
		return nil, fmt.Errorf("fetch utxos not supported for non-wallet node")
	}
	utxos, err := FetchUtxosOf(addr, tokenID, amount, false, w.table)
	if err != nil {
		logger.Warnf("Utxos for %s token id: %+v error: %s", addr, tokenID, err)
		return nil, err
	}
	return utxos, nil
}
