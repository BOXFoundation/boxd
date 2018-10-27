// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package boxd

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/BOXFoundation/boxd/blocksync"
	"github.com/BOXFoundation/boxd/boxd/eventbus"
	"github.com/BOXFoundation/boxd/boxd/service"
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
	"github.com/jbenet/goprocess"
)

var logger = log.NewLogger("boxd") // logger for node package

// Server is the boxd server instance, which contains all services,
// including grpc, p2p, database...
type Server struct {
	sm   sync.Mutex
	proc goprocess.Process

	bus         eventbus.Bus
	cfg         *config.Config
	database    *storage.Database
	peer        *p2p.BoxPeer
	grpcsvr     *grpcserver.Server
	blockChain  *chain.BlockChain
	txPool      *txpool.TransactionPool
	syncManager *blocksync.SyncManager
}

// NewServer new a boxd server
func NewServer(cfg *config.Config) *Server {
	server := &Server{
		proc: goprocess.WithSignals(os.Interrupt),
		bus:  eventbus.Default(),
		cfg:  cfg,
	}
	server.initEventListener()
	server.proc.SetTeardown(server.teardown)
	return server
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

// Prepare to run the boxd server
func (server *Server) Prepare() {
	runtime.GOMAXPROCS(runtime.NumCPU())

	// make sure the cfg is correct and all directories are ok.
	server.cfg.Prepare()
	// setup logger
	log.Setup(&server.cfg.Log)
}

var _ service.Server = (*Server)(nil)

// Run to start node server.
func (server *Server) Run() error {
	var proc = server.proc // parent goprocess
	var cfg = server.cfg

	// start database life cycle
	database, err := storage.NewDatabase(proc, &cfg.Database)
	if err != nil {
		logger.Fatalf("Failed to initialize database: %v", err) // exit in case of error during initialization of database
	}
	server.database = database

	// start p2p service
	peer, err := p2p.NewBoxPeer(database.Proc(), &cfg.P2p, database, server.bus)
	if err != nil {
		logger.Fatalf("Failed to new BoxPeer...") // exit in case of error during creating p2p server instance
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

	blockChain, err := chain.NewBlockChain(peer.Proc(), peer, database)
	if err != nil {
		logger.Fatalf("Failed to new BlockChain...", err) // exit in case of error during creating p2p server instance
	}
	server.blockChain = blockChain

	txPool := txpool.NewTransactionPool(blockChain.Proc(), peer, blockChain)
	server.txPool = txPool

	consensus := dpos.NewDpos(txPool.Proc(), blockChain, txPool, peer, &cfg.Dpos)

	syncManager := blocksync.NewSyncManager(blockChain, peer, consensus, blockChain.Proc())
	server.syncManager = syncManager
	blockChain.Setup(syncManager)

	peer.Run()
	blockChain.Run()
	txPool.Run()
	// if cfg.Dpos.EnableMint {
	// 	consensus.Run()
	// }
	consensus.Run()
	syncManager.Run()
	if len(cfg.P2p.Seeds) > 0 {
		syncManager.StartSync()
	}

	if cfg.RPC.Enabled {
		server.grpcsvr, _ = grpcserver.NewServer(txPool.Proc(), &cfg.RPC, blockChain, txPool, server.bus)
		server.grpcsvr.Run()
	}

	// goprocesses dependencies
	//            root
	//              |
	//           database
	//              |
	//            peer
	//              |
	//            chain
	//              |
	//            txpool
	//             /   \
	//          rpc    consensus

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

// Proc returns the goprocess to run the server
func (server *Server) Proc() goprocess.Process {
	return server.proc
}

// Stop the server
func (server *Server) Stop() {
	server.proc.Close()
}

func (server *Server) initEventListener() {
	// TopicSetDebugLevel
	server.bus.Reply(eventbus.TopicSetDebugLevel, func(newLevel string, out chan<- bool) {
		out <- log.SetLogLevel(newLevel)
	}, false)

	// TopicGetDatabaseKeys
	server.bus.Reply(eventbus.TopicGetDatabaseKeys, func(parent context.Context, table string, prefix string, skip int32, limit int32, out chan<- []string) {
		var result []string
		defer func() {
			out <- result
		}()

		var s storage.Table
		var err error
		if len(table) == 0 {
			s = server.database
		} else if s, err = server.database.Table(table); err != nil {
			return
		}

		ctx, cancel := context.WithTimeout(parent, time.Second*5)
		defer cancel()

		var keys <-chan []byte
		if len(prefix) == 0 {
			keys = s.IterKeys(ctx)
		} else {
			keys = s.IterKeysWithPrefix(ctx, []byte(prefix))
		}
		var i = 0
		for k := range keys {
			if i >= int(skip) {
				if i >= int(skip+limit) {
					break
				}
				result = append(result, string(k))
			}
			i++
		}
	}, false)

	// TopicGetDatabaseValue
	server.bus.Reply(eventbus.TopicGetDatabaseValue, func(table string, key string, out chan<- []byte) {
		var result []byte
		defer func() {
			out <- result
		}()

		var s storage.Table
		var err error

		if len(table) == 0 {
			s = server.database
		} else {
			s, err = server.database.Table(table)
			if err != nil {
				return
			}
		}
		if v, err := s.Get([]byte(key)); err == nil {
			result = v
		}
	}, false)
}
