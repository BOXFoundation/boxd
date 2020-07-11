// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package tokencmd

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/BOXFoundation/boxd/commands/box/common"
	"github.com/BOXFoundation/boxd/commands/box/root"
	"github.com/BOXFoundation/boxd/core"
	"github.com/BOXFoundation/boxd/core/txlogic"
	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/crypto"
	rpcpb "github.com/BOXFoundation/boxd/rpc/pb"
	"github.com/BOXFoundation/boxd/rpc/rpcutil"
	"github.com/BOXFoundation/boxd/wallet"
	"github.com/spf13/cobra"
)

var (
	cfgFile   string
	walletDir string
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "token",
	Short: "Token subcommand",
	// Uncomment the following line if your bare application
	// has an action associated with it:
	//	Run: func(cmd *cobra.Command, args []string) { },
}

// Init adds the sub command to the root command.
func init() {
	root.RootCmd.AddCommand(rootCmd)
	rootCmd.PersistentFlags().StringVar(&walletDir, "wallet_dir", common.DefaultWalletDir, "Specify directory to search keystore files")
	rootCmd.AddCommand(
		&cobra.Command{
			Use:   "issue [issuer] [owner] [name] [symbol] [supply] [decimal]",
			Short: "issue a new token",
			Run:   createTokenCmdFunc,
		},
		&cobra.Command{
			Use:   "transfer [from] [tokenID] [addr1,addr2...] [amount1,amount2 ...]",
			Short: "transfer tokens",
			Run:   transferTokenCmdFunc,
		},
		&cobra.Command{
			Use:   "getbalance [tokenID] [addr1, addr2 ...]",
			Short: "get token balance",
			Run:   getTokenBalanceCmdFunc,
		},
	)
}

func createTokenCmdFunc(cmd *cobra.Command, args []string) {
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
	if strings.ToUpper(tokenSymbol) == "BOX" {
		fmt.Println("The value of tokenSymbol cannot be:", tokenSymbol)
		return
	}
	if tokenDecimals < 0 || tokenDecimals > 8 {
		fmt.Println("tokenDecimals must be greater than zero and less than eight")
		return
	}
	if uint64(tokenTotalSupply) > math.MaxUint64/uint64(math.Pow10(tokenDecimals)) {
		fmt.Println("Tokentotalsupply exceeds precision limit")
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
		fmt.Printf("Invalid address: %s, error: %s\n", args[0], err)
		return
	}
	conn, err := rpcutil.GetGRPCConn(common.GetRPCAddr())
	if err != nil {
		fmt.Println(err)
		return
	}
	defer conn.Close()

	tag := txlogic.NewTokenTag(tokenName, tokenSymbol, uint32(tokenDecimals),
		uint64(tokenTotalSupply))
	tx, tid, _, err := rpcutil.NewIssueTokenTx(account, toAddr.Hash160(), tag, conn)
	if err != nil {
		fmt.Println(err)
		return
	}
	_, err = rpcutil.SendTransaction(conn, tx)
	if err != nil && !strings.Contains(err.Error(), core.ErrOrphanTransaction.Error()) {
		fmt.Println(err)
		return
	}

	fmt.Println("Created Token Address:", txlogic.EncodeOutPoint(
		txlogic.NewPbOutPoint(&tid.Hash, tid.Index)))
}

func transferTokenCmdFunc(cmd *cobra.Command, args []string) {
	if len(args) != 4 {
		fmt.Println(cmd.Use)
		return
	}
	// from account
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
	// token id
	tokenID, err := txlogic.DecodeOutPoint(args[1])
	if err != nil {
		fmt.Printf("DecodeOutPoint string %s to TokenID failed: %s\n", args[1], err)
		return
	}
	tokenHash := new(crypto.HashType)
	if err = tokenHash.SetBytes(tokenID.Hash); err != nil {
		fmt.Println("convert type []byte to HashType failed:", err)
		return
	}
	// to address
	to := strings.Split(args[2], ",")
	toHashes := make([]*types.AddressHash, 0, len(to))
	for _, addr := range to {
		address, err := types.ParseAddress(addr)
		if err != nil {
			_, ok1 := address.(*types.AddressPubKeyHash)
			_, ok2 := address.(*types.AddressTypeSplit)
			if !ok1 && !ok2 {
				fmt.Printf("invaild address for %s, err: %s\n", addr, err)
				return
			}
		}
		toHashes = append(toHashes, address.Hash160())
	}
	amountStr := strings.Split(args[3], ",")
	amounts := make([]uint64, 0, len(amountStr))
	for _, x := range amountStr {
		tmp, err := strconv.ParseUint(x, 10, 64)
		if err != nil {
			fmt.Printf("Conversion %s to unsigned numbers failed: %s\n", x, err)
			return
		}
		amounts = append(amounts, uint64(tmp))
	}
	//call RPC
	conn, err := rpcutil.GetGRPCConn(common.GetRPCAddr())
	if err != nil {
		fmt.Println(err)
		return
	}
	defer conn.Close()
	tx, _, _, err := rpcutil.NewTokenTx(account, toHashes, amounts, tokenHash.String(), tokenID.Index, conn)
	if err != nil {
		fmt.Println(err)
		return
	}
	hashStr, err := rpcutil.SendTransaction(conn, tx)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println("Tx Hash:", hashStr)
}

func getTokenBalanceCmdFunc(cmd *cobra.Command, args []string) {
	if len(args) != 2 {
		fmt.Println(cmd.Use)
		return
	}
	// hash
	tokenID, err := txlogic.DecodeOutPoint(args[0])
	if err != nil {
		fmt.Println(err)
		return
	}
	tokenHash := new(crypto.HashType)
	if err := tokenHash.SetBytes(tokenID.Hash); err != nil {
		fmt.Println("Conversion the type of tokenHash failed:", err)
		return
	}
	tokenIndex := tokenID.Index
	// address
	addrs := strings.Split(args[1], ",")
	for _, addr := range addrs {
		if _, err := types.ParseAddress(addr); err != nil {
			fmt.Println("invalid address")
			return
		}
	}
	respRPC, err := rpcutil.RPCCall(rpcpb.NewTransactionCommandClient, "GetTokenBalance",
		&rpcpb.GetTokenBalanceReq{Addrs: addrs, TokenHash: tokenHash.String(), TokenIndex: tokenIndex}, common.GetRPCAddr())
	if err != nil {
		fmt.Println(err)
		return
	}
	resp, ok := respRPC.(*rpcpb.GetBalanceResp)
	if !ok {
		fmt.Println("Convertion to rpcpb.GetBalanceRespfailed")
		return
	}
	if resp.Code != 0 {
		fmt.Println(resp.Message)
		return
	}
	for i, b := range resp.Balances {
		fmt.Printf("%s: %d\n", addrs[i], b)
	}
}
