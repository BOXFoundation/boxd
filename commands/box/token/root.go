// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package tokencmd

import (
	"fmt"
	"path"
	"strconv"

	root "github.com/BOXFoundation/boxd/commands/box/root"
	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/crypto"
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
	Use:   "token",
	Short: "Token subcommand",
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
			Use:   "issue",
			Short: "issue a new token",
			Run:   createTokenCmdFunc,
		},
		&cobra.Command{
			Use:   "transfer",
			Short: "transfer tokens",
			Run:   transferTokenCmdFunc,
		},
		&cobra.Command{
			Use:   "getbalance",
			Short: "get token balance",
			Run:   getTokenBalanceCmdFunc,
		},
	)
}

func createTokenCmdFunc(cmd *cobra.Command, args []string) {
	fmt.Println("createToken called")
	if len(args) != 4 {
		fmt.Println("Invalid argument number")
		return
	}
	toAddr := &types.AddressPubKeyHash{}
	tokenName := args[2]
	err1 := toAddr.SetString(args[1])
	tokenTotalSupply, err2 := strconv.Atoi(args[3])
	if err1 != nil || err2 != nil {
		fmt.Println("Invalid argument format")
		return
	}
	wltMgr, err := wallet.NewWalletManager(walletDir)
	if err != nil {
		fmt.Println(err)
		return
	}
	// from pub key hash
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
	tx, err := client.CreateTokenIssueTx(viper.GetViper(), account.PubKeyHash(),
		toAddr.ScriptAddress(), account.PublicKey(), tokenName, uint64(tokenTotalSupply), account)
	if err != nil {
		fmt.Println(err)
	} else {
		fmt.Println(tx)
	}
}

func transferTokenCmdFunc(cmd *cobra.Command, args []string) {
	fmt.Println("transferToken called")
	if len(args) != 5 {
		fmt.Println("Invalid argument number")
		return
	}
	toAddr := &types.AddressPubKeyHash{}
	err1 := toAddr.SetString(args[1])
	tokenTxHash := &crypto.HashType{}
	err2 := tokenTxHash.SetString(args[2])
	tokenTxOutIdx, err3 := strconv.Atoi(args[3])
	amount, err4 := strconv.Atoi(args[4])
	if err1 != nil || err2 != nil || err3 != nil || err4 != nil {
		fmt.Println("Invalid argument format")
		return
	}
	wltMgr, err := wallet.NewWalletManager(walletDir)
	if err != nil {
		fmt.Println(err)
		return
	}
	// from pub key hash
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
	tx, err := client.CreateTokenTransferTx(viper.GetViper(), account.PubKeyHash(),
		toAddr.ScriptAddress(), account.PublicKey(), tokenTxHash, uint32(tokenTxOutIdx), uint64(amount), account)
	if err != nil {
		fmt.Println(err)
	} else {
		fmt.Println(tx)
	}
}

func getTokenBalanceCmdFunc(cmd *cobra.Command, args []string) {
	fmt.Println("getTokenBalance called")
	if len(args) != 3 {
		fmt.Println("Invalid argument number")
		return
	}
	tokenTxHash := &crypto.HashType{}
	err1 := tokenTxHash.SetString(args[1])
	tokenTxOutIdx, err2 := strconv.Atoi(args[2])
	if err1 != nil || err2 != nil {
		fmt.Println("Invalid argument format")
		return
	}
	wltMgr, err := wallet.NewWalletManager(walletDir)
	if err != nil {
		fmt.Println(err)
		return
	}
	// from pub key hash
	account, exists := wltMgr.GetAccount(args[0])
	if !exists {
		fmt.Printf("Account %s not managed\n", args[0])
		return
	}

	balance := client.GetTokenBalance(viper.GetViper(), account.PubKeyHash(), tokenTxHash, uint32(tokenTxOutIdx))
	fmt.Printf("Token balance of %s: %d\n", args[0], balance)
}
