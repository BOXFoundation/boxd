// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"os"
	"os/signal"
	"runtime"

	"github.com/BOXFoundation/Quicksilver/cmd"
	config "github.com/BOXFoundation/Quicksilver/config"
	"github.com/BOXFoundation/Quicksilver/log"
	p2p "github.com/BOXFoundation/Quicksilver/p2p"
)

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	cmd.Execute(startNodeServer)
}

// NodeContext is the context to start the server
var NodeContext context.Context

// NodeCancel is the cancel function to stop the server
var NodeCancel context.CancelFunc

var logger *log.Logger

func init() {
	NodeContext, NodeCancel = context.WithCancel(context.Background())

	logger = log.NewLogger("main")
}

// start node server
func startNodeServer(cfg *config.Config) error {
	log.Setup(cfg) // setup logger

	interruptListener() // listenning OS Signals

	var host, err = p2p.NewDefaultHost(NodeContext, cfg.ListenAddr, cfg.ListenPort)
	if err != nil {
		logger.Error(err)
		return err
	}

	// connect to other peers passed via commandline
	for _, multiaddr := range cfg.AddPeers {
		err := host.ConnectPeer(NodeContext, multiaddr)
		if err != nil {
			logger.Warn(err)
		} else {
			logger.Infof("Peer %s connected.", multiaddr)
		}
	}

	select {
	case <-NodeContext.Done():
		logger.Info("Box server is down.")
	}

	return nil
}

// interruptListener listens for OS Signals such as SIGINT (Ctrl+C) and shutdown
// requests via func ContexCancel.
func interruptListener() {
	// interruptSignals defines the default signals to catch in order to do a proper
	// shutdown.  This may be modified during init depending on the platform.
	var interruptSignals = []os.Signal{os.Interrupt}

	go func() {
		interruptChannel := make(chan os.Signal, 1)
		signal.Notify(interruptChannel, interruptSignals...)

		// Listen for initial shutdown signal and close the returned
		// channel to notify the caller.
		for {
			select {
			case sig := <-interruptChannel:
				logger.Infof("Received signal (%s). Shutting down...", sig)
				NodeCancel()
			}
		}
	}()
}
