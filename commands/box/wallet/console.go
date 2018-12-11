// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package walletcmd

import (
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/spf13/viper"

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
	c.s.AddCmd(&ishell.Cmd{
		Name: "send",
		Func: c.SendFrom,
	})
	c.s.AddCmd(&ishell.Cmd{
		Name: "repeatSend",
		Func: c.repeatSend,
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
func parseSendTarget(args []string) (map[types.Address]uint64, error) {
	targets := make(map[types.Address]uint64)
	for i := 0; i < len(args)/2; i++ {
		addr, err := types.NewAddress(args[i*2])
		if err != nil {
			return targets, err
		}
		amount, err := strconv.Atoi(args[i*2+1])
		if err != nil {
			return targets, err
		}
		targets[addr] = uint64(amount)
	}
	return targets, nil
}

func (c *consoleBackend) SendFrom(ctx *ishell.Context) {
	if len(ctx.Args) < 3 {
		ctx.Err(fmt.Errorf("invalid argument number"))
		//ctx.Println("Invalid argument number")
		return
	}
	target, err := parseSendTarget(ctx.Args[1:])
	if err != nil {
		ctx.Err(err)
		//fmt.Println(err)
		return
	}
	account, exists := c.w.GetAccount(ctx.Args[0])
	if !exists {
		ctx.Err(fmt.Errorf("account %s not managed", ctx.Args[0]))
		return
	}
	ctx.Println("Please Input Your Passphrase")
	passphrase := ctx.ReadPassword()
	if err := account.UnlockWithPassphrase(passphrase); err != nil {
		ctx.Err(err)
		//fmt.Println("Fail to unlock account", err)
		return
	}
	publickey := account.PublicKey()
	tx, err := c.a[ctx.Args[0]].CreateSendTransaction(publickey, target, account)
	if err != nil {
		ctx.Err(err)
	}
	conn := client.NewConnectionWithViper(viper.GetViper())
	defer conn.Close()
	if err := client.SendTransaction(conn, tx); err != nil {
		ctx.Err(err)
	}
	ctx.Println("transaction sended")
}

func (c *consoleBackend) repeatSend(ctx *ishell.Context) {
	if len(ctx.Args) < 4 {
		ctx.Err(fmt.Errorf("invalid argument number"))
		//ctx.Println("Invalid argument number")
		return
	}
	target, err := parseSendTarget(ctx.Args[2:])
	if err != nil {
		ctx.Err(err)
		//fmt.Println(err)
		return
	}
	account, exists := c.w.GetAccount(ctx.Args[0])
	if !exists {
		ctx.Err(fmt.Errorf("account %s not managed", ctx.Args[0]))
		return
	}
	ctx.Println("Please Input Your Passphrase")
	passphrase := ctx.ReadPassword()
	if err := account.UnlockWithPassphrase(passphrase); err != nil {
		ctx.Err(err)
		//fmt.Println("Fail to unlock account", err)
		return
	}
	times, err := strconv.ParseInt(ctx.Args[1], 10, 32)
	if err != nil {
		ctx.Err(err)
		return
	}
	publickey := account.PublicKey()
	var txs []*types.Transaction

	now := time.Now()
	for i := 0; i < int(times); i++ {
		tx, err := c.a[ctx.Args[0]].CreateSendTransaction(publickey, target, account)
		if err != nil {
			ctx.Err(err)
			return
		}
		txs = append(txs, tx)
	}
	elapsed := time.Since(now)
	now = time.Now()
	ctx.Printf("%v transactions created, cost: %s\n", times, elapsed)
	conn := client.NewConnectionWithViper(viper.GetViper())
	defer conn.Close()
	for _, tx := range txs {
		if err := client.SendTransaction(conn, tx); err != nil {
			ctx.Err(err)
			return
		}
	}
	elapsed = time.Since(now)
	ctx.Printf("%v transaction sent, cost: %s\n", times, elapsed)
}
