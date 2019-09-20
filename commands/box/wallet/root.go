// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package walletcmd

import (
	"encoding/hex"
	"fmt"
	"os"
	"path"
	"time"

	"github.com/BOXFoundation/boxd/commands/box/common"
	"github.com/BOXFoundation/boxd/commands/box/root"
	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/crypto"
	rpcpb "github.com/BOXFoundation/boxd/rpc/pb"
	"github.com/BOXFoundation/boxd/rpc/rpcutil"
	"github.com/BOXFoundation/boxd/util"
	"github.com/BOXFoundation/boxd/wallet"
	"github.com/spf13/cobra"
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
			Run:   dumpwallet,
		},
		&cobra.Command{
			Use:   "encryptwallet [passphrase]",
			Short: "Encrypt a wallet with a passphrase",
			Run: func(cmd *cobra.Command, args []string) {
				fmt.Println("encryptwallet called")
			},
		},
		&cobra.Command{
			Use:   "getwalletinfo [address]",
			Short: "Get the basic informatio for a wallet",
			Run:   getwalletinfo,
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
			Use:   "decodeaddress [address]",
			Short: "decode address from base58 format to hash160",
			Run:   decodeAddress,
		},
		&cobra.Command{
			Use:   "getbalance [address]",
			Short: "Get the balance for any given address",
			Run:   getBalanceCmdFunc,
		},
		&cobra.Command{
			Use:   "signmessage [message] [optional publickey]",
			Short: "Sign a message with a publickey",
			Run:   signMessageCmdFunc,
		},
		&cobra.Command{
			Use:   "validateaddress [address]",
			Short: "Check if an address is valid",
			Run:   validateMessageCmdFunc,
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
	addr, err := wltMgr.NewAccount(passphrase)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Printf("Created new account. Address:%s\n", addr)
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
	addr, err := wltMgr.NewAccountWithPrivKey(privKey, passphrase)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Printf("Created new account. Address:%s\n", addr)
}

func listAccountCmdFunc(cmd *cobra.Command, args []string) {
	wltMgr, err := wallet.NewWalletManager(walletDir)
	if err != nil {
		fmt.Println(err)
		return
	}
	for _, acc := range wltMgr.ListAccounts() {
		fmt.Println("Managed Address:", acc.Addr(), "Public Key Hash:", hex.EncodeToString(acc.PubKeyHash()))
	}
}

func getwalletinfo(cmd *cobra.Command, args []string) {
	if len(args) < 1 {
		fmt.Println("address needed")
		return
	}
	addr := args[0]
	wltMgr, err := wallet.NewWalletManager(walletDir)
	if err != nil {
		fmt.Println(err)
		return
	}
	walletinfo, exist := wltMgr.GetAccount(addr)
	if exist {
		fmt.Println("path: ", walletinfo.Path)
		fmt.Println("Address: ", walletinfo.Addr())
		fmt.Printf("Pubkey Hash: %x \n", walletinfo.PubKeyHash())
		fmt.Println("UnLocked: ", walletinfo.Unlocked)
	}
}

