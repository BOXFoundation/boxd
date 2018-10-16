// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package transactioncmd

import (
	"encoding/hex"
	"fmt"
	"strconv"

	root "github.com/BOXFoundation/boxd/commands/box/root"
	"github.com/BOXFoundation/boxd/rpc/client"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cfgFile string

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "tx",
	Short: "A brief description of your application",
	Long: `A longer description that spans multiple lines and likely contains
examples and usage of using your application. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	// Uncomment the following line if your bare application
	// has an action associated with it:
	//	Run: func(cmd *cobra.Command, args []string) { },
}

// Init adds the sub command to the root command.
func init() {
	root.RootCmd.AddCommand(rootCmd)
	rootCmd.AddCommand(
		&cobra.Command{
			Use:   "listutxos",
			Short: "list all utxos",
			RunE: func(cmd *cobra.Command, args []string) error {
				fmt.Println("list utxos called")
				return client.ListUtxos(viper.GetViper())
			},
		},
		&cobra.Command{
			Use:   "sendfrom [fromaccount] [toaddress] [amount]",
			Short: "Send coins from an account to an address",
			RunE: func(cmd *cobra.Command, args []string) error {
				fmt.Println("sendfrom called")
				if len(args) != 3 {
					return fmt.Errorf("Invalid argument number")
				}
				fromPubKey, err1 := hex.DecodeString(args[0])
				toPubKey, err2 := hex.DecodeString(args[1])
				amount, err3 := strconv.Atoi(args[2])
				if err1 != nil || err2 != nil || err3 != nil {
					return fmt.Errorf("Invalid argument format")
				}
				return client.CreateTransaction(viper.GetViper(), fromPubKey, toPubKey, int64(amount))
			},
		},
		&cobra.Command{
			Use:   "sendmany [fromaccount] [toaddresslist]",
			Short: "Send coins to multiple addresses",
			Run: func(cmd *cobra.Command, args []string) {
				fmt.Println("sendmany called")
			},
		},
		&cobra.Command{
			Use:   "sendtoaddress [address]",
			Short: "Send coins to an address",
			Run: func(cmd *cobra.Command, args []string) {
				fmt.Println("sendtoaddress called")
			},
		},
		&cobra.Command{
			Use:   "signrawtx [rawtx]",
			Short: "Sign a transaction with privatekey and send it to the network",
			Run: func(cmd *cobra.Command, args []string) {
				fmt.Println("signrawtx called")
			},
		},
	)
}
