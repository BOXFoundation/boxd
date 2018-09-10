// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

// Package cmd defines and implements command-line commands and flags
// used by Quicksilver. Commands and flags are implemented using Cobra.
package cmd

import (
	"fmt"
	"net"
	"os"
	"strings"

	config "github.com/BOXFoundation/Quicksilver/config"
	log "github.com/BOXFoundation/Quicksilver/log"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/spf13/cobra"
)

// CommandFunc is the callback function to pass back the parameters as config object
type CommandFunc func(cfg *config.Config) error

// root command
var verbose bool
var loglevel string

var cmdBox = &cobra.Command{
	Use:   "box [# verbose]",
	Short: "BOX Payout command-line interface",
	Long: `BOX Payout, a lightweight blockchain built for processing
			multi-party payments on digital content apps.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Starting p2p node...")
	},
}

func init() {
	cmdBox.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")
	cmdBox.PersistentFlags().StringVarP(&loglevel, "log-level", "l", "info", "log level [debug|info|warn|error|fatal]")
}

// DefaultPeerPort is the default listen port for box p2p message.
const DefaultPeerPort = 19199

// fullnode sub-command
func initStartCommand(root *cobra.Command, callback CommandFunc) {
	var addpeers = make([]string, 0, 8)
	var listenAddr net.IP
	var listenPort uint

	var cmdStart = &cobra.Command{
		Use:   "start [# addpeer] [# listen-addr] [# listen-port]",
		Short: "starts fullnode server.",
		Long:  `starts fullnode server. It will start a p2p server and then sync blockchain data from remote peers.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			peers := make([]ma.Multiaddr, 0, 8)
			for _, addr := range addpeers {
				var pa, err = ma.NewMultiaddr(addr)

				if err == nil {
					peers = append(peers, pa)
				} else {
					fmt.Printf("Invalid multi address: %s\n", addr)
				}
			}
			var llevel = log.LevelInfo
			loglevel = strings.ToLower(loglevel)
			if loglevel, ok := log.LevelValue[loglevel]; ok {
				llevel = loglevel
			}

			var cfg = &config.Config{
				ListenAddr:      listenAddr,
				ListenPort:      listenPort,
				AddPeers:        peers,
				DefaultPeerPort: DefaultPeerPort,
				LogLevel:        int(llevel),
			}

			return callback(cfg)
		},
	}

	cmdStart.Flags().StringSliceVar(&addpeers, "addpeer", []string{}, "addresse and port of remote peers, seperated by comma.")
	cmdStart.Flags().IPVar(&listenAddr, "listen-addr", net.IPv4zero, "local p2p listen address.")
	cmdStart.Flags().UintVar(&listenPort, "listen-port", DefaultPeerPort, "local p2p listen address.")

	root.AddCommand(cmdStart)
}

// Execute executes the root command
func Execute(callbacks ...CommandFunc) {
	initStartCommand(cmdBox, callbacks[0])
	if err := cmdBox.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
