// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package tokencmd

import (
	"fmt"
	"path"
	"strconv"
	"strings"

	root "github.com/BOXFoundation/boxd/commands/box/root"
	"github.com/BOXFoundation/boxd/core"
	"github.com/BOXFoundation/boxd/core/types"
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
	if len(args) != 6 {
		fmt.Println("Invalid argument number")
		return
	}

	toAddr, err1 := types.NewAddress(args[1])
	tokenName := args[2]
	tokenSymbol := args[3]
	tokenTotalSupply, err2 := strconv.Atoi(args[4])
	tokenDecimals, err3 := strconv.Atoi(args[5])
	if err1 != nil && err2 != nil && err3 != nil {
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
	_, err = types.NewAddress(args[0])
	if err != nil {
		fmt.Println("Invalid address: ", args[0])
		return
	}
	conn := client.NewConnectionWithViper(viper.GetViper())
	defer conn.Close()

	tag := types.NewTokenTag(tokenName, tokenSymbol, uint8(tokenDecimals))
	tx, _, _, err := client.NewIssueTokenTx(account, toAddr.String(), tag,
		uint64(tokenTotalSupply), conn)
	if err != nil {
		fmt.Println(err)
		return
	}
	err = client.SendTransaction(conn, tx)
	if err != nil && !strings.Contains(err.Error(), core.ErrOrphanTransaction.Error()) {
		fmt.Println(err)
		return
	}

	hash, _ := tx.TxHash()
	tk := types.NewTokenFromOutpoint(types.OutPoint{
		Hash:  *hash,
		Index: 0,
	})
	fmt.Println("Created Token Address: ", tk.String())
}

func transferTokenCmdFunc(cmd *cobra.Command, args []string) {
	fmt.Println("transferToken called")
	if len(args) != 4 {
		fmt.Println("Invalid argument number")
		return
	}
	token := &types.Token{}
	if err := token.SetString(args[1]); err != nil {
		fmt.Println("Invalid token address")
		return
	}
	//targets, err := parseSendTarget(args[2:])
	//if err != nil {
	//	fmt.Println(err)
	//	return
	//}
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
	//fromAddr, err := types.NewAddress(args[0])
	//if err != nil {
	//	fmt.Println("Invalid address: ", args[0])
	//	return
	//}
	conn := client.NewConnectionWithViper(viper.GetViper())
	defer conn.Close()
	tx := new(types.Transaction)
	//tx, err := client.CreateTokenTransferTx(conn, fromAddr, targets,
	//	account.PublicKey(), token.OutPoint().Hash, token.OutPoint().Index, account)
	//if err != nil {
	//	fmt.Println(err)
	//}
	hash, _ := tx.TxHash()
	fmt.Println("Tx Hash:", hash.String())
	fmt.Println(util.PrettyPrint(tx))
}

func getTokenBalanceCmdFunc(cmd *cobra.Command, args []string) {
	fmt.Println("getTokenBalance called")
	if len(args) != 2 {
		fmt.Println("Invalid argument number")
		return
	}
	token := &types.Token{}
	if err := token.SetString(args[1]); err != nil {
		fmt.Println("Invalid token address")
		return
	}
	addrs := args[0:1]
	//addr, err := types.NewAddress(args[0])
	//if err != nil {
	//	fmt.Println("Invalid address: ", args[0])
	//	return
	//}
	conn := client.NewConnectionWithViper(viper.GetViper())
	defer conn.Close()
	tid := types.TokenID(token.OutPoint())
	balance, _ := client.GetTokenBalance(conn, addrs, &tid)
	fmt.Printf("Token balance of %s: %d\n", args[0], balance)
}

func parseSendTarget(args []string) (map[types.Address]uint64, error) {
	targets := make(map[types.Address]uint64)
	for i := 0; i < len(args)/2; i++ {
		addr, err := types.NewAddress(args[i*2])
		if err != nil {
			return targets, err
		}
		amount, err := strconv.Atoi(args[i*2+1])
		if err != nil {
			return targets, err
		}
		targets[addr] = uint64(amount)
	}
	return targets, nil
}
