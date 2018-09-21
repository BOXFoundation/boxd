// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package node

import (
	"os"
	"runtime"
	"sync"

	config "github.com/BOXFoundation/Quicksilver/config"
	"github.com/BOXFoundation/Quicksilver/log"
	p2p "github.com/BOXFoundation/Quicksilver/p2p"
	grpcserver "github.com/BOXFoundation/Quicksilver/rpc/server"
	storage "github.com/BOXFoundation/Quicksilver/storage"
	"github.com/jbenet/goprocess"
	"github.com/spf13/viper"
)

// RootProcess is the root process of the app
var RootProcess goprocess.Process

var logger = log.NewLogger("node")

func init() {
	RootProcess = goprocess.WithSignals(os.Interrupt)
}

// nodeServer is the boxd server instance, which contains all services,
// including grpc, p2p, database...
var nodeServer = struct {
	sm sync.Mutex

	cfg      config.Config
	database *storage.Database
	peer     *p2p.BoxPeer
	grpcsvr  *grpcserver.Server
}{}

// Start function starts node server.
func Start(v *viper.Viper) error {
	runtime.GOMAXPROCS(runtime.NumCPU())

	var cfg = &nodeServer.cfg
	// init config object from viper
	if err := v.Unmarshal(cfg); err != nil {
		logger.Fatal("Failed to read cfg", err) // exit in case of cfg error
	}

	cfg.Prepare() // make sure the cfg is correct and all directories are ok.

	log.Setup(&cfg.Log) // setup logger

	// start database life cycle
	var database, err = storage.NewDatabase(RootProcess, &cfg.Database)
	if err != nil {
		logger.Fatal("Failed to initialize database...") // exit in case of error during initialization of database
	}
	nodeServer.database = database

	peer, err := p2p.NewBoxPeer(&cfg.P2p, RootProcess)
	if err != nil {
		logger.Error("Failed to new BoxPeer...") // exit in case of error during creating p2p server instance
		RootProcess.Close()
	} else {
		nodeServer.peer = peer
		nodeServer.peer.Bootstrap()
	}

	if cfg.RPC.Enabled {
		nodeServer.grpcsvr, _ = grpcserver.NewServer(RootProcess, &cfg.RPC)
	}

	// var host, err = p2p.NewDefaultHost(RootProcess, net.ParseIP(v.GetString("node.listen.address")), uint(v.GetInt("node.listen.port")))
	// if err != nil {
	// 	logger.Error(err)
	// 	return err
	// }

	// connect to other peers passed via commandline
	// for _, addr := range v.GetStringSlice("node.addpeer") {
	// 	if maddr, err := ma.NewMultiaddr(addr); err == nil {
	// 		err := host.ConnectPeer(RootProcess, maddr)
	// 		if err != nil {
	// 			logger.Warn(err)
	// 		} else {
	// 			logger.Infof("Peer %s connected.", maddr)
	// 		}
	// 	} else {
	// 		logger.Warnf("Invalid multiaddress %s", addr)
	// 	}
	// }

	select {
	case <-RootProcess.Closing():
		logger.Info("Box server is shutting down...")
	}

	select {
	case <-RootProcess.Closed():
		logger.Info("Box server is down.")
	}

	return nil
}
