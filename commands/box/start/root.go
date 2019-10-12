// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package start

import (
	"fmt"
	"net"

	"github.com/BOXFoundation/boxd/boxd"
	root "github.com/BOXFoundation/boxd/commands/box/root"
	"github.com/BOXFoundation/boxd/config"
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
		cfg := &config.Config{}
		// init config object from viper
		if err := viper.Unmarshal(cfg); err != nil {
			// exit in case of cfg error
			fmt.Print("Failed to read config ", err)
			return nil
		}
		boxd := boxd.NewServer(cfg)
		boxd.Prepare()
		return boxd.Run()
	},
}

func init() {
	root.RootCmd.AddCommand(startCmd)

	startCmd.Flags().StringSlice("addpeer", []string{}, "addresse and port of remote peers, seperated by comma.")
	viper.BindPFlag("p2p.addpeer", startCmd.Flags().Lookup("addpeer"))

	startCmd.Flags().IP("listen-addr", net.IPv4zero, "local p2p listen address.")
	viper.BindPFlag("p2p.address", startCmd.Flags().Lookup("listen-addr"))

	startCmd.Flags().Uint("listen-port", DefaultPeerPort, "local p2p listen port.")
	viper.BindPFlag("p2p.port", startCmd.Flags().Lookup("listen-port"))

	startCmd.Flags().Bool("rpc", true, "start rpc server (default true).")
	viper.BindPFlag("rpc.enable", startCmd.Flags().Lookup("rpc"))

	startCmd.Flags().String("database", "rocksdb", "database name [rocksdb|mem]")
	viper.BindPFlag("database.name", startCmd.Flags().Lookup("database"))

	viper.SetDefault("p2p.key_path", "peer.key")
}
