// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package transactioncmd

import (
	"encoding/hex"
	"fmt"
	"path"
	"strconv"
	"strings"

	"github.com/BOXFoundation/boxd/commands/box/common"
	"github.com/BOXFoundation/boxd/commands/box/root"
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
			Use:   "sendrawtx [rawtx]",
			Short: "Send a raw transaction to the network",
			Run:   sendrawtx,
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
			Use:   "gettxcountbyhash [blockhash]",
			Short: "Get the transaction amount by hash",
			Run:   getTxCountByHash,
		},
		&cobra.Command{
			Use:   "gettxcountbyheight [blockheight]",
			Short: "Get the transaction amount by height",
			Run:   getTxCountByHeight,
		},
		&cobra.Command{
			Use:   "gettxbyhash [blockhash] [tx_index]",
			Short: "Get the transaction by block_hash and block_index",
			Run:   getTxByHash,
		},
		&cobra.Command{
			Use:   "gettxbyheight [blockheight] [tx_index]",
			Short: "Get the transaction  by height and tx_index",
			Run:   getTxByHeight,
		},
		&cobra.Command{
			Use:   "fetchutxo [addr] [amount] [token_hash] [token_index]",
			Short: "Fetch Utxos",
			Run:   fetchUtxo,
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
	respRPC, err := rpcutil.RPCCall(rpcpb.NewTransactionCommandClient, "MakeUnsignedTx", req, common.GetRPCAddr())
	if err != nil {
		fmt.Println(err)
		return
	}
	resp, ok := respRPC.(*rpcpb.MakeTxResp)
	if !ok {
		fmt.Println("Conversion to rpcpb.MakeTxResp failed")
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

func sendrawtx(cmd *cobra.Command, args []string) {
	if len(args) != 1 {
		fmt.Println("Can only enter one string for a transaction")
		return
	}
	conn, err := rpcutil.GetGRPCConn(common.GetRPCAddr())
	if err != nil {
		fmt.Println("Get RPC conn failed:", err)
		return
	}
	defer conn.Close()
	txByte, err := hex.DecodeString(args[0])
	if err != nil {
		fmt.Println("DecodeString failed:", err)
		return
	}
	tx := new(types.Transaction)
	if err := tx.Unmarshal(txByte); err != nil {
		fmt.Println("unmarshal error: ", err)
		return
	}
	resp, err := rpcutil.SendTransaction(conn, tx)
	if err != nil {
		fmt.Println("Send Transaction failed:", err)
		return
	}
	fmt.Println(format.PrettyPrint(resp))
}

func getRawTxCmdFunc(cmd *cobra.Command, args []string) {
	if len(args) < 1 {
		fmt.Println("Param txhash required")
		return
	}
	hash := crypto.HashType{}
	hash.SetString(args[0])
	conn, err := rpcutil.GetGRPCConn(common.GetRPCAddr())
	if err != nil {
		fmt.Println("Get RPC conn failed:", err)
		return
	}
	defer conn.Close()
	tx, err := rpcutil.GetRawTransaction(conn, hash.GetBytes())
	if err != nil {
		fmt.Println("Get raw transaction failed:", err)
		return
	}
	fmt.Println(format.PrettyPrint(tx))
}

func getTxCountByHash(cmd *cobra.Command, args []string) {
	if len(args) != 1 {
		fmt.Println("Invalid argument number")
		return
	}
	respRPC, err := rpcutil.RPCCall(rpcpb.NewContorlCommandClient, "GetBlockTransactionCountByHash",
		&rpcpb.GetBlockTransactionCountByHashReq{BlockHash: args[0]}, common.GetRPCAddr())
	if err != nil {
		fmt.Println(err)
		return
	}
	resp, ok := respRPC.(*rpcpb.GetBlockTxCountResp)
	if !ok {
		fmt.Println("Convertion to rpcpb.GetBlockTxCountResp failed")
		return
	}
	if resp.Code != 0 {
		fmt.Println(resp.Message)
		return
	}
	fmt.Println("transaction amount:", resp.Count)
}

func getTxCountByHeight(cmd *cobra.Command, args []string) {
	if len(args) != 1 {
		fmt.Println("Invalid argument number")
		return
	}
	height, err := strconv.ParseUint(args[0], 10, 64)
	if err != nil {
		fmt.Println("the type of height conversion failed:", err)
		return
	}
	respRPC, err := rpcutil.RPCCall(rpcpb.NewContorlCommandClient, "GetBlockTransactionCountByHeight",
		&rpcpb.GetBlockTransactionCountByHeightReq{Height: uint32(height)}, common.GetRPCAddr())
	if err != nil {
		fmt.Println("RPC call failed:", err)
		return
	}
	resp, ok := respRPC.(*rpcpb.GetBlockTxCountResp)
	if !ok {
		fmt.Println("Conversion to rpcpb.GetBlockTxCountResp failed")
		return
	}
	if resp.Code != 0 {
		fmt.Println(resp.Message)
		return
	}
	fmt.Println("transaction amount:", resp.Count)
}

func getTxByHash(cmd *cobra.Command, args []string) {
	if len(args) != 2 {
		fmt.Println("Invalid argument number")
		return
	}
	index, err := strconv.ParseUint(args[1], 10, 64)
	if err != nil {
		fmt.Println("the type of index conversion failed:", err)
		return
	}
	respRPC, err := rpcutil.RPCCall(rpcpb.NewContorlCommandClient, "GetTransactionByBlockHashAndIndexReq",
		&rpcpb.GetTransactionByBlockHashAndIndexReq{BlockHash: args[0], Index: uint32(index)}, common.GetRPCAddr())
	if err != nil {
		fmt.Println("RPC call failed:", err)
		return
	}
	resp, ok := respRPC.(*rpcpb.GetTxResp)
	if !ok {
		fmt.Println("Conversion to rpcpb.GetTxResp failed")
		return
	}
	if resp.Code != 0 {
		fmt.Println(resp.Message)
		return
	}
	tx := new(types.Transaction)
	err = tx.FromProtoMessage(resp.Tx)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println("Transaction:", format.PrettyPrint(tx))
}

func getTxByHeight(cmd *cobra.Command, args []string) {
	if len(args) != 2 {
		fmt.Println("Invalid argument number")
		return
	}
	height, err := strconv.ParseUint(args[0], 10, 64)
	if err != nil {
		fmt.Println("the type of height conversion failed:", err)
		return
	}
	index, err := strconv.ParseUint(args[1], 10, 64)
	if err != nil {
		fmt.Println("the type of index conversion failed:", err)
		return
	}
	respRPC, err := rpcutil.RPCCall(rpcpb.NewContorlCommandClient, "GetTransactionByBlockHeightAndIndex",
		&rpcpb.GetTransactionByBlockHeightAndIndexReq{Height: uint32(height), Index: uint32(index)}, common.GetRPCAddr())
	if err != nil {
		fmt.Println("RPC call failed:", err)
		return
	}
	resp, ok := respRPC.(*rpcpb.GetTxResp)
	if !ok {
		fmt.Println("Conversion to rpcpb.GetTxResp failed")
		return
	}
	tx := &types.Transaction{}
	err = tx.FromProtoMessage(resp.Tx)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println("Transaction:", format.PrettyPrint(tx))
}

func decoderawtx(cmd *cobra.Command, args []string) {
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
	if err := types.ValidateAddr(args[0]); err != nil {
		fmt.Println("Verification address failed", err)
		return
	}
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
	fmt.Println("rawtx:", strTx)
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
	conn, err := rpcutil.GetGRPCConn(common.GetRPCAddr())
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

func fetchUtxo(cmd *cobra.Command, args []string) {
	if err := types.ValidateAddr(args[0]); err != nil {
		fmt.Println("verification address failed:", err)
		return
	}
	conn, err := rpcutil.GetGRPCConn(common.GetRPCAddr())
	if err != nil {
		fmt.Println("Get RPC conn failed:", err)
	}
	defer conn.Close()
	amount, err := strconv.ParseUint(args[1], 10, 64)
	if err != nil {
		fmt.Println("the type of amount conversion failed:", err)
		return
	}
	index, err := strconv.ParseUint(args[3], 10, 64)
	if err != nil {
		fmt.Println("the type of index conversion failed:", err)
		return
	}
	req := &rpcpb.FetchUtxosReq{
		Addr:       args[0],
		Amount:     amount,
		TokenHash:  args[2],
		TokenIndex: uint32(index),
	}
	respRPC, err := rpcutil.RPCCall(rpcpb.NewTransactionCommandClient, "FetchUtxos", req, common.GetRPCAddr())
	if err != nil {
		fmt.Println("RPC call failed:", err)
		return
	}
	resp, ok := respRPC.(*rpcpb.FetchUtxosResp)
	if !ok {
		fmt.Println("Conversion to rpcpb.FetchUtxosResp failed")
		return
	}
	if resp.Code != 0 {
		fmt.Println(resp.Message)
		return
	}
	fmt.Println("utxos:", resp.Utxos)
}
