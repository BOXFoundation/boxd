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
	rpcpb "github.com/BOXFoundation/boxd/rpc/pb"
	"github.com/BOXFoundation/boxd/rpc/rpcutil"
	"github.com/BOXFoundation/boxd/script"
	"github.com/BOXFoundation/boxd/util"
	format "github.com/BOXFoundation/boxd/util/format"
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
			Use:   "createtx [from] [to1,to2...] [amounts1,amounts2...]",
			Short: "create an unsigned transaction,",
			Long:  "Returns a set of raw message for signing and a hex encoded transaction string",
			Run:   maketx,
		},
		&cobra.Command{
			Use:   "signrawtx [address] [rawtx]",
			Short: "Sign a transaction with privatekey and send it to the network",
			Run:   signrawtx,
		},
		&cobra.Command{
			Use:   "createrawtx [from] [txhash1,txhash2...] [vout1,vout2...] [to1,to2...] [amount1,amount2...]",
			Short: "Create a raw transaction",
			Run:   createRawTransaction,
		},
		&cobra.Command{
			Use:   "decoderawtx",
			Short: "decode a raw transaction",
			Long:  `Given a hex-encoded transaction string, return the details of the transaction`,
			Run:   decoderawtx,
		},
		&cobra.Command{
			Use:   "viewtxdetail [txhash]",
			Short: "Get the raw transaction for a transaction hash",
			Run:   getRawTxCmdFunc,
		},
		&cobra.Command{
			Use:   "viewblockdetail [blockhash]",
			Short: "Get the raw blockInformation for a block hash",
			Run:   getBlockDetailCmdFunc,
		},
		&cobra.Command{
			Use:   "sendrawtx [rawtx]",
			Short: "Send a raw transaction to the network",
			Run:   sendrawtx,
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
			Use:   "fetchutxos [addr] [amount] [token_hash] [token_index]",
			Short: "fetch utxos ",
			Run:   fetchUtxosCmdFunc,
		},
		&cobra.Command{
			Use:   "getrawtx [txhash]",
			Short: "Get the raw transaction for a txid",
			Run:   getRawTxCmdFunc,
		},
	)
}

func maketx(cmd *cobra.Command, args []string) {
	if len(args) != 3 {
		fmt.Println("Invalide argument number")
		return
	}

	from := args[0]
	toAddr := strings.Split(args[1], ",")

	// check address
	if err := types.ValidateAddr(append(toAddr, from)...); err != nil {
		fmt.Println(err)
		return
	}
	amountStr := strings.Split(args[2], ",")
	amounts := make([]uint64, 0, len(amountStr))
	for _, x := range amountStr {
		tmp, err := strconv.ParseUint(x, 10, 64)
		if err != nil {
			fmt.Println("Conversion failed: ", err)
			return
		}
		amounts = append(amounts, uint64(tmp))
	}

	req := &rpcpb.MakeTxReq{
		From:    from,
		To:      toAddr,
		Amounts: amounts,
	}
	conn, err := rpcutil.GetGRPCConn(getRPCAddr())
	if err != nil {
		fmt.Println(err)
		return
	}
	defer conn.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	client := rpcpb.NewTransactionCommandClient(conn)
	resp, err := client.MakeUnsignedTx(ctx, req)
	if err != nil {
		fmt.Println("make transaction failed: ", err)
		return
	}
	tx := resp.Tx

	//get raw messages
	rawMsgs := resp.RawMsgs
	if len(rawMsgs) != len(tx.Vin) {
		fmt.Println("Inconsistent parameter numbers")
		return
	}
	fmt.Println("transaction: ", format.PrettyPrint(tx))
	rawTxBytes, err := tx.Marshal()
	if err != nil {
		fmt.Println("transaction marshal failed: ", err)
		return
	}
	rawTx := hex.EncodeToString(rawTxBytes)
	fmt.Println("raw transaction: ", rawTx)
	rawMsgStr := make([]string, 0)
	for _, x := range rawMsgs {
		rawMsgStr = append(rawMsgStr, hex.EncodeToString(x))
	}
	fmt.Println("rawMsgs: ", format.PrettyPrint(rawMsgStr))
}

