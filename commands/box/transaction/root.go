// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package transactioncmd

import (
	"encoding/hex"
	"fmt"
	"path"
	"strconv"

	root "github.com/BOXFoundation/boxd/commands/box/root"
	"github.com/BOXFoundation/boxd/rpc/client"
	"github.com/BOXFoundation/boxd/util"
	"github.com/BOXFoundation/boxd/wallet"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cfgFile string
var walletDir string
var defaultWalletDir = path.Join(util.HomeDir(), ".box_keystore")

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
	rootCmd.PersistentFlags().StringVar(&walletDir, "wallet_dir", defaultWalletDir, "Specify directory to search keystore files")
	rootCmd.AddCommand(
		&cobra.Command{
			Use:   "listutxos",
			Short: "list all utxos",
			Run:   listAllUtxoCmdFunc,
		},
		&cobra.Command{
			Use:   "sendfrom [fromaccount] [toaddress] [amount]",
			Short: "Send coins from an account to an address",
			Run:   sendFromCmdFunc,
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

func listAllUtxoCmdFunc(cmd *cobra.Command, args []string) {
	fmt.Println("list utxos called")
	r, err := client.ListUtxos(viper.GetViper())
	if err != nil {
		fmt.Println(err)
	} else {
		fmt.Println(util.PrettyPrint(r))
	}
}

func sendFromCmdFunc(cmd *cobra.Command, args []string) {
	fmt.Println("sendfrom called")
	if len(args) != 3 {
		fmt.Println("Invalid argument number")
		return
	}
	fromPubKeyHash, err1 := hex.DecodeString(args[0])
	toPubKeyHash, err2 := hex.DecodeString(args[1])
	amount, err3 := strconv.Atoi(args[2])
	if err1 != nil || err2 != nil || err3 != nil {
		fmt.Println("Invalid argument format")
		return
	}
	wltMgr, err := wallet.NewWalletManager(walletDir)
	if err != nil {
		fmt.Println(err)
		return
	}
	account, exists := wltMgr.GetAccount(args[0])
	if !exists {
		fmt.Printf("Account %s not managed\n", args[0])
		return
	}
	passphrase, err := wallet.ReadPassphraseStdin()
	if err != nil {
		fmt.Println(err)
		return
	}
	if err := account.UnlockWithPassphrase(passphrase); err != nil {
		fmt.Println("Fail to unlock account", err)
		return
	}
	tx, err := client.CreateTransaction(viper.GetViper(), fromPubKeyHash, toPubKeyHash, account.PublicKey(), int64(amount), account)
	if err != nil {
		fmt.Println(err)
	} else {
		fmt.Println(util.PrettyPrint(tx))
	}
}
