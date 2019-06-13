// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package ctl

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strconv"

	"github.com/BOXFoundation/boxd/commands/box/root"
	"github.com/BOXFoundation/boxd/config"
	"github.com/BOXFoundation/boxd/rpc/rpcutil"
	"github.com/BOXFoundation/boxd/util"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	walletDir string

	defaultWalletDir = path.Join(util.HomeDir(), ".box_keystore")
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "vm [command]",
	Short: "The vm command line interface",
	// Uncomment the following line if your bare application
	// has an action associated with it:
	//	Run: func(cmd *cobra.Command, args []string) { },
}

func init() {
	root.RootCmd.AddCommand(rootCmd)
	rootCmd.PersistentFlags().StringVar(&walletDir, "wallet_dir", defaultWalletDir, "Specify directory to search keystore files")
	rootCmd.AddCommand(
		&cobra.Command{
			Use:   "importabi [contract_addr] [abi]",
			Short: "Import abi for contract.",
			Run:   importAbi,
		},
		&cobra.Command{
			Use:   "docall [from] [to] [data] [height] [timeout]",
			Short: "Call contract.",
			Run:   docall,
		},
	)
}

func importAbi(cmd *cobra.Command, args []string) {
	if len(args) < 2 {
		fmt.Println("Invalide argument number")
		return
	}
	abifile := args[0] + ".abi"
	abi := args[1]

	if _, err := os.Stat(abifile); !os.IsNotExist(err) {
		fmt.Println("The abi already exists and the command will overwrite it.")
	}
	file, err := os.OpenFile(abifile, os.O_WRONLY|os.O_TRUNC|os.O_APPEND|os.O_CREATE, 0600)
	defer file.Close()
	if err != nil {
		fmt.Println(err)
		return
	}
	file.Write([]byte(abi))
}

func docall(cmd *cobra.Command, args []string) {
	if len(args) < 3 {
		fmt.Println("Invalide argument number")
		return
	}
	from := args[0]
	to := args[1]
	data := args[2]

	var height, timeout uint64
	var err error
	if len(args) >= 4 {
		height, err = strconv.ParseUint(args[3], 10, 32)
		if err != nil {
			fmt.Println(err)
			return
		}
	}
	if len(args) >= 5 {
		timeout, err = strconv.ParseUint(args[4], 10, 32)
		if err != nil {
			fmt.Println(err)
			return
		}
	}

	conn, err := rpcutil.GetGRPCConn(getRPCAddr())

	abifile := to + ".abi"
	if _, err := os.Stat(abifile); os.IsNotExist(err) {
		fmt.Println("Please import abi at first")
	}
	fileContent, err := ioutil.ReadFile(abifile)

	output, err := rpcutil.DoCall(conn, from, to, data, uint32(height), uint32(timeout), fileContent)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer conn.Close()
	fmt.Println(output)
}

func getRPCAddr() string {
	var cfg config.Config
	viper.Unmarshal(&cfg)
	return fmt.Sprintf("%s:%d", cfg.RPC.Address, cfg.RPC.Port)
}
