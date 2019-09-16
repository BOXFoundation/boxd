// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package ctl

import (
	"context"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/BOXFoundation/boxd/commands/box/root"
	"github.com/BOXFoundation/boxd/config"
	"github.com/BOXFoundation/boxd/core/abi"
	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/crypto"
	rpcpb "github.com/BOXFoundation/boxd/rpc/pb"
	"github.com/BOXFoundation/boxd/rpc/rpcutil"
	"github.com/BOXFoundation/boxd/script"
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
			Use:   "importabi [contract_address] [abi_file_name]",
			Short: "Import abi for contract.",
			Run:   importAbi,
		},
		&cobra.Command{
			Use:   "call [from] [to] [data] [height] [timeout]",
			Short: "Call contract.",
			Run:   docall,
		},
		&cobra.Command{
			Use:   "deploy [from] [amount] [gaslimit] [nonce] [data]",
			Short: "Deploy a contract",
			Long: `On each invocation, "nonce" must be incremented by one. 
The return value is a hex-encoded transaction sequence and a contract address.`,
			Run: deploycontract,
		},
		&cobra.Command{
			Use:   "send [from] [contractaddress] [amount] [gaslimit] [nonce] [data]",
			Short: "Calling a contract",
			Long: `On each invocation, "nonce" must be incremented by one.
Successful call will return a transaction hash value`,
			Run: callcontract,
		},
		&cobra.Command{
			Use:   "encode [contractaddress] [methodname] [args...]",
			Short: "Get an input string to send or call",
			Run:   encode,
		},
		&cobra.Command{
			Use:   "getlogs [hash] [from] [to] [address] [topics]",
			Short: "Get returns logs matching the given argument that are stored within the state",
			Run:   getLogs,
		},
		&cobra.Command{
			Use:   "nonce [addr]",
			Short: "Get the nonce of address ",
			Run:   getNonce,
		},
		&cobra.Command{
			Use:   "getcode [contractaddress]",
			Short: "Get the code of contract_address",
			Run:   getCode,
		},
		&cobra.Command{
			Use:   "estimategas [from] [to] [data] [height] [timeout]",
			Short: "Get estimategas about contract_transaction",
			Run:   getEstimateGas,
		},
		&cobra.Command{
			Use:   "getstorage [address] [position] [height]",
			Short: "Get the position of variable in storsge",
			Run:   getStorageAt,
		},
	)
}

func encode(cmd *cobra.Command, args []string) {
	if len(args) < 2 {
		fmt.Println("Invalid argument number")
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
		// Try use event.
		if event, ok := aabi.Events[methodname]; ok {
			fmt.Println(hex.EncodeToString(event.ID().Bytes()))
			return
		}
		fmt.Println(err)
	} else {
		fmt.Println("encode information :", hex.EncodeToString(data))
	}
}

