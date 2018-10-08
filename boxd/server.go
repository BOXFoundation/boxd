// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package boxd

import (
	"os"
	"runtime"
	"sync"

	config "github.com/BOXFoundation/Quicksilver/config"
	"github.com/BOXFoundation/Quicksilver/consensus/dpos"
	"github.com/BOXFoundation/Quicksilver/core"
	"github.com/BOXFoundation/Quicksilver/log"
	p2p "github.com/BOXFoundation/Quicksilver/p2p"
	grpcserver "github.com/BOXFoundation/Quicksilver/rpc/server"
	storage "github.com/BOXFoundation/Quicksilver/storage"
	_ "github.com/BOXFoundation/Quicksilver/storage/memdb"   // init memdb
	_ "github.com/BOXFoundation/Quicksilver/storage/rocksdb" // init rocksdb
	"github.com/BOXFoundation/Quicksilver/types"
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
// 	bc       *core.BlockChain
// }{
// 	proc: goprocess.WithSignals(os.Interrupt),
// }

// Server is the boxd server instance, which contains all services,
// including grpc, p2p, database...
type Server struct {
	sm   sync.Mutex
	proc goprocess.Process

	cfg      config.Config
	database *storage.Database
	peer     *p2p.BoxPeer
	grpcsvr  *grpcserver.Server
	bc       *core.BlockChain
}

// Cfg return server config.
func (server *Server) Cfg() types.Config {
	return server.cfg
}

// BlockChain return block chain ref.
func (server *Server) BlockChain() *core.BlockChain {
	return server.bc
}

// NewServer new a boxd server
func NewServer() *Server {
	return &Server{
		proc: goprocess.WithSignals(os.Interrupt),
	}
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

	bc, err := core.NewBlockChain(proc, peer, database.Storage)
	if err != nil {
		logger.Fatalf("Failed to new BlockChain...", err) // exit in case of error during creating p2p server instance
		proc.Close()
	}
	server.bc = bc
	consensus := dpos.NewDpos(bc, peer, proc, &cfg.Dpos)

	if cfg.RPC.Enabled {
		server.grpcsvr, _ = grpcserver.NewServer(proc, &cfg.RPC, server)
	}

	peer.Run()
	bc.Run()
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
