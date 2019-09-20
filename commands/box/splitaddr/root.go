// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package splitaddrcmd

import (
	"fmt"
	"math"
	"path"
	"strconv"

	"github.com/BOXFoundation/boxd/commands/box/common"
	root "github.com/BOXFoundation/boxd/commands/box/root"
	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/rpc/rpcutil"
	"github.com/BOXFoundation/boxd/util"
	format "github.com/BOXFoundation/boxd/util/format"
	"github.com/BOXFoundation/boxd/wallet"
	"github.com/spf13/cobra"
)

const (
	opReturnAmount = 0
)

var cfgFile string
var walletDir string
var defaultWalletDir = path.Join(util.HomeDir(), ".box_keystore")

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "splitaddr",
	Short: "Split address subcommand",
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
			Use:   "create fromaddr [(addr1, weight1), (addr2, weight2), (addr3, weight3), ...]",
			Short: "Create a split address from multiple addresses and their weights: address order matters",
			Run:   createCmdFunc,
		},
	)
}

func createCmdFunc(cmd *cobra.Command, args []string) {
	if len(args) < 3 || len(args)%2 == 0 {
		fmt.Println("Invalid argument number: expect odd number larger than or equal to 3")
		return
	}
	// account
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
	// addrs and weights
	addrs, weights := make([]string, 0), make([]uint32, 0)
	for i := 1; i < len(args)-1; i += 2 {
		addrs = append(addrs, args[i])
		a, err := strconv.ParseUint(args[i+1], 10, 64)
		if err != nil || a >= math.MaxUint32 {
			fmt.Printf("Invalid amount %s\n", args[i+1])
			return
		}
		weights = append(weights, uint32(a))
	}
	if err := types.ValidateAddr(addrs...); err != nil {
		fmt.Println(err)
		return
	}
	addrHashes := make([]*types.AddressHash, 0, len(addrs))
	for _, addr := range addrs {
		address, _ := types.ParseAddress(addr)
		addrHashes = append(addrHashes, address.Hash160())
	}
	// conn
	conn, err := rpcutil.GetGRPCConn(common.GetRPCAddr())
	if err != nil {
		fmt.Println("Get Rpc conn failed:", err)
		return
	}
	defer conn.Close()
	// send tx
	tx, _, err := rpcutil.NewSplitAddrTx(account, addrHashes, weights, conn)
	if err != nil {
		fmt.Println("NewSplitAddrTx call failed:", err)
		return
	}
	hashStr, err := rpcutil.SendTransaction(conn, tx)
	if err != nil {
		fmt.Println("send transaction failed:", err)
		return
	}
	fmt.Println("Tx Hash: ", hashStr)
	fmt.Println(format.PrettyPrint(tx))
}
