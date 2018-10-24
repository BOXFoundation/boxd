// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package walletcmd

import (
	"encoding/hex"
	"fmt"
	"path"

	root "github.com/BOXFoundation/boxd/commands/box/root"
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
	Use:   "wallet",
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
			Use:   "newaccount [account]",
			Short: "Create a new account",
			Run:   newAccountCmdFunc,
		},
		&cobra.Command{
			Use:   "dumpprivkey [address]",
			Short: "Dump private key for an address",
			Run:   dumpPrivKeyCmdFunc,
		},
		&cobra.Command{
			Use:   "dumpwallet [filename]",
			Short: "Dump wallet to a file",
			Run: func(cmd *cobra.Command, args []string) {
				fmt.Println("dumpwallet called")
			},
		},
		&cobra.Command{
			Use:   "encryptwallet [passphrase]",
			Short: "Encrypt a wallet with a passphrase",
			Run: func(cmd *cobra.Command, args []string) {
				fmt.Println("encryptwallet called")
			},
		},
		&cobra.Command{
			Use:   "getbalance [account]",
			Short: "Get the balance for an account",
			Run: func(cmd *cobra.Command, args []string) {
				fmt.Println("getbalance called")
			},
		},
		&cobra.Command{
			Use:   "getwalletinfo",
			Short: "Get the basic informatio for a wallet",
			Run: func(cmd *cobra.Command, args []string) {
				fmt.Println("getwalletinfo called")
			},
		},
		&cobra.Command{
			Use:   "importprivkey [privatekey]",
			Short: "Import a private key from other wallets",
			Run:   importPrivateKeyCmdFunc,
		},
		&cobra.Command{
			Use:   "importwallet [filename]",
			Short: "Import a wallet from a file",
			Run: func(cmd *cobra.Command, args []string) {
				fmt.Println("importwallet called")
			},
		},
		&cobra.Command{
			Use:   "listaccounts",
			Short: "List local accounts",
			Run:   listAccountCmdFunc,
		},
		&cobra.Command{
			Use:   "listreceivedbyaccount",
			Short: "List received transactions groups by account",
			Run: func(cmd *cobra.Command, args []string) {
				fmt.Println("listreceivedbyaccount called")
			},
		},
		&cobra.Command{
			Use:   "listreceivedbyaddress",
			Short: "List received transactions grouped by address",
			Run: func(cmd *cobra.Command, args []string) {
				fmt.Println("listreceivedbyaddress called")
			},
		},
		&cobra.Command{
			Use:   "listtransactions [account] [count] [from]",
			Short: "List transactions for an account",
			Run:   listTransactionsCmdFunc,
		},
	)
}

func newAccountCmdFunc(cmd *cobra.Command, args []string) {
	wltMgr, err := wallet.NewWalletManager(walletDir)
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	passphrase, err := wallet.ReadPassphraseStdin()
	if err != nil {
		fmt.Println(err)
		return
	}
	acc, addr, err := wltMgr.NewAccount(passphrase)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Printf("Created new account: %s\nAddress:%s", acc, addr)
}

func importPrivateKeyCmdFunc(cmd *cobra.Command, args []string) {
	if len(args) < 1 {
		fmt.Println("Missing param private key")
		return
	}
	privKeyBytes, err := hex.DecodeString(args[0])
	if err != nil {
		fmt.Println("Invalid private key", err)
		return
	}
	wltMgr, err := wallet.NewWalletManager(walletDir)
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	privKey, _, err := crypto.KeyPairFromBytes(privKeyBytes)
	if err != nil {
		fmt.Println(err)
		return
	}
	passphrase, err := wallet.ReadPassphraseStdin()
	if err != nil {
		fmt.Println(err)
		return
	}
	acc, addr, err := wltMgr.NewAccountWithPrivKey(privKey, passphrase)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Printf("Created new account: %s\nAddress:%s", acc, addr)
}

func listAccountCmdFunc(cmd *cobra.Command, args []string) {
	fmt.Println("listaccounts called")
	wltMgr, err := wallet.NewWalletManager(walletDir)
	if err != nil {
		fmt.Println(err)
		return
	}
	for _, acc := range wltMgr.ListAccounts() {
		fmt.Println("Managed Address:", acc.Addr(), "Public Key Hash:", hex.EncodeToString(acc.PubKeyHash()))
	}
}

func dumpPrivKeyCmdFunc(cmd *cobra.Command, args []string) {
	fmt.Println("dumprivkey called")
	if len(args) < 0 {
		fmt.Println("address needed")
		return
	}
	addr := args[0]
	wltMgr, err := wallet.NewWalletManager(walletDir)
	if err != nil {
		fmt.Println(err)
		return
	}
	passphrase, err := wallet.ReadPassphraseStdin()
	if err != nil {
		fmt.Println(err)
		return
	}
	privateKey, err := wltMgr.DumpPrivKey(addr, passphrase)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Printf("Address: %s\nPrivate Key: %s", addr, privateKey)
}

func listTransactionsCmdFunc(cmd *cobra.Command, args []string) {
	fmt.Println("listtransactions called")
	if len(args) > 0 {
		client.ListTransactions(viper.GetViper(), args[0])
	}
}
