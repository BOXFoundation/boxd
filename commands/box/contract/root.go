// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package ctl

import (
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strconv"
	"strings"

	"github.com/BOXFoundation/boxd/commands/box/root"
	"github.com/BOXFoundation/boxd/config"
	"github.com/BOXFoundation/boxd/core"
	"github.com/BOXFoundation/boxd/core/abi"
	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/rpc/rpcutil"
	"github.com/BOXFoundation/boxd/util"
	"github.com/BOXFoundation/boxd/wallet"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	walletDir string

	defaultWalletDir = path.Join(util.HomeDir(), ".box_keystore")
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "contract [command]",
	Short: "The contract command line interface",
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
			Use:   "call [from] [to] [data] [height] [timeout]",
			Short: "Call contract.",
			Run:   docall,
		},
		&cobra.Command{
			Use:   "deploy [from] [amount] [gasprice] [gaslimit] [nonce] [data]",
			Short: "Deploy a contract",
			Long: `On each invocation, "nonce" must be incremented by one. 
The return value is a hex-encoded transaction sequence and a contract address.`,
			Run: deploy,
		},
		&cobra.Command{
			Use:   "send [from] [contractaddress] [amount] [gasprice] [gaslimit] [nonce] [data]",
			Short: "Calling a contract",
			Long: `On each invocation, "nonce" must be incremented by one.
Successful call will return a transaction hash value`,
			Run: send,
		},
		&cobra.Command{
			Use:   "encode [contractaddress] [methodname] [args...]",
			Short: "Get an input string to send or call",
			Run:   encode,
		},
	)
}

func encode(cmd *cobra.Command, args []string) {
	if len(args) < 2 {
		fmt.Println("Invalide argument number")
		return
	}
	abifile := args[0] + ".abi"
	methodname := args[1]

	if _, err := os.Stat(abifile); os.IsNotExist(err) {
		fmt.Println("Please import abi at first")
	}
	fileContent, err := ioutil.ReadFile(abifile)

	aabi := abi.ABI{}
	err = aabi.UnmarshalJSON(fileContent)
	if err != nil {
		fmt.Println("Invalid abi format")
	}

	var data []byte
	if len(args) == 2 {
		data, err = aabi.Pack(methodname)
	} else {
		params := []interface{}{}
		for _, arg := range args[2:] {
			params = append(params, arg)
		}
		data, err = aabi.Pack(methodname, params...)
	}

	if err != nil {
		fmt.Println(err)
	} else {
		fmt.Println(hex.EncodeToString(data))
	}
}

func importAbi(cmd *cobra.Command, args []string) {
	if len(args) < 2 {
		fmt.Println("Invalid argument number")
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
		fmt.Println("Invalid argument number")
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

func deploy(command *cobra.Command, args []string) {
	if len(args) != 6 {
		fmt.Println("Invalid argument number")
		return
	}
	from := args[0]
	if err := types.ValidateAddr(from); err != nil {
		fmt.Println("from address is Invalid: ", err)
		return
	}
	wltMgr, err := wallet.NewWalletManager(walletDir)
	if err != nil {
		fmt.Println(err)
		return
	}
	account, exists := wltMgr.GetAccount(from)
	if !exists {
		fmt.Printf("Account %s not managed\n", from)
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
	amount, err := strconv.ParseUint(args[1], 10, 64)
	if err != nil {
		fmt.Println("amount type conversion error: ", err)
		return
	}
	gasprice, err := strconv.ParseUint(args[2], 10, 64)
	if err != nil {
		fmt.Println("gasprice type conversion error: ", err)
		return
	}
	gaslimit, err := strconv.ParseUint(args[3], 10, 64)
	if err != nil {
		fmt.Println("gaslimit type conversion error: ", err)
		return
	}
	nonce, err := strconv.ParseUint(args[4], 10, 64)
	if err != nil {
		fmt.Println("nonce type conversion error: ", err)
		return
	}
	codeBytes, err := hex.DecodeString(args[5])
	if err != nil {
		fmt.Println("decode contract code error: ", err)
		return
	}
	//get gRPC connection
	conn, err := rpcutil.GetGRPCConn(getRPCAddr())
	if err != nil {
		fmt.Println(err)
		return
	}
	defer conn.Close()
	//deploy
	tx, contractAddr, err := rpcutil.NewContractDeployTx(account, amount, gasprice, gaslimit, nonce, codeBytes, conn)
	if err != nil {
		fmt.Println("deploy contract error: ", err)
		return
	}
	hash, err := rpcutil.SendTransaction(conn, tx)
	if err != nil && !strings.Contains(err.Error(), core.ErrOrphanTransaction.Error()) {
		fmt.Println("send transaction error: ", err)
		return
	}
	fmt.Println("Tx Hash: ", hash)
	fmt.Println("Contract Address: ", contractAddr)
}

func send(cmd *cobra.Command, args []string) {
	if len(args) != 7 {
		fmt.Println("Invalide argument number")
		return
	}
	from := args[0]
	if err := types.ValidateAddr(from); err != nil {
		fmt.Println("from address is Invalid: ", err)
		return
	}
	wltMgr, err := wallet.NewWalletManager(walletDir)
	if err != nil {
		fmt.Println(err)
		return
	}
	account, exists := wltMgr.GetAccount(from)
	if !exists {
		fmt.Printf("Account %s not managed\n", from)
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
	contractAddr := args[1]
	amount, err := strconv.ParseUint(args[2], 10, 64)
	if err != nil {
		fmt.Println("get amount error: ", err)
		return
	}
	gasprice, err := strconv.ParseUint(args[3], 10, 64)
	if err != nil {
		fmt.Println("get gasprice error: ", err)
		return
	}
	gaslimit, err := strconv.ParseUint(args[4], 10, 64)
	if err != nil {
		fmt.Println("get getlimit error: ", err)
		return
	}
	nonce, err := strconv.ParseUint(args[5], 10, 64)
	if err != nil {
		fmt.Println("get nonce error: ", err)
		return
	}
	codeBytes, err := hex.DecodeString(args[6])
	if err != nil {
		fmt.Println("conversion byte error: ", err)
		return
	}
	//get gRPC connection
	conn, err := rpcutil.GetGRPCConn(getRPCAddr())
	if err != nil {
		fmt.Println(err)
		return
	}
	defer conn.Close()

	tx, err := rpcutil.NewContractCallTx(account, contractAddr, amount, gasprice, gaslimit, nonce, codeBytes, conn)
	if err != nil {
		fmt.Println("call contract error: ", err)
		return
	}
	hash, err := rpcutil.SendTransaction(conn, tx)
	if err != nil && !strings.Contains(err.Error(), core.ErrOrphanTransaction.Error()) {
		fmt.Println("send transaction error: ", err)
		return
	}
	fmt.Println("Tx Hash: ", hash)
}

func getRPCAddr() string {
	var cfg config.Config
	viper.Unmarshal(&cfg)
	return fmt.Sprintf("%s:%d", cfg.RPC.Address, cfg.RPC.Port)
}
