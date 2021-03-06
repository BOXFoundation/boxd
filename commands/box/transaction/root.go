// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package transactioncmd

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
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
	format "github.com/BOXFoundation/boxd/util/format"
	"github.com/BOXFoundation/boxd/wallet"
	"github.com/spf13/cobra"
)

var cfgFile string
var walletDir string

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "tx",
	Short: "A brief description of your application",
	// Uncomment the following line if your bare application
	// has an action associated with it:
	//	Run: func(cmd *cobra.Command, args []string) { },
	Example: `
	1.make unsigned transaction
	  ./box tx createtx b1fc1Vzz73WvBtzNQNbBSrxNCUC1Zrbnq4m b1dZ8aPQ2UPeYHfGV5hshyswbxRBZQjuz2L,b1gFAdBjy8gu3vFRbg1qJEXtiV9xZTVTKx1 10,20
	2.sign transaction 
	  ./box --wallet_dir keyfile tx signrawtx b1fc1Vzz73WvBtzNQNbBSrxNCUC1Zrbnq4m 12240a220a20206029513377e45fed5cf8486b76664864a8138d1cc5248c84202eab2c803b541a1d080a121976a9146aeacee33019c341b1700de7a0485ce13f3d02b988ac1a1d0814121976a914886d68724989517491e92c420a820a9e9d5b41d688ac1a2408d2f499ecb3aacf3a121976a914816666b318349468f8146e76e4e3751d937c14cb88ac
	3.send raw transaction
	  ./box tx sendrawtx 1290010a220a20206029513377e45fed5cf8486b76664864a8138d1cc5248c84202eab2c803b54126a473045022100dc42cfb8348bb10d91278069912351e7271c4e4744fd0f91de6451c6c227a6f602200ebf1c3fcfd47e609a539e59769b77e83690dc628bf246b9cfe49ca2154648bb2103ac5906f34b6f12150d49942dcd3df4b30716cb78abc9e3f6e488e2c1f28ab8bd1a1d080a121976a9146aeacee33019c341b1700de7a0485ce13f3d02b988ac1a1d0814121976a914886d68724989517491e92c420a820a9e9d5b41d688ac1a24089297cdecb3aacf3a121976a914816666b318349468f8146e76e4e3751d937c14cb88ac
	4.Sendfrom command sends an amount of box from a managed account to another account
	  ./box tx sendfrom b1YLUNwJD124sv9piRvkqcmfcujTZtHhHSz b1UJRzHvXoHA8DGGGGZaCSQBkVyoPEqmQUN 50000
	5.view the details of transaction
	  ./box tx detailtx a22fc0cc4754ee179071afe28f676526c00a6dc0ba963e6d7695f6cdcc91e045
	`,
}

// Init adds the sub command to the root command.
func init() {
	root.RootCmd.AddCommand(rootCmd)
	rootCmd.PersistentFlags().StringVar(&walletDir, "wallet_dir", common.DefaultWalletDir, "Specify directory to search keystore files")
	rootCmd.AddCommand(
		&cobra.Command{
			Use:   "sendfrom [fromaccount] [toaddress] [amount]",
			Short: "Send coins from an account to an address",
			Run:   sendFromCmdFunc,
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
			Use:   "sendrawtx [rawtx]",
			Short: "Send a raw transaction to the network",
			Run:   sendrawtx,
		},
		&cobra.Command{
			Use:   "getrawtx [txhash]",
			Short: "Get the raw transaction for a txid",
			Run:   getRawTxCmdFunc,
		},
		&cobra.Command{
			Use:   "gettxcount[optional|blockheight,blockhash]",
			Short: "Get the transaction amount",
			Run:   getTxCount,
		},
		&cobra.Command{
			Use:   "detailtx [tx_hash]",
			Short: "view the details of the transaction by tx_hash",
			Run:   detailTx,
		},
		&cobra.Command{
			Use:   "fetchutxo [addr] [amount] [token_hash] [token_index]",
			Short: "Fetch Utxos",
			Run:   fetchUtxo,
		},
		// &cobra.Command{
		// 	Use:   "sendtoaddress [address]",
		// 	Short: "Send coins to an address",
		// 	Run: func(cmd *cobra.Command, args []string) {
		// 		fmt.Println("sendtoaddress called")
		// 	},
		// },
	)
}

