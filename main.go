// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package main

import (
	"os"
	"runtime"

	"github.com/BOXFoundation/Quicksilver/cmd"
	config "github.com/BOXFoundation/Quicksilver/config"
	"github.com/BOXFoundation/Quicksilver/log"
	p2p "github.com/BOXFoundation/Quicksilver/p2p"
	"github.com/jbenet/goprocess"
)

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	cmd.Execute(startNodeServer)
}

// RootProcess is the root process of the app
var RootProcess goprocess.Process

var logger *log.Logger

func init() {
	RootProcess = goprocess.WithSignals(os.Interrupt)
	logger = log.NewLogger("main")
}

// start node server
func startNodeServer(cfg *config.Config) error {
	log.Setup(cfg) // setup logger

	var host, err = p2p.NewDefaultHost(RootProcess, cfg.ListenAddr, cfg.ListenPort)
	if err != nil {
		logger.Error(err)
		return err
	}

	// connect to other peers passed via commandline
	for _, multiaddr := range cfg.AddPeers {
		err := host.ConnectPeer(RootProcess, multiaddr)
		if err != nil {
			logger.Warn(err)
		} else {
			logger.Infof("Peer %s connected.", multiaddr)
		}
	}

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
