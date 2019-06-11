// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package transactioncmd

import (
	"context"
	"encoding/hex"
	"fmt"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/BOXFoundation/boxd/commands/box/root"
	"github.com/BOXFoundation/boxd/config"
	"github.com/BOXFoundation/boxd/core"
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

var cfgFile string
var walletDir string
var defaultWalletDir = path.Join(util.HomeDir(), ".box_keystore")

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "tx",
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
			Use:   "sendfrom [fromaccount] [toaddress] [amount]",
			Short: "Send coins from an account to an address",
			Run:   sendFromCmdFunc,
		},
		&cobra.Command{
			Use:   "sendtoaddress [address]",
			Short: "Send coins to an address",
			Run: func(cmd *cobra.Command, args []string) {
				fmt.Println("sendtoaddress called")
			},
		},
		&cobra.Command{
			Use:   "signrawtx [rawtx]",
			Short: "Sign a transaction with privatekey and send it to the network",
			Run: func(cmd *cobra.Command, args []string) {
				fmt.Println("signrawtx called")
			},
		},
		&cobra.Command{
			Use:   "createrawtx [from] [txhash1,txhash2...] [vout1,vout2...] [to1,to2...] [amount1,amount2...]",
			Short: "Create a raw transaction",
			Run:   createRawTransaction,
		},
		&cobra.Command{
			Use:   "decoderawtx",
			Short: "A brief description of your command",
			Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra  application.`,
			Run: decoderawtx,
		},
		&cobra.Command{
			Use:   "getrawtx [txhash]",
			Short: "Get the raw transaction for a txid",
			Run:   getRawTxCmdFunc,
		},
		&cobra.Command{
			Use:   "sendrawtx [rawtx]",
			Short: "Send a raw transaction to the network",
			Run:   sendrawtx,
		},
		&cobra.Command{
			Use:   "deploycontract [from] [amount] [gasprice] [gaslimit] [nonce] [data]",
			Short: "Deploy a contract",
			Long: `On each invocation, "nonce" must be incremented by one. 
The return value is a hex-encoded transaction sequence and a contract address.`,
			Run: deploycontract,
		},
		&cobra.Command{
			Use:   "callcontract [from] [contractaddress] [amount] [gasprice] [gaslimit] [nonce] [data]",
			Short: "Calling a contract",
			Long: `On each invocation, "nonce" must be incremented by one.
Successful call will return a transaction hash value`,
			Run: callcontract,
		},
	)
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

func createRawTransaction(cmd *cobra.Command, args []string) {
	if len(args) < 4 {
		fmt.Println("Invalide argument number")
		return
	}
	fmt.Println("createRawTx called")
	from := args[0]
	//Cut characters around commas
	txHashStr := strings.Split(args[1], ",")
	txHash := make([]crypto.HashType, 0)
	for _, x := range txHashStr {
		tmp := crypto.HashType{}
		if err := tmp.SetString(x); err != nil {
			fmt.Println("set string error: ", err)
			return
		}
		txHash = append(txHash, tmp)
	}
	voutStr := strings.Split(args[2], ",")
	vout := make([]uint32, 0)
	for _, x := range voutStr {
		tmp, err := strconv.Atoi(x)
		if err != nil {
			fmt.Println("Type conversion failed: ", err)
			return
		}
		vout = append(vout, uint32(tmp))
	}
	toStr := strings.Split(args[3], ",")
	to := make([]string, 0)
	for _, x := range toStr {
		to = append(to, x)
	}
	amountStr := strings.Split(args[4], ",")
	amounts := make([]uint64, 0)
	for _, x := range amountStr {
		tmp, err := strconv.Atoi(x)
		if err != nil {
			fmt.Println("Type conversion failed: ", err)
			return
		}
		amounts = append(amounts, uint64(tmp))
	}
	if len(txHash) != len(vout) {
		fmt.Println(" The number of [txid] should be the same as the number of [vout]")
		return
	}
	if len(to) != len(amounts) {
		fmt.Println("The number of [to] should be the same as the number of [amount]")
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
		return
	}
	tx, err := rpcutil.CreateRawTransaction(from, txHash, vout, to, amounts, height)
	if err != nil {
		fmt.Println(err)
		return
	}
	mashalTx, err := tx.Marshal()
	if err != nil {
		fmt.Println(err)
		return
	}
	strTx := hex.EncodeToString(mashalTx)
	fmt.Println(strTx)
}

func sendrawtx(cmd *cobra.Command, args []string) {
	fmt.Println("sendrawtx called")
	if len(args) != 1 {
		fmt.Println("Can only enter one string for a transaction")
		return
	}
	conn, err := rpcutil.GetGRPCConn(getRPCAddr())
	if err != nil {
		fmt.Println(err)
		return
	}
	defer conn.Close()
	txByte, err := hex.DecodeString(args[0])
	if err != nil {
		fmt.Println(err)
		return
	}
	tx := new(types.Transaction)
	if err := tx.Unmarshal(txByte); err != nil {
		fmt.Println("unmarshal error: ", err)
		return
	}
	resp, err := rpcutil.SendTransaction(conn, tx)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(util.PrettyPrint(resp))
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

func decoderawtx(cmd *cobra.Command, args []string) {
	fmt.Println("decoderawtx called")
	if len(args) != 1 {
		fmt.Println("Invalide argument number")
		return
	}
	txByte, err := hex.DecodeString(args[0])
	if err != nil {
		fmt.Println(err)
		return
	}
	tx := new(types.Transaction)
	if err := tx.Unmarshal(txByte); err != nil {
		fmt.Println("Unmarshal error: ", err)
		return
	}
	fmt.Println(util.PrettyPrint(tx))
}

func sendFromCmdFunc(cmd *cobra.Command, args []string) {
	if len(args) < 3 {
		fmt.Println("Invalid argument number")
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
	// to address
	addrs, amounts := make([]string, 0), make([]uint64, 0)
	for i := 1; i < len(args)-1; i += 2 {
		addrs = append(addrs, args[i])
		a, err := strconv.ParseUint(args[i+1], 10, 64)
		if err != nil {
			fmt.Printf("Invalid amount %s", args[i+1])
			return
		}
		amounts = append(amounts, a)
	}
	if err := types.ValidateAddr(addrs...); err != nil {
		fmt.Println(err)
		return
	}
	//
	conn, err := rpcutil.GetGRPCConn(getRPCAddr())
	if err != nil {
		fmt.Println(err)
	}
	defer conn.Close()
	tx, _, _, err := rpcutil.NewTx(account, addrs, amounts, conn)
	if err != nil {
		fmt.Println("new tx error: ", err)
		return
	}
	hash, err := rpcutil.SendTransaction(conn, tx)
	if err != nil && !strings.Contains(err.Error(), core.ErrOrphanTransaction.Error()) {
		fmt.Println(err)
	}
	fmt.Println("Tx Hash:", hash)
	fmt.Println(util.PrettyPrint(tx))
}

func parseSendParams(args []string) (addrs []string, amounts []uint64, err error) {
	for i := 0; i < len(args)/2; i++ {
		_, err := types.NewAddress(args[i*2])
		if err != nil {
			return nil, nil, err
		}
		addrs = append(addrs, args[i*2])
		amount, err := strconv.Atoi(args[i*2+1])
		if err != nil {
			return nil, nil, err
		}
		amounts = append(amounts, uint64(amount))
	}
	return addrs, amounts, nil
}

func getRPCAddr() string {
	var cfg config.Config
	viper.Unmarshal(&cfg)
	return fmt.Sprintf("%s:%d", cfg.RPC.Address, cfg.RPC.Port)
}