func maketx(cmd *cobra.Command, args []string) {
	if len(args) != 3 {
		fmt.Println(cmd.Use)
		return
	}
	from := args[0]
	toAddr := strings.Split(args[1], ",")
	// check address
	if _, err := types.NewAddress(from); err != nil {
		fmt.Println("invalid address")
		return
	}
	for _, addr := range toAddr {
		if err := types.ValidateAddr(addr); err != nil {
			fmt.Printf("invaild address for %s, err: %s\n", addr, err)
			return
		}
	}
	amountStr := strings.Split(args[2], ",")
	amounts := make([]uint64, 0, len(amountStr))
	for _, x := range amountStr {
		tmp, err := strconv.ParseUint(x, 10, 64)
		if err != nil {
			fmt.Printf("Conversion %s to unsigned numbers failed: %s\n", x, err)
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
	fmt.Println("transaction:", format.PrettyPrint(tx))
	rawTxBytes, err := tx.Marshal()
	if err != nil {
		fmt.Println("transaction marshal failed:", err)
		return
	}
	rawTx := hex.EncodeToString(rawTxBytes)
	fmt.Println("raw transaction:", rawTx)
	fmt.Println("rawMsgs:", rawMsgs)
}

func sendrawtx(cmd *cobra.Command, args []string) {
	if len(args) != 1 {
		fmt.Println(cmd.Use)
		return
	}
	conn, err := rpcutil.GetGRPCConn(common.GetRPCAddr())
	if err != nil {
		fmt.Println(err)
		return
	}
	defer conn.Close()
	txByte, err := hex.DecodeString(args[0])
	if err != nil {
		fmt.Printf("DecodeString %s failed:%v\n", args[0], err)
		return
	}
	tx := new(types.Transaction)
	if err := tx.Unmarshal(txByte); err != nil {
		fmt.Printf("tx unmarshal %s error: %v\n", txByte, err)
		return
	}
	hash, err := rpcutil.SendTransaction(conn, tx)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Printf("Tx %s has been sent to remote successfully\n", hash)
}

func getRawTxCmdFunc(cmd *cobra.Command, args []string) {
	if len(args) < 1 {
		fmt.Println(cmd.Use)
		return
	}
	hash := new(crypto.HashType)
	if err := hash.SetString(args[0]); err != nil {
		fmt.Println("invalid hash")
		return
	}
	respRPC, err := rpcutil.RPCCall(rpcpb.NewTransactionCommandClient, "GetRawTransaction",
		&rpcpb.GetRawTransactionRequest{Hash: hash.String()}, common.GetRPCAddr())
	if err != nil {
		fmt.Println(err)
		return
	}
	resp, ok := respRPC.(*rpcpb.GetRawTransactionResponse)
	if !ok {
		fmt.Println("Convertion to rpcpb.GetRawTransactionResponse failed")
		return
	}
	if resp.Code != 0 {
		fmt.Println(resp.Message)
		return
	}
	tx := new(types.Transaction)
	if err = tx.FromProtoMessage(resp.Tx); err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(format.PrettyPrint(tx))

}

func getTxCount(cmd *cobra.Command, args []string) {
	if len(args) == 0 {
		fmt.Println(cmd.Use)
		return
	}
	var (
		hash   = new(crypto.HashType)
		height uint64
	)
	if err := hash.SetString(args[0]); err != nil {
		height, err = strconv.ParseUint(args[0], 10, 32)
		if err != nil {
			fmt.Println("invalid argument:", args[0])
			return
		}
	}
	respRPC, err := rpcutil.RPCCall(rpcpb.NewContorlCommandClient, "GetTxCount", &rpcpb.GetTxCountReq{BlockHeight: uint32(height), BlockHash: hash.String()}, common.GetRPCAddr())
	if err != nil {
		fmt.Println(err)
		return
	}
	resp, ok := respRPC.(*rpcpb.GetTxCountResp)
	if !ok {
		fmt.Println("Conversion rpcpb.GetBlockResponse failed")
		return
	}
	if resp.Code != 0 {
		fmt.Println(resp.Message)
		return
	}
	fmt.Println(resp.Count)
}

func detailTx(cmd *cobra.Command, args []string) {
	if len(args) != 1 {
		fmt.Println(cmd.Use)
		return
	}
	hash := new(crypto.HashType)
	if err := hash.SetString(args[0]); err != nil {
		fmt.Println("invalid hash")
		return
	}
	respRPC, err := rpcutil.RPCCall(rpcpb.NewWebApiClient, "ViewTxDetail",
		&rpcpb.ViewTxDetailReq{Hash: hash.String()}, common.GetRPCAddr())
	if err != nil {
		fmt.Println(err)
		return
	}
	resp, ok := respRPC.(*rpcpb.ViewTxDetailResp)
	if !ok {
		fmt.Println("Convertion to rpcpb.GetRawTransactionResponse failed")
		return
	}
	if resp.Code != 0 {
		fmt.Println(resp.Message)
		return
	}
	txDetail, err := json.MarshalIndent(resp.Detail, "", "  ")
	if err != nil {
		fmt.Println("format the detail of tx from remote error:", err)
		return
	}
	fmt.Println(string(txDetail))
}

func decoderawtx(cmd *cobra.Command, args []string) {
	if len(args) != 1 {
		fmt.Println(cmd.Use)
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
		fmt.Println(cmd.Use)
		return
	}
	if _, err := types.NewAddress(args[0]); err != nil {
		fmt.Println("invalid address")
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
	fmt.Println("rawtx:", strTx)
}

func sendFromCmdFunc(cmd *cobra.Command, args []string) {
	if len(args) < 3 {
		fmt.Println(cmd.Use)
		return
	}
	if _, err := types.NewAddress(args[0]); err != nil {
		fmt.Println("invalid address")
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
		fmt.Println("new tx error:", err)
		return
	}
	hash, err := rpcutil.SendTransaction(conn, tx)
	if err != nil && !strings.Contains(err.Error(), core.ErrOrphanTransaction.Error()) {
		fmt.Println(err)
		return
	}
	fmt.Printf("Tx %s has been sent to remote successfully\n", hash)
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
	if len(args) != 2 && len(args) != 4 {
		fmt.Println(cmd.Use)
		return
	}
	if _, err := types.NewAddress(args[0]); err != nil {
		fmt.Println("invalid address")
		return
	}
	amount, err := strconv.ParseUint(args[1], 10, 64)
	if err != nil {
		fmt.Printf("Conversion %s to unsigned numbers failed: %s\n", args[1], err)
		return
	}
	var (
		tHashStr string
		tIdx     uint64
	)
	if len(args) == 4 {
		tHash := new(crypto.HashType)
		if err := tHash.SetString(args[2]); err != nil {
			fmt.Println("invalid token hash, error:", err)
			return
		}
		tHashStr = tHash.String()
		tIdx, err = strconv.ParseUint(args[3], 10, 64)
		if err != nil {
			fmt.Println("invalid token index, error:", err)
			return
		}
	}
	req := &rpcpb.FetchUtxosReq{
		Addr:       args[0],
		Amount:     amount,
		TokenHash:  tHashStr,
		TokenIndex: uint32(tIdx),
	}
	respRPC, err := rpcutil.RPCCall(rpcpb.NewTransactionCommandClient, "FetchUtxos", req, common.GetRPCAddr())
	if err != nil {
		fmt.Println(err)
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
	utxoBytes, _ := json.MarshalIndent(resp.Utxos, "", "  ")
	fmt.Println(string(utxoBytes))
}
