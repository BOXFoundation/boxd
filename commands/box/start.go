// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package box

import (
	"net"

	"github.com/BOXFoundation/Quicksilver/node"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// DefaultPeerPort is the default listen port for box p2p message.
const DefaultPeerPort = 19199

// startCmd represents the start command, to start fullnode server.
var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start full node server.",
	RunE: func(cmd *cobra.Command, args []string) error {
		return node.Start(viper.GetViper())
	}}

func init() {
	rootCmd.AddCommand(startCmd)

	startCmd.Flags().StringSlice("addpeer", []string{}, "addresse and port of remote peers, seperated by comma.")
	viper.BindPFlag("p2p.addpeer", startCmd.Flags().Lookup("addpeer"))

	startCmd.Flags().IP("listen-addr", net.IPv4zero, "local p2p listen address.")
	viper.BindPFlag("p2p.address", startCmd.Flags().Lookup("listen-addr"))

	startCmd.Flags().Uint("listen-port", DefaultPeerPort, "local p2p listen port.")
	viper.BindPFlag("p2p.port", startCmd.Flags().Lookup("listen-port"))

	startCmd.Flags().Bool("rpc", true, "start rpc server (default true).")
	viper.BindPFlag("rpc.enabled", startCmd.Flags().Lookup("rpc"))

	startCmd.Flags().String("database", "rocksdb", "database name [rocksdb|mem]")
	viper.BindPFlag("database.name", startCmd.Flags().Lookup("database"))

	viper.SetDefault("p2p.key_path", "peer.key")
}
