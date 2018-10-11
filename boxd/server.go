// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package boxd

import (
	"fmt"
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/BOXFoundation/boxd/boxd/eventbus"
	config "github.com/BOXFoundation/boxd/config"
	"github.com/BOXFoundation/boxd/consensus/dpos"
	"github.com/BOXFoundation/boxd/core/chain"
	"github.com/BOXFoundation/boxd/core/txpool"
	"github.com/BOXFoundation/boxd/log"
	p2p "github.com/BOXFoundation/boxd/p2p"
	grpcserver "github.com/BOXFoundation/boxd/rpc/server"
	storage "github.com/BOXFoundation/boxd/storage"
	_ "github.com/BOXFoundation/boxd/storage/memdb"   // init memdb
	_ "github.com/BOXFoundation/boxd/storage/rocksdb" // init rocksdb
	"github.com/BOXFoundation/boxd/types"
	"github.com/jbenet/goprocess"
	"github.com/spf13/viper"
)

var logger = log.NewLogger("boxd") // logger for node package

// BoxdServer is the boxd server instance, which contains all services,
// including grpc, p2p, database...
// var BoxdServer = struct {
// 	sm   sync.Mutex
// 	proc goprocess.Process

// 	cfg      config.Config
// 	database *storage.Database
// 	peer     *p2p.BoxPeer
// 	grpcsvr  *grpcserver.Server
// 	bc       *chain.BlockChain
// }{
// 	proc: goprocess.WithSignals(os.Interrupt),
// }

// Server is the boxd server instance, which contains all services,
// including grpc, p2p, database...
type Server struct {
	sm   sync.Mutex
	proc goprocess.Process

	bus      eventbus.Bus
	cfg      config.Config
	database *storage.Database
	peer     *p2p.BoxPeer
	grpcsvr  *grpcserver.Server
	bc       *chain.BlockChain
	tp       *txpool.TransactionPool
}

// Cfg return server config.
func (server *Server) Cfg() types.Config {
	return server.cfg
}

// BlockChain return block chain ref.
func (server *Server) BlockChain() *chain.BlockChain {
	return server.bc
}

// TxPool returns tx pool ref.
func (server *Server) TxPool() *txpool.TransactionPool {
	return server.tp
}

// teardown
func (server *Server) teardown() error {
	done := make(chan int)
	defer close(done)

	timer := time.NewTimer(15 * time.Second)
	go func() {
		server.bus.WaitAsync() // get all async msgs processed
		done <- 0
	}()

	select {
	case <-timer.C:
		logger.Warn("Box server teardown timeout.")
		return fmt.Errorf("timeout to shutdown eventbus")
	case <-done:
		logger.Info("Box server teardown finished.")
		return nil
	}
}

// NewServer new a boxd server
func NewServer() *Server {
	server := &Server{
		proc: goprocess.WithSignals(os.Interrupt),
		bus:  eventbus.New(),
	}
	server.proc.SetTeardown(server.teardown)
	return server
}

// Start function starts node server.
func (server *Server) Start(v *viper.Viper) error {
	runtime.GOMAXPROCS(runtime.NumCPU())

	var proc = server.proc // parent goprocess
	var cfg = &server.cfg
	// init config object from viper
	if err := v.Unmarshal(cfg); err != nil {
		logger.Fatal("Failed to read cfg", err) // exit in case of cfg error
	}

	cfg.Prepare() // make sure the cfg is correct and all directories are ok.

	log.Setup(&cfg.Log) // setup logger

	// start database life cycle
	var database, err = storage.NewDatabase(proc, &cfg.Database)
	if err != nil {
		logger.Fatalf("Failed to initialize database: %v", err) // exit in case of error during initialization of database
	}
	server.database = database

	// start p2p service
	peer, err := p2p.NewBoxPeer(proc, &cfg.P2p, database)
	if err != nil {
		logger.Fatalf("Failed to new BoxPeer...") // exit in case of error during creating p2p server instance
		proc.Close()
	}
	// Add peers configured by user
	for _, addr := range cfg.P2p.AddPeers {
		if err := peer.AddAddrToPeerstore(addr); err != nil {
			logger.Errorf("Add peer error: %v", err)
		} else {
			logger.Infof("Peer %s added.", addr)
		}
	}
	server.peer = peer

	bc, err := chain.NewBlockChain(proc, peer, database.Storage)
	if err != nil {
		logger.Fatalf("Failed to new BlockChain...", err) // exit in case of error during creating p2p server instance
		proc.Close()
	}
	server.bc = bc

	tp := txpool.NewTransactionPool(proc, peer, bc)
	server.tp = tp

	consensus := dpos.NewDpos(bc, tp, peer, proc, &cfg.Dpos)

	if cfg.RPC.Enabled {
		server.grpcsvr, _ = grpcserver.NewServer(proc, &cfg.RPC, server)
	}

	peer.Run()
	bc.Run()
	tp.Run()
	consensus.Run()

	select {
	case <-proc.Closing():
		logger.Info("Box server is shutting down...")
	}

	select {
	case <-proc.Closed():
		logger.Info("Box server is down.")
	}

	return nil
}
