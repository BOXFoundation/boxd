// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package walletcmd

import (
	"fmt"
	"os"
	"sync"

	"github.com/BOXFoundation/boxd/wallet"

	"github.com/BOXFoundation/boxd/rpc/client"

	"github.com/BOXFoundation/boxd/core/types"

	"github.com/BOXFoundation/boxd/wallet/walletdata"

	"google.golang.org/grpc"

	"github.com/jbenet/goprocess"
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

var _ walletdata.BlockFetcher = (*rpcBlockFetcher)(nil)

type consoleBackend struct {
	s *ishell.Shell
	w *wallet.Manager
	b walletdata.BlockCatcher
	a map[string]walletdata.AddrProcessor
}

func startConsole(cli *grpc.ClientConn, walletMgr *wallet.Manager) {
	shell := ishell.New()
	shell.Println("Wallet Interactive Shell")
	p := goprocess.WithSignals(os.Interrupt)
	fetcher := &rpcBlockFetcher{
		conn: cli,
		mux:  &sync.Mutex{},
	}
	bc := walletdata.NewBlockCatcher(p, fetcher)
	c := &consoleBackend{
		s: shell,
		w: walletMgr,
		b: bc,
		a: make(map[string]walletdata.AddrProcessor),
	}
	c.init()
	if err := bc.Run(); err != nil {
		panic(err)
	}
	shell.Run()
	select {
	case <-p.Closing():
		fmt.Print("Terminating")
		return
	}
	//shell.Run()
}

func (c *consoleBackend) init() {
	for _, acc := range c.w.ListAccounts() {
		processor := walletdata.NewMemAddrProcessor(acc.AddrType())
		c.b.RegisterListener(processor)
		c.a[acc.Addr()] = processor
	}
	c.s.AddCmd(&ishell.Cmd{
		Name: "listAccounts",
		Func: c.ListAccounts,
	})
	c.s.AddCmd(&ishell.Cmd{
		Name: "getbalance",
		Func: c.GetBalance,
	})
}

func (c *consoleBackend) ListAccounts(ctx *ishell.Context) {
	for _, addr := range c.w.ListAccounts() {
		ctx.Println("Addr: ", addr.Addr())
	}
}

func (c *consoleBackend) GetBalance(ctx *ishell.Context) {
	addrString := ctx.Args[0]
	processor, ok := c.a[addrString]
	if !ok {
		ctx.Println("Address not managed: ", addrString)
	} else {
		amount := processor.GetBalance()
		ctx.Printf("Balance of address: %v is %v\n", addrString, amount)
	}
}
