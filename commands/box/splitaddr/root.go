// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package splitaddrcmd

import (
	"encoding/hex"
	"fmt"
	"path"
	"strconv"
	"strings"

	root "github.com/BOXFoundation/boxd/commands/box/root"
	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/rpc/client"
	"github.com/BOXFoundation/boxd/script"
	"github.com/BOXFoundation/boxd/util"
	"github.com/BOXFoundation/boxd/wallet"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
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
	fmt.Println("create called")
	if len(args) < 3 || len(args)%2 == 0 {
		fmt.Println("Invalid argument number: expect odd number larger than or equal to 3")
		return
	}
	addrs, weights, err := parseAddrWeight(args[1:])
	if err != nil {
		fmt.Println(err)
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
	fromAddr, err := types.NewAddress(args[0])
	if err != nil {
		fmt.Println("Invalid address: ", args[0])
	}
	target := make(map[types.Address]uint64)
	target[fromAddr /* just a dummy value here */] = opReturnAmount

	conn := client.NewConnectionWithViper(viper.GetViper())
	defer conn.Close()
	tx, err := client.CreateSplitAddrTransaction(conn, fromAddr, account.PublicKey(), addrs, weights, account)
	if err != nil {
		fmt.Println(err)
	} else {
		hash, _ := tx.TxHash()
		fmt.Println("Tx Hash:", hash.String())
		fmt.Println(util.PrettyPrint(tx))

		splitAddr, err := getSplitAddr(tx.Vout[0].ScriptPubKey)
		if err != nil {
			fmt.Println(err)
			return
		}
		fmt.Printf("Split address generated for `%s`: %v\n", args[1:], splitAddr)
	}
}

func parseAddrWeight(args []string) ([]types.Address, []uint64, error) {
	addrs := make([]types.Address, 0)
	weights := make([]uint64, 0)
	for i := 0; i < len(args)/2; i++ {
		addr, err := types.NewAddress(args[i*2])
		if err != nil {
			return nil, nil, err
		}
		addrs = append(addrs, addr)

		weight, err := strconv.Atoi(args[i*2+1])
		if err != nil {
			return nil, nil, err
		}
		weights = append(weights, uint64(weight))
	}
	return addrs, weights, nil
}

// create a split address from arguments
func getSplitAddr(scriptPubKey []byte) (string, error) {
	// e.g., OP_RETURN aaeb7c5e48182fbd309a4e6a7e0de57e56f4cb16 ce86056786e3415530f8cc739fb414a87435b4b6 01 3ba03e454aed097836f2957a120f95ecf76a2771 04
	splitAddrScriptStr := script.NewScriptFromBytes(scriptPubKey).Disasm()
	s := strings.Split(splitAddrScriptStr, " ")
	// e.g., aaeb7c5e48182fbd309a4e6a7e0de57e56f4cb16
	pubKeyHash, err := hex.DecodeString(s[1])
	if err != nil {
		return "", err
	}
	addr, err := types.NewAddressPubKeyHash(pubKeyHash)
	if err != nil {
		return "", err
	}
	return addr.String(), nil
}