func importAbi(cmd *cobra.Command, args []string) {
	if len(args) < 2 {
		fmt.Println("Invalid argument number")
		return
	}
	src, err := os.Open(args[1])
	if err != nil {
		fmt.Println("Failed to open src file. ", args[1])
		return
	}
	defer src.Close()

	dstname := args[0] + ".abi"
	if _, err := os.Stat(dstname); !os.IsNotExist(err) {
		fmt.Println("The abi already exists and the command will overwrite it.")
	}
	dst, err := os.OpenFile(dstname, os.O_WRONLY|os.O_TRUNC|os.O_APPEND|os.O_CREATE, 0600)
	if err != nil {
		fmt.Println("Failed to open dst file. ", dstname)
		return
	}
	defer dst.Close()
	if _, err := io.Copy(dst, src); err != nil {
		fmt.Println("Failed to copy file. ", err)
	}
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

func deploycontract(cmd *cobra.Command, args []string) {
	if len(args) != 5 {
		fmt.Println("Invalid argument number")
		return
	}
	from := args[0]
	//validate address
	if err := types.ValidateAddr(from); err != nil {
		fmt.Println("From address is Invalid: ", err)
		return
	}
	if len(args[4]) == 0 {
		fmt.Println("data error")
		return
	}
	amount, err := strconv.ParseUint(args[1], 10, 64)
	if err != nil {
		fmt.Println("amount type conversion error: ", err)
		return
	}
	gaslimit, err := strconv.ParseUint(args[2], 10, 64)
	if err != nil {
		fmt.Println("gaslimit type conversion error: ", err)
		return
	}

	nonce, err := strconv.ParseUint(args[3], 10, 64)
	if err != nil {
		fmt.Println("nonce type conversion error: ", err)
		return
	}
	data := args[4]
	req := &rpcpb.MakeContractTxReq{
		From:     from,
		Amount:   amount,
		GasLimit: gaslimit,
		Nonce:    nonce,
		IsDeploy: true,
		Data:     data,
	}
	contractAddr, resp, err := signAndSendTx(req)
	if err != nil {
		fmt.Println("sign and send transaction error: ", err)
		return
	}
	if resp.Code != 0 {
		fmt.Println("send transaction failed: ", resp.Message)
		return
	}
	if resp.Hash == "" {
		fmt.Println("deploy contract failed . ")
		return
	}

	fmt.Println("successful deployment,transaction hash: ", resp.Hash)
	fmt.Println("contract address: ", contractAddr)
	//fmt.Println(util.PrettyPrint(resp))

}

func callcontract(cmd *cobra.Command, args []string) {
	if len(args) != 6 {
		fmt.Println("Invalid argument number")
		return
	}
	from := args[0]
	if err := types.ValidateAddr(from); err != nil {
		fmt.Println("From address is Invalid: ", err)
		return
	}
	if len(args[5]) == 0 {
		fmt.Println("Data error")
		return
	}
	contractAddr := args[1]
	amount, err := strconv.ParseUint(args[2], 10, 64)
	if err != nil {
		fmt.Println("get amount error: ", err)
		return
	}
	gaslimit, err := strconv.ParseUint(args[3], 10, 64)
	if err != nil {
		fmt.Println("get getlimit error: ", err)
		return
	}
	nonce, err := strconv.ParseUint(args[4], 10, 64)
	if err != nil {
		fmt.Println("get nonce error: ", err)
		return
	}
	data := args[5]
	req := &rpcpb.MakeContractTxReq{
		From:     from,
		To:       contractAddr,
		Amount:   amount,
		GasLimit: gaslimit,
		Nonce:    nonce,
		Data:     data,
	}
	_, resp, err := signAndSendTx(req)
	if err != nil || resp == nil {
		fmt.Println(err)
		return
	}
	if resp.Hash == "" {
		fmt.Println("Failed call，transaction hash value is empty", resp.Message)
		return
	}
	fmt.Println("Successful call,transaction hash: ", resp.Hash)
}

func getRPCAddr() string {
	var cfg config.Config
	viper.Unmarshal(&cfg)
	return fmt.Sprintf("%s:%d", cfg.RPC.Address, cfg.RPC.Port)
}

func signAndSendTx(req *rpcpb.MakeContractTxReq) (string, *rpcpb.SendTransactionResp, error) {
	conn, err := rpcutil.GetGRPCConn(getRPCAddr())
	if err != nil {
		fmt.Println("get gRPC connection error: ", err)
		return "", nil, err
	}
	defer conn.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	client := rpcpb.NewTransactionCommandClient(conn)
	wltMgr, err := wallet.NewWalletManager(walletDir)
	if err != nil {
		fmt.Println("new wallet manager error: ", err)
		return "", nil, err
	}
	resp, err := client.MakeUnsignedContractTx(ctx, req)
	if err != nil {
		return "", nil, err
	}
	if resp.Code != 0 {
		return "", nil, fmt.Errorf("make unsigned contract tx failed. Err: %v", resp.Message)
	}
	rawMsgs := resp.GetRawMsgs()
	sigHashes := make([]*crypto.HashType, 0, len(rawMsgs))
	for _, msg := range rawMsgs {
		hash := crypto.DoubleHashH(msg)
		sigHashes = append(sigHashes, &hash)
	}
	// sign msg
	accI, ok := wltMgr.GetAccount(req.From)
	if !ok {
		err = fmt.Errorf("account for %s not initialled", req.From)
		return "", nil, err
	}
	passphrase, err := wallet.ReadPassphraseStdin()
	if err := accI.UnlockWithPassphrase(passphrase); err != nil {
		fmt.Println("Fail to unlock account: ")
		return "", nil, err
	}
	var scriptSigs [][]byte
	for _, sigHash := range sigHashes {
		sig, err := accI.Sign(sigHash)
		if err != nil {
			fmt.Println("sign error: ")
			return "", nil, err
		}
		scriptSig := script.SignatureScript(sig, accI.PublicKey())
		scriptSigs = append(scriptSigs, *scriptSig)
	}
	// make tx with sig
	tx := resp.GetTx()
	for i, sig := range scriptSigs {
		tx.Vin[i].ScriptSig = sig
	}

	// send tx
	sendTransaction := &rpcpb.SendTransactionReq{Tx: tx}
	sendTxResp, err := client.SendTransaction(ctx, sendTransaction)
	if err != nil || sendTxResp.Code != 0 {
		return "", sendTxResp, err
	}

	return resp.ContractAddr, sendTxResp, nil
}

func getLogs(cmd *cobra.Command, args []string) {
	fmt.Println("getLogs called")
	//arg[0]represents block hash , arg[1] "from"andarg[2"to " represent log between from and to
	//arg[3]reprensents address arg[4]represents topics
	if len(args) != 5 {
		fmt.Println("Invalid argument number")
		return
	}
	hash := args[0]
	from, err := strconv.ParseUint(args[1], 10, 32)
	if err != nil {
		fmt.Println(err)
		return
	}
	to, err := strconv.ParseUint(args[2], 10, 32)
	if err != nil {
		fmt.Println(err)
		return
	}
	address := strings.Split(args[3], ",")
	topicsStr := strings.Split(args[4], ",")
	topics := []*rpcpb.LogsReqTopiclist{&rpcpb.LogsReqTopiclist{Topics: topicsStr}}

	conn, err := rpcutil.GetGRPCConn(getRPCAddr())
	if err != nil {
		fmt.Println(err)
		return
	}
	defer conn.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req := &rpcpb.LogsReq{
		Uid:       "",
		Hash:      hash,
		From:      uint32(from),
		To:        uint32(to),
		Addresses: address,
		Topics:    topics,
	}
	client := rpcpb.NewWebApiClient(conn)
	resp, err := client.GetLogs(ctx, req)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println("Successful, log information:", resp.Logs)
}

func getNonce(cmd *cobra.Command, args []string) {
	fmt.Println("get nonce called")
	if len(args) != 1 {
		fmt.Println("Invalid argument number")
		return
	}
	addr := args[0]
	//validate address
	if err := types.ValidateAddr(addr); err != nil {
		fmt.Println("From address is Invalid: ", err)
		return
	}
	conn, err := rpcutil.GetGRPCConn(getRPCAddr())
	if err != nil {
		fmt.Println(err)
		return
	}
	defer conn.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	client := rpcpb.NewWebApiClient(conn)
	req := &rpcpb.NonceReq{
		Addr: addr,
	}
	resp, err := client.Nonce(ctx, req)
	if err != nil {
		fmt.Println("RPC call failed ：", err)
		return
	}
	fmt.Println("Nonce : ", resp.Nonce)
}
func getCode(cmd *cobra.Command, args []string) {
	fmt.Println("get code called")
	if len(args) != 1 {
		fmt.Println("Invalid argument number")
		return
	}
	conn, err := rpcutil.GetGRPCConn(getRPCAddr())
	if err != nil {
		fmt.Println(err)
		return
	}
	defer conn.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	client := rpcpb.NewWebApiClient(conn)
	req := &rpcpb.GetCodeReq{
		Address: args[0],
	}
	resp, err := client.GetCode(ctx, req)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println("Code : ", resp.Data)
}

func getEstimateGas(cmd *cobra.Command, args []string) {
	fmt.Println("get estimate gas called")
	if len(args) != 5 {
		fmt.Println("Invalid argument number")
		return
	}
	conn, err := rpcutil.GetGRPCConn(getRPCAddr())
	if err != nil {
		fmt.Println(err)
		return
	}
	defer conn.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	client := rpcpb.NewWebApiClient(conn)
	height, err := strconv.ParseUint(args[3], 10, 64)
	if err != nil {
		fmt.Println(err)
		return
	}
	timeout, err := strconv.ParseUint(args[4], 10, 64)
	if err != nil {
		fmt.Println(err)
		return
	}
	req := &rpcpb.CallReq{
		From:    args[0],
		To:      args[1],
		Data:    args[2],
		Height:  uint32(height),
		Timeout: uint32(timeout),
	}
	resp, err := client.EstimateGas(ctx, req)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println("Estimate Gas : ", resp.Gas)
}

func getStorageAt(cmd *cobra.Command, args []string) {
	fmt.Println("get code called")
	if len(args) != 3 {
		fmt.Println("Invalid argument number")
		return
	}
	conn, err := rpcutil.GetGRPCConn(getRPCAddr())
	if err != nil {
		fmt.Println(err)
		return
	}
	defer conn.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	client := rpcpb.NewWebApiClient(conn)
	height, err := strconv.ParseUint(args[2], 10, 64)
	if err != nil {
		fmt.Println(err)
		return
	}
	req := &rpcpb.StorageReq{
		Address:  args[0],
		Position: args[1],
		Height:   uint32(height),
	}
	resp, err := client.GetStorageAt(ctx, req)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println("position : ", resp.Data)
}