func createRawTransaction(cmd *cobra.Command, args []string) {
	if len(args) != 4 {
		fmt.Println("Invalid argument number")
		return
	}
	fmt.Println("createRawTx called")
	from := args[0]
	fromAddress, err := types.NewAddress(from)
	if err != nil {
		fmt.Println("Invalid from address")
		return
	}
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
	toHashes := make([]*types.AddressHash, 0, len(to))
	for _, addr := range to {
		address, err := types.ParseAddress(addr)
		if err != nil {
			fmt.Println("Invalid to address")
			return
		}
		toHashes = append(toHashes, address.Hash160())
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
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req := &rpcpb.GetCurrentBlockHeightRequest{}
	client := rpcpb.NewContorlCommandClient(conn)
	resp, err := client.GetCurrentBlockHeight(ctx, req)
	if err != nil {
		fmt.Println(err)
		return
	}
	height := resp.Height
	tx, err := rpcutil.CreateRawTransaction(fromAddress.Hash160(), toHashes,
		txHash, vout, amounts, height)
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
	fmt.Println(format.PrettyPrint(resp))
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
		return
	}
	fmt.Println(format.PrettyPrint(tx))
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

func decoderawtx(cmd *cobra.Command, args []string) {
	fmt.Println("decoderawtx called")
	if len(args) != 1 {
		fmt.Println("Invalid argument number")
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
	fmt.Println(format.PrettyPrint(tx))
}

func signrawtx(cmd *cobra.Command, args []string) {
	fmt.Println("signrawtx called")
	//arg[0] represents 'from address'
	//arg[1] represents 'rawtx'
	if len(args) < 2 {
		fmt.Println("Invalide argument number")
		return
	}
	txBytes, err := hex.DecodeString(args[1])
	if err != nil {
		fmt.Println(err)
		return
	}
	tx := new(types.Transaction)
	if err := tx.Unmarshal(txBytes); err != nil {
		fmt.Println("Unmarshal error: ", err)
		return
	}
	// get wallet manager
	wltMgr, err := wallet.NewWalletManager(walletDir)
	if err != nil {
		fmt.Println("get wallet manager error", err)
		return
	}
	accI, ok := wltMgr.GetAccount(args[0])
	if !ok {
		fmt.Printf("account for %s not initialled\n", args[0])
		return
	}
	passphrase, err := wallet.ReadPassphraseStdin()

	if err := accI.UnlockWithPassphrase(passphrase); err != nil {
		fmt.Println("Fail to unlock account", err)
		return
	}
	pbk := accI.PublicKey()
	//get raw messages
	msgs := make([][]byte, 0, len(tx.Vin))
	newTx := tx.Copy()
	for _, in := range newTx.Vin {
		in.ScriptSig = pbk
	}
	data, err := newTx.Marshal()
	if err != nil {
		return
	}
	msgs = append(msgs, data)
	rawMsgTest := msgs
	sigHashes := make([]*crypto.HashType, 0, len(rawMsgTest))
	for _, msg := range rawMsgTest {
		hash := crypto.DoubleHashH(msg)
		sigHashes = append(sigHashes, &hash)
	}

	scriptSigs := make([][]byte, 0)
	// get script sign
	for _, sigHash := range sigHashes {
		sig, err := accI.Sign(sigHash)
		if err != nil {
			fmt.Println("sign error: ", err)
			return
		}
		scriptSig := script.SignatureScript(sig, accI.PublicKey())
		scriptSigs = append(scriptSigs, *scriptSig)
	}
	//Assign the signature to the corresponding vin
	for i, sig := range scriptSigs {
		tx.Vin[i].ScriptSig = sig
	}
	fmt.Println(format.PrettyPrint(tx))
	mashalTx, err := tx.Marshal()
	if err != nil {
		fmt.Println(err)
		return
	}
	strTx := hex.EncodeToString(mashalTx)
	fmt.Println("rawtx: ", strTx)
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
	toHashes := make([]*types.AddressHash, 0, len(addrs))
	for _, addr := range addrs {
		address, err := types.ParseAddress(addr)
		if err != nil {
			fmt.Println("Invalid to address")
			return
		}
		toHashes = append(toHashes, address.Hash160())
	}
	conn, err := rpcutil.GetGRPCConn(getRPCAddr())
	if err != nil {
		fmt.Println(err)
	}
	defer conn.Close()
	tx, _, err := rpcutil.NewTx(account, toHashes, amounts, conn)
	if err != nil {
		fmt.Println("new tx error: ", err)
		return
	}
	hash, err := rpcutil.SendTransaction(conn, tx)
	if err != nil && !strings.Contains(err.Error(), core.ErrOrphanTransaction.Error()) {
		fmt.Println(err)
	}
	fmt.Println("Tx Hash: ", hash)
	fmt.Println(format.PrettyPrint(tx))
}

func fetchUtxosCmdFunc(cmd *cobra.Command, args []string) {

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
