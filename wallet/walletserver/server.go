// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package walletserver

import (
	"github.com/BOXFoundation/boxd/boxd/eventbus"
	"github.com/BOXFoundation/boxd/log"
	"github.com/BOXFoundation/boxd/storage"
	"github.com/jbenet/goprocess"
)

var logger = log.NewLogger("wallet-server")

type WalletServer struct {
	proc goprocess.Process
}

func NewWalletServer(parent goprocess.Process, config *Config, s storage.Storage, bus eventbus.Bus) (*WalletServer, error) {
	proc := goprocess.WithParent(parent)
	//ctx := goprocessctx.OnClosingContext(proc)
	wServer := &WalletServer{proc: proc}
	return wServer, nil
}

func (w *WalletServer) Run() error {
	logger.Info("Wallet Server Start Running")
	return nil
}

func (w *WalletServer) Proc() goprocess.Process {
	return w.proc
}

func (w *WalletServer) Stop() {
	logger.Info("Wallet Server Stop Running")
	w.proc.Close()
}
