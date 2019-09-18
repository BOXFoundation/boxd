// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package ctl

import (
	"fmt"
	"path"
	"strconv"

	"github.com/BOXFoundation/boxd/commands/box/root"
	"github.com/BOXFoundation/boxd/config"
	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/p2p"
	rpcpb "github.com/BOXFoundation/boxd/rpc/pb"
	"github.com/BOXFoundation/boxd/rpc/rpcutil"
	"github.com/BOXFoundation/boxd/util"
	format "github.com/BOXFoundation/boxd/util/format"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
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

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "get version of boxd",
	Run:   versionCmdFunc,
}

func init() {
	root.RootCmd.AddCommand(versionCmd)
	//
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
			Use:   "getblock [hash]",
			Short: "Get the block with a specific hash",
			Run:   getBlockCmdFunc,
		},
		&cobra.Command{
			Use:   "getblockbyheight [height]",
			Short: "Get the block with a specific height",
			Run:   getBLockByHeight,
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
			Use:   "viewblockdetail [blockhash]",
			Short: "Get the raw blockInformation for a block hash",
			Run:   getBlockDetailCmdFunc,
		},
		&cobra.Command{
			Use:   "getinfo",
			Short: "Get info about the local node",
			Run:   getInfoCmdFunc,
		},
		&cobra.Command{
			Use:   "peerid",
			Short: "get peerid at p2p",
			Run:   getPeerIDCmdFunc,
		},
		&cobra.Command{
			Use:   "getnetworkid ",
			Short: "Get the basic info and performance metrics of a network",
			Run:   getNetWorkID,
		},
		&cobra.Command{
			Use:   "searchrawtxs [address]",
			Short: "Search transactions for a given address",
			Run: func(cmd *cobra.Command, args []string) {
				fmt.Println("searchrawtx called")
			},
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
		fmt.Printf("Block info of hash %s is\n%s\n", hash, format.PrettyPrint(block))
	}
}

func getBLockByHeight(cmd *cobra.Command, args []string) {
	fmt.Println("getblockbyheight called")
	if len(args) == 0 {
		fmt.Println("Parameter block height required")
		return
	}
	height, err := strconv.ParseUint(args[0], 10, 32)
	if err != nil {
		fmt.Println("getblockbyheight failed: ", err)
		return
	}
	respRPC, err := rpcutil.RPCCall(rpcpb.NewContorlCommandClient, "GetBlockByHeight",
		&rpcpb.GetBlockByHeightReq{Height: uint32(height)}, getRPCAddr())
	if err != nil {
		fmt.Println(err)
		return
	}
	resp := respRPC.(*rpcpb.GetBlockResponse)
	block := &types.Block{}
	err = block.FromProtoMessage(resp.Block)
	if err != nil {
		fmt.Println("the format of block conversion failed: ", err)
	}
	fmt.Printf("Block Information \n%s\n", format.PrettyPrint(block))
}

func getBlockCountCmdFunc(cmd *cobra.Command, args []string) {
	fmt.Println("getblockcount called")
	respRPC, err := rpcutil.RPCCall(rpcpb.NewContorlCommandClient, "GetCurrentBlockHeight",
		&rpcpb.GetCurrentBlockHeightRequest{}, getRPCAddr())
	if err != nil {
		fmt.Println(err)
		return
	}
	resp := respRPC.(*rpcpb.GetCurrentBlockHeightResponse)
	fmt.Println("Current Block Height:", resp.Height)
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
		fmt.Printf("Block Header of hash %s is\n%s\n", hash, format.PrettyPrint(header))
	}
}

func getBlockDetailCmdFunc(cmd *cobra.Command, args []string) {
	fmt.Println("getBlockDetail called")
	if len(args) < 1 {
		fmt.Println("Param txhash required")
		return
	}
	hash := args[0]
	conn, err := rpcutil.GetGRPCConn(getRPCAddr())
	if err != nil {
		fmt.Println(err)
		return
	}
	defer conn.Close()
	//
	block, err := rpcutil.GetBlock(conn, hash)
	if err != nil {
		fmt.Println(err)
	} else {
		fmt.Println(format.PrettyPrint(block))
	}

}

func versionCmdFunc(cmd *cobra.Command, args []string) {
	if len(args) != 0 {
		fmt.Println("parameters are not needed")
		return
	}
	fmt.Printf("boxd ver %s %s(%s) %s\n", config.Version, config.GitCommit, config.GitBranch, config.GoVersion)
}
func getInfoCmdFunc(cmd *cobra.Command, args []string) {
	if len(args) != 0 {
		fmt.Println("Invalid argument number")
		return
	}
	respRPC, err := rpcutil.RPCCall(rpcpb.NewContorlCommandClient, "GetNodeInfo",
		&rpcpb.GetNodeInfoRequest{}, getRPCAddr())
	if err != nil {
		fmt.Println(err)
		return
	}
	resp := respRPC.(*rpcpb.GetNodeInfoResponse)
	if resp.Code != 0 {
		fmt.Println(resp.Message)
		return
	}
	fmt.Println(format.PrettyPrint(resp))
}
func getPeerIDCmdFunc(cmd *cobra.Command, args []string) {
	if len(args) != 0 {
		fmt.Println("Invalid argument number")
		return
	}
	respRPC, err := rpcutil.RPCCall(rpcpb.NewWebApiClient, "PeerID",
		&rpcpb.PeerIDReq{}, getRPCAddr())
	if err != nil {
		fmt.Println(err)
		return
	}
	resp := respRPC.(*rpcpb.PeerIDResp)
	if resp.Code != 0 {
		fmt.Println(resp.Message)
		return
	}
	fmt.Println("Peer ID: ", format.PrettyPrint(resp.Peerid))
}

func getNetWorkID(cmd *cobra.Command, args []string) {
	if len(args) != 0 {
		fmt.Println("Invalid argument number ")
		return
	}
	respRPC, err := rpcutil.RPCCall(rpcpb.NewContorlCommandClient, "GetNetworkID",
		&rpcpb.GetNetworkIDRequest{}, getRPCAddr())
	if err != nil {
		fmt.Println(err)
		return
	}
	resp := respRPC.(*rpcpb.GetNetworkIDResponse)
	if resp.Code != 0 {
		fmt.Println(resp.Message)
		return
	}
	fmt.Printf("network id: %d\nnetwork literal: %s\n", resp.Id, resp.Literal)
}

func getRPCAddr() string {
	var cfg config.Config
	viper.Unmarshal(&cfg)
	return fmt.Sprintf("%s:%d", cfg.RPC.Address, cfg.RPC.Port)
}
