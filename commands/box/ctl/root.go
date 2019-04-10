// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package ctl

import (
	"encoding/hex"
	"fmt"
	"github.com/BOXFoundation/boxd/commands/box/root"
	"github.com/BOXFoundation/boxd/config"
	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/crypto"
	"github.com/BOXFoundation/boxd/p2p"
	"github.com/BOXFoundation/boxd/rpc/rpcutil"
	"github.com/BOXFoundation/boxd/util"
	"github.com/BOXFoundation/boxd/wallet"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"path"
	"strconv"
	"strings"
)

var (
	walletDir string

	defaultWalletDir = path.Join(util.HomeDir(), ".box_keystore")
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "ctl [command]",
	Short: "Client to interact with boxd",
	// Uncomment the following line if your bare application
	// has an action associated with it:
	//	Run: func(cmd *cobra.Command, args []string) { },
}

func init() {
	root.RootCmd.AddCommand(rootCmd)
	rootCmd.PersistentFlags().StringVar(&walletDir, "wallet_dir", defaultWalletDir, "Specify directory to search keystore files")
	rootCmd.AddCommand(
		&cobra.Command{
			Use:   "addnode [netaddr] add|remove",
			Short: "Add or remove a peer node",
			Run: func(cmd *cobra.Command, args []string) {
				fmt.Println("addnode called")
			},
		},
		&cobra.Command{
			Use:   "createrawtx",
			Short: "Create a raw transaction",
			Run: createRawTx,
		},
		&cobra.Command{
			Use:   "debuglevel [debug|info|warning|error|fatal]",
			Short: "Set the debug level of boxd",
			Run:   debugLevelCmdFunc,
		},
		&cobra.Command{
			Use:   "networkid [id]",
			Short: "Update networkid of boxd",
			Run:   updateNetworkID,
		},
		&cobra.Command{
			Use:   "decoderawtx",
			Short: "A brief description of your command",
			Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
			Run: func(cmd *cobra.Command, args []string) {
				fmt.Println("decoderawtx called")
			},
		},
		&cobra.Command{
			Use:   "getbalance [address]",
			Short: "Get the balance for any given address",
			Run:   getBalanceCmdFunc,
		},
		&cobra.Command{
			Use:   "getblock [hash]",
			Short: "Get the block with a specific hash",
			Run:   getBlockCmdFunc,
		},
		&cobra.Command{
			Use:   "getblockcount",
			Short: "Get the total block count",
			Run:   getBlockCountCmdFunc,
		},
		&cobra.Command{
			Use:   "getblockhash [height]",
			Short: "Get the hash of a block at a given index",
			Run:   getBlockHashCmdFunc,
		},
		&cobra.Command{
			Use:   "getblockheader [hash]",
			Short: "Get the block header for a hash",
			Run:   getBlockHeaderCmdFunc,
		},
		&cobra.Command{
			Use:   "getinfo",
			Short: "Get info about the local node",
			Run: func(cmd *cobra.Command, args []string) {
				fmt.Println("getinfo called")
			},
		},
		&cobra.Command{
			Use:   "getnetworkinfo [network]",
			Short: "Get the basic info and performance metrics of a network",
			Run: func(cmd *cobra.Command, args []string) {
				fmt.Println("getnetworkinfo called")
			},
		},
		&cobra.Command{
			Use:   "getrawtx [txhash]",
			Short: "Get the raw transaction for a txid",
			Run:   getRawTxCmdFunc,
		},
		&cobra.Command{
			Use:   "searchrawtxs [address]",
			Short: "Search transactions for a given address",
			Run: func(cmd *cobra.Command, args []string) {
				fmt.Println("searchrawtx called")
			},
		},
		&cobra.Command{
			Use:   "sendrawtx [rawtx]",
			Short: "Send a raw transaction to the network",
			Run: func(cmd *cobra.Command, args []string) {
				fmt.Println("sendrawtx called")
			},
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
		&cobra.Command{
			Use:   "verifychain",
			Short: "Verify the local chain",
			Run: func(cmd *cobra.Command, args []string) {
				fmt.Println("verifychain called")
			},
		},
		&cobra.Command{
			Use:   "verifymessage [message] [publickey]",
			Short: "Verify a message with a public key",
			Run: func(cmd *cobra.Command, args []string) {
				fmt.Println("verifymessage called")
			},
		},
	)
}

func debugLevelCmdFunc(cmd *cobra.Command, args []string) {
	level := "info"
	if len(args) > 0 {
		level = args[0]
	}
	conn, err := rpcutil.GetGRPCConn(getRPCAddr())
	if err != nil {
		fmt.Println(err)
		return
	}
	defer conn.Close()
	rpcutil.SetDebugLevel(conn, level)
	fmt.Println("success")
}

// NOTE: should be remove in product env
func updateNetworkID(cmd *cobra.Command, args []string) {
	id := p2p.Testnet // default is testnet
	if len(args) > 0 {
		n, err := strconv.Atoi(args[0])
		if err != nil {
			fmt.Println("args[0] is not a uint32 number")
			return
		}
		id = uint32(n)
	}
	conn, err := rpcutil.GetGRPCConn(getRPCAddr())
	if err != nil {
		fmt.Println(err)
		return
	}
	defer conn.Close()
	rpcutil.UpdateNetworkID(conn, id)
}

func getBalanceCmdFunc(cmd *cobra.Command, args []string) {
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
	conn, err := rpcutil.GetGRPCConn(getRPCAddr())
	if err != nil {
		fmt.Println(err)
		return
	}
	defer conn.Close()
	if err := types.ValidateAddr(addrs...); err != nil {
		fmt.Println(err)
		return
	}
	balances, err := rpcutil.GetBalance(conn, addrs)
	if err != nil {
		fmt.Println(err)
		return
	}
	for i, b := range balances {
		fmt.Printf("%s: %d\n", addrs[i], b)
	}
}

func getBlockCmdFunc(cmd *cobra.Command, args []string) {
	fmt.Println("getblock called")
	if len(args) == 0 {
		fmt.Println("Parameter block hash required")
		return
	}
	hash := args[0]
	conn, err := rpcutil.GetGRPCConn(getRPCAddr())
	if err != nil {
		fmt.Println(err)
		return
	}
	defer conn.Close()
	block, err := rpcutil.GetBlock(conn, hash)
	if err != nil {
		fmt.Println(err)
	} else {
		fmt.Printf("Block info of hash %s is\n%s\n", hash, util.PrettyPrint(block))
	}
}

func getBlockCountCmdFunc(cmd *cobra.Command, args []string) {
	fmt.Println("getblockcount called")
	conn, err := rpcutil.GetGRPCConn(getRPCAddr())
	if err != nil {
		fmt.Println(err)
		return
	}
	defer conn.Close()
	height, err := rpcutil.GetBlockCount(conn)
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println("Current Height: ", height)
}

func getBlockHashCmdFunc(cmd *cobra.Command, args []string) {
	fmt.Println("getblockhash called")
	if len(args) == 0 {
		fmt.Println("Parameter block height required")
		return
	}
	height64, err := strconv.ParseUint(args[0], 10, 32)
	if err != nil {
		fmt.Println(err)
		return
	}
	height := uint32(height64)
	conn, err := rpcutil.GetGRPCConn(getRPCAddr())
	if err != nil {
		fmt.Println(err)
		return
	}
	defer conn.Close()
	hash, err := rpcutil.GetBlockHash(conn, height)
	if err != nil {
		fmt.Println(err)
	} else {
		fmt.Printf("Block hash of height %d is %s\n", height, hash)
	}
}

func getBlockHeaderCmdFunc(cmd *cobra.Command, args []string) {
	fmt.Println("getblockheader called")
	if len(args) == 0 {
		fmt.Println("Parameter block hash required")
		return
	}
	hash := args[0]
	conn, err := rpcutil.GetGRPCConn(getRPCAddr())
	if err != nil {
		fmt.Println(err)
		return
	}
	defer conn.Close()
	header, err := rpcutil.GetBlockHeader(conn, hash)
	if err != nil {
		fmt.Println(err)
	} else {
		fmt.Printf("Block Header of hash %s is\n%s\n", hash, util.PrettyPrint(header))
	}
}

func getRawTxCmdFunc(cmd *cobra.Command, args []string) {
	fmt.Println("getrawtx called")
	if len(args) < 1 {
		fmt.Println("Param txhash required")
		return
	}
	hash := crypto.HashType{}
	hash.SetString(args[0])
	conn, err := rpcutil.GetGRPCConn(getRPCAddr())
	if err != nil {
		fmt.Println(err)
		return
	}
	defer conn.Close()
	tx, err := rpcutil.GetRawTransaction(conn, hash.GetBytes())
	if err != nil {
		fmt.Println(err)
	} else {
		fmt.Println(util.PrettyPrint(tx))
	}
}

func createRawTx(cmd *cobra.Command,args []string){
	if len(args)<4{
		fmt.Println("Invalide argument number")
		return
	}
	from:=args[0]
	txid_str:=strings.Split(args[1],",")
	vout_str:=strings.Split(args[2],",")
	to_str:=strings.Split(args[3],",")
	amount_str:=strings.Split(args[4],",")
	txid:=make([]crypto.HashType,0)
	vout:=make([]uint32,0)
	to:=make([]string,0)
	amount:=make([]uint64,0)
	for _,x:=range txid_str{
		tmp:=crypto.HashType{}
		tmp.SetString(x)
		fmt.Println(tmp)
		txid=append(txid,tmp)
	}
	for _,x:=range vout_str{
		tmp,_:=strconv.Atoi(x)
		vout=append(vout,uint32(tmp))
	}
	for _,x:=range to_str{
		to=append(to,x)
	}
	for _,x:=range amount_str{
		tmp,_:=strconv.Atoi(x)
		amount=append(amount,uint64(tmp))
	}
	if len(txid)!=len(vout){
		fmt.Println("Invalide argument number")
		return
	}
	if len(to)!=len(amount){
		fmt.Println("Invalide argument number")
		return
	}
	conn, err := rpcutil.GetGRPCConn(getRPCAddr())
	if err != nil {
		fmt.Println(err)
		return
	}
	defer conn.Close()
	height, err := rpcutil.GetBlockCount(conn)
	if err != nil {
		fmt.Println(err)
	}
	tx,_,err:=rpcutil.CreateRawTx(from,txid,vout,to,amount,height)
	fmt.Println(util.PrettyPrint(tx))

	}

func signMessageCmdFunc(cmd *cobra.Command, args []string) {
	fmt.Println("signmessage called")
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
	}
	fmt.Println("Signature: ", hex.EncodeToString(sig))
}

func validateMessageCmdFunc(cmd *cobra.Command, args []string) {
	if len(args) < 1 {
		fmt.Println("Please specify the address to validate")
		return
	}
	fmt.Println("validateing address", args[0])
	addr := &types.AddressPubKeyHash{}
	if err := addr.SetString(args[0]); err != nil {
		fmt.Println(err)
	} else {
		fmt.Println(args[0], " is a valid address")
	}
}

func getRPCAddr() string {
	var cfg config.Config
	viper.Unmarshal(&cfg)
	return fmt.Sprintf("%s:%d", cfg.RPC.Address, cfg.RPC.Port)
}
