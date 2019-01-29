// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package walletcmd

import (
	"sync"

	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/rpc/client"
	"github.com/BOXFoundation/boxd/wallet"
	"google.golang.org/grpc"
	"gopkg.in/abiosoft/ishell.v2"
)

type rpcBlockFetcher struct {
	conn *grpc.ClientConn
	mux  *sync.Mutex
}

// Height returns current block height of rpc target server
func (r *rpcBlockFetcher) Height() (uint32, error) {
	r.mux.Lock()
	defer r.mux.Unlock()
	return client.GetBlockCount(r.conn)
}

// HashForHeight returns hash string for the height param
func (r *rpcBlockFetcher) HashForHeight(height uint32) (string, error) {
	r.mux.Lock()
	defer r.mux.Unlock()
	return client.GetBlockHash(r.conn, height)
}

// Block returns block info of hash param
func (r *rpcBlockFetcher) Block(hashStr string) (*types.Block, error) {
	r.mux.Lock()
	defer r.mux.Unlock()
	return client.GetBlock(r.conn, hashStr)
}

type consoleBackend struct {
	s *ishell.Shell
	w *wallet.Manager
}

func startConsole(cli *grpc.ClientConn, walletMgr *wallet.Manager) {
	shell := ishell.New()
	shell.Println("Wallet Interactive Shell")
	c := &consoleBackend{
		s: shell,
		w: walletMgr,
	}
	c.init()
}

func (c *consoleBackend) init() {
	c.s.AddCmd(&ishell.Cmd{
		Name: "listAccounts",
		Func: c.ListAccounts,
	})
}

func (c *consoleBackend) ListAccounts(ctx *ishell.Context) {
	for _, addr := range c.w.ListAccounts() {
		ctx.Println("Addr: ", addr.Addr())
	}
}
