// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"fmt"
	"runtime"

	"github.com/BOXFoundation/Quicksilver/cmd"
	config "github.com/BOXFoundation/Quicksilver/config"
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

func init() {
	NodeContext, NodeCancel = context.WithCancel(context.Background())
}

// start node server
func startNodeServer(cfg *config.Config) error {
	var host, err = p2p.NewDefaultHost(NodeContext, cfg.ListenAddr, cfg.ListenPort)
	if err != nil {
		return err
	}

	// connect to other peers passed via commandline
	for _, multiaddr := range cfg.AddPeers {
		err := host.ConnectPeer(NodeContext, multiaddr)
		if err != nil {
			fmt.Println(err)
		} else {
			fmt.Printf("Peer %s connected.\n", multiaddr)
		}
	}

	select {} // TODO loop

	return nil
}