func dumpPrivKeyCmdFunc(cmd *cobra.Command, args []string) {
	if len(args) < 1 {
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
	fmt.Printf("Address: %s\nPrivate Key: %s\n", addr, privateKey)
}

func dumpwallet(cmd *cobra.Command, args []string) {
	if len(args) < 1 {
		fmt.Println("file name needed")
		return
	}

	if _, err := os.Stat(args[0]); !os.IsNotExist(err) {
		fmt.Println("The file already exists and the command will overwrite the contents of the file.")

	}
	file, err := os.OpenFile(args[0], os.O_WRONLY|os.O_TRUNC|os.O_APPEND|os.O_CREATE, 0600)
	defer file.Close()
	if err != nil {
		fmt.Println(err)
		return
	}
	file.Write([]byte("# Wallet dump created by ContentBox\n" + "# * Created on " + time.Now().Format("2006-01-02 15:04:05") + "\n"))

	addrs := make([]string, 0)
	wltMgr, err := wallet.NewWalletManager(walletDir)
	if err != nil {
		fmt.Println(err)
		return
	}
	for _, acc := range wltMgr.ListAccounts() {
		addrs = append(addrs, acc.Addr())
	}
	success := 0

	for _, x := range addrs {
		addr := x
		wltMgr, err := wallet.NewWalletManager(walletDir)
		if err != nil {
			fmt.Println(err)
			return
		}
		fmt.Print("addr: " + addr)
		for i := 3; i >= 1; i-- {
			fmt.Printf(" You have %d remaining input opportunities\n", i)
			passphrase, err := wallet.ReadPassphraseStdin()
			if err != nil {
				fmt.Println(err)
				return
			}
			privateKey, err := wltMgr.DumpPrivKey(addr, passphrase)
			if err != nil {
				fmt.Print("error:", err)
				if i == 1 {
					fmt.Println("This wallet dump failed ")

				}
				continue
			}
			fmt.Printf("Address: %s dump successful \n", addr)
			file.Write([]byte("privateKey: " + privateKey + " " + time.Now().Format("2006-01-02 15:04:05") + " # addr=" + addr + "\n"))
			success++
			break

		}
		fmt.Println()
	}
	fmt.Printf("All wallets are dumped. %d successful %d failed\n", success, len(addrs)-success)
}

func decodeAddress(cmd *cobra.Command, args []string) {
	if len(args) != 1 && len(args[0]) == 0 {
		fmt.Println("invalid args")
		fmt.Println(cmd.UsageString())
		return
	}
	address, err := types.ParseAddress(args[0])
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Printf("address hash: %x\n", address.Hash160()[:])
}

func getBalanceCmdFunc(cmd *cobra.Command, args []string) {
	if len(args) != 1 {
		fmt.Println("Invalid argument number")
	}
	addrs := make([]string, 0)
	if len(args) < 1 {
		wltMgr, err := wallet.NewWalletManager(walletDir)
		if err != nil {
			fmt.Println(err)
			return
		}
		for _, acc := range wltMgr.ListAccounts() {
			addrs = append(addrs, acc.Addr())
		}
	} else {
		addrs = args
	}
	if err := types.ValidateAddr(addrs...); err != nil {
		fmt.Println("Verification address failed:", err)
		return
	}
	respRPC, err := rpcutil.RPCCall(rpcpb.NewTransactionCommandClient, "GetBalance",
		&rpcpb.GetBalanceReq{Addrs: addrs}, common.GetRPCAddr())
	if err != nil {
		fmt.Println(err)
		return
	}
	resp, ok := respRPC.(*rpcpb.GetBalanceResp)
	if !ok {
		fmt.Println("Conversion to rpcpb.GetBalanceResp failed")
		return
	}
	if resp.Code != 0 {
		fmt.Println(resp.Message)
		return
	}
	for i, b := range resp.Balances {
		fmt.Printf("%s: %d\n", addrs[i], b)
	}
}

func signMessageCmdFunc(cmd *cobra.Command, args []string) {
	if len(args) < 2 {
		fmt.Println("Please input the hex format of signature and publickey hash")
		return
	}
	msg, err := hex.DecodeString(args[0])
	if err != nil {
		fmt.Println(err)
		return
	}
	wltMgr, err := wallet.NewWalletManager(walletDir)
	if err != nil {
		fmt.Println(err)
		return
	}
	if _, exists := wltMgr.GetAccount(args[1]); !exists {
		fmt.Println(args[1], " is not a managed account")
		return
	}
	passphrase, err := wallet.ReadPassphraseStdin()
	if err != nil {
		fmt.Println(err)
		return
	}
	sig, err := wltMgr.Sign(msg, args[1], passphrase)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println("Signature:", hex.EncodeToString(sig))
}

func validateMessageCmdFunc(cmd *cobra.Command, args []string) {
	if len(args) < 1 {
		fmt.Println("Please specify the address to validate")
		return
	}
	addr := new(types.AddressPubKeyHash)
	if err := addr.SetString(args[0]); err != nil {
		fmt.Println(err)
	} else {
		fmt.Println(args[0], " is a valid address")
	}
}
