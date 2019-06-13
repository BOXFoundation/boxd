// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package ctl

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strconv"
	"time"

	"github.com/BOXFoundation/boxd/commands/box/root"
	"github.com/BOXFoundation/boxd/config"
	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/crypto"
	"github.com/BOXFoundation/boxd/rpc/pb"
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
	Use:   "vm [command]",
	Short: "The vm command line interface",
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
			Use:   "docall [from] [to] [data] [height] [timeout]",
			Short: "Call contract.",
			Run:   docall,
		},
		&cobra.Command{
			Use:   "deploycontract [from] [amount] [gasprice] [gaslimit] [nonce] [data]",
			Short: "Deploy a contract",
			Long: `On each invocation, "nonce" must be incremented by one. 
The return value is a hex-encoded transaction sequence and a contract address.`,
			Run: deploycontract,
		},
		&cobra.Command{
			Use:   "send [from] [contractaddress] [amount] [gasprice] [gaslimit] [nonce] [data]",
			Short: "Calling a contract",
			Long: `On each invocation, "nonce" must be incremented by one.
Successful call will return a transaction hash value`,
			Run: callcontract,
		},
	)
}

func importAbi(cmd *cobra.Command, args []string) {
	if len(args) < 2 {
		fmt.Println("Invalide argument number")
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
		fmt.Println("Invalide argument number")
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
	if len(args) != 6 {
		fmt.Println("Invalide argument number")
		return
	}
	from := args[0]
	//validate address
	if err := types.ValidateAddr(from); err != nil {
		fmt.Println("from address is Invalide: ", err)
		return
	}
	if len(args[5]) == 0 {
		fmt.Println("data error")
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
	data := args[5]
	req := &rpcpb.MakeContractTxReq{
		From:     from,
		Amount:   amount,
		GasPrice: gasprice,
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
	if len(args) != 7 {
		fmt.Println("Invalide argument number")
		return
	}
	from := args[0]
	if err := types.ValidateAddr(from); err != nil {
		fmt.Println("from address is Invalide: ", err)
		return
	}
	if len(args[6]) == 0 {
		fmt.Println("data error")
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
	data := args[6]
	req := &rpcpb.MakeContractTxReq{
		From:     from,
		To:       contractAddr,
		Amount:   amount,
		GasPrice: gasprice,
		GasLimit: gaslimit,
		Nonce:    nonce,
		Data:     data,
	}
	_, resp, err := signAndSendTx(req)
	if err != nil {
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
		fmt.Println("make contract transaction error: ", err)
		return "", nil, err
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
		fmt.Printf("account for %s not initialled\n", req.From)
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
	if err != nil {
		fmt.Println("send transaction failed: ", sendTxResp.Message)
		return "", nil, err
	}

	return resp.ContractAddr, sendTxResp, nil
}
