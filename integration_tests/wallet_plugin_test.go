// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	corepb "github.com/BOXFoundation/boxd/core/pb"
	"github.com/BOXFoundation/boxd/core/txlogic"
	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/crypto"
	"github.com/BOXFoundation/boxd/integration_tests/utils"
	rpcpb "github.com/BOXFoundation/boxd/rpc/pb"
	"github.com/BOXFoundation/boxd/rpc/rpcutil"
	"github.com/BOXFoundation/boxd/script"
	acc "github.com/BOXFoundation/boxd/wallet/account"
)

func init() {
	utils.LoadConf()
	initAcc()
}

func initAcc() {
	file := utils.LocalConf.KeyDir + "pre.keystore"
	addr, acc := utils.MinerAccounts(file)
	AddrToAcc.Store(addr[0], acc[0])
}

type makeTxResp interface {
	GetRawMsgs() [][]byte
	GetTx() *corepb.Transaction
}

type makeTxRespFunc func(
	ctx context.Context, client rpcpb.TransactionCommandClient,
) (string, makeTxResp)

func flow(t *testing.T, respFunc makeTxRespFunc) string {
	// get grpc conn
	conn, err := rpcutil.GetGRPCConn("127.0.0.1:19111")
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	client := rpcpb.NewTransactionCommandClient(conn)

	from, resp := respFunc(ctx, client)
	respBytes, _ := json.MarshalIndent(resp, "", "  ")
	logger.Infof("resp: %s", string(respBytes))

	rawMsgs := resp.GetRawMsgs()
	sigHashes := make([]*crypto.HashType, 0, len(rawMsgs))
	for _, msg := range rawMsgs {
		hash := crypto.DoubleHashH(msg)
		sigHashes = append(sigHashes, &hash)
	}
	// sign msg
	accI, ok := AddrToAcc.Load(from)
	if !ok {
		t.Fatalf("account for %s not initialled", from)
	}
	account := accI.(*acc.Account)
	var scriptSigs [][]byte
	for _, sigHash := range sigHashes {
		sig, err := account.Sign(sigHash)
		if err != nil {
			t.Fatalf("sign error for %+v: %s", sigHash, err)
		}
		scriptSig := script.SignatureScript(sig, account.PublicKey())
		scriptSigs = append(scriptSigs, *scriptSig)
	}

	// make tx with sig
	tx := resp.GetTx()
	for i, sig := range scriptSigs {
		tx.Vin[i].ScriptSig = sig
	}

	// send tx
	stReq := &rpcpb.SendTransactionReq{Tx: tx}
	sendTxResp, err := client.SendTransaction(ctx, stReq)
	if err != nil {
		t.Fatal(err)
	}
	if sendTxResp.Code != 0 {
		t.Fatalf("send tx resp: %+v", sendTxResp)
	}
	logger.Infof("send tx succeed, return hash: %+v", sendTxResp)

	time.Sleep(2 * time.Second)

	// view tx detail
	vconn, err := rpcutil.GetGRPCConn("127.0.0.1:19111")
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	vctx, vcancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer vcancel()
	viewClient := rpcpb.NewWebApiClient(vconn)
	viewReq := &rpcpb.ViewTxDetailReq{
		Hash: sendTxResp.Hash,
	}
	txDetailResp, err := viewClient.ViewTxDetail(vctx, viewReq)
	if err != nil {
		t.Fatal(err)
	}
	bytes, _ := json.MarshalIndent(txDetailResp, "", "  ")
	t.Logf("tx detail: %s", string(bytes))

	return txDetailResp.Detail.Hash
}

// NOTE: to run this test case needs to start a node
// port: 19111
func _TestNormalTx(t *testing.T) {

	from := "b1fc1Vzz73WvBtzNQNbBSrxNCUC1Zrbnq4m"
	to := []string{
		//"b1e4se6C2bwX3vTWHcHmT9s87kxrBGEJkEn",
		//"b1fJy9WSDDn1vwiDb8Cd7GiF3mPzaYFPEdy",
		//"b1fNm3MuBgAKD7WwZbLZBWJ6nV2JVVcFAv7",
		//"b1n4ffVkctWmXptM6ojVkrA3vsrtyf9nm1e",
		"b2KWSqUWZHTdP4g8kHkHtFtNc8Nofr1twqz",
	}
	//amounts := []uint64{100, 200, 300, 400}
	amounts := []uint64{1000}
	req := &rpcpb.MakeTxReq{
		From:     from,
		To:       to,
		Amounts:  amounts,
		GasPrice: 10,
	}
	//
	flow(t, func(ctx context.Context,
		client rpcpb.TransactionCommandClient) (string, makeTxResp) {
		resp, err := client.MakeUnsignedTx(ctx, req)
		if err != nil {
			t.Fatal(err)
		}
		return from, resp
	})
}

// NOTE: to run this test case needs to start a node
// port: 19111
func _TestSplitAddrTx(t *testing.T) {

	from := "b1fc1Vzz73WvBtzNQNbBSrxNCUC1Zrbnq4m"
	addrs := []string{
		"b1YMx5kufN2qELzKaoaBWzks2MZknYqqPnh",
		"b1b1ncaV56DBPkSjhUVHePDErSESrBRUnyU",
		"b1nfRQofEAHAkayCvwAfr4EVxhZEpQWhp8N",
		"b1oKfcV9tsiBjaTT4DxwANsrPi6br76vjqc",
	}
	weights := []uint64{1, 2, 3, 4}
	req := &rpcpb.MakeSplitAddrTxReq{
		From:     from,
		Addrs:    addrs,
		Weights:  weights,
		GasPrice: 10,
	}
	//
	flow(t, func(ctx context.Context,
		client rpcpb.TransactionCommandClient) (string, makeTxResp) {
		resp, err := client.MakeUnsignedSplitAddrTx(ctx, req)
		if err != nil {
			t.Fatal(err)
		}
		return from, resp
	})
}

// NOTE: to run this test case needs to start a node
// port: 19111
func _TestTokenTx(t *testing.T) {

	// token issue
	issuer := "b1fc1Vzz73WvBtzNQNbBSrxNCUC1Zrbnq4m"
	owner := issuer
	tag := txlogic.NewTokenTag("box test token", "XOX", 4, 20000)
	req := &rpcpb.MakeTokenIssueTxReq{
		Issuer:   issuer,
		Owner:    owner,
		Tag:      tag,
		GasPrice: 10,
	}
	bytes, _ := json.MarshalIndent(req, "", "  ")
	logger.Errorf("make unsigned token issue tx: %s", string(bytes))
	//
	hashStr := flow(t, func(ctx context.Context,
		client rpcpb.TransactionCommandClient) (string, makeTxResp) {
		resp, err := client.MakeUnsignedTokenIssueTx(ctx, req)
		if err != nil {
			t.Fatal(err)
		}
		return issuer, resp
	})

	// token transfer
	from := "b1fc1Vzz73WvBtzNQNbBSrxNCUC1Zrbnq4m"
	to := []string{
		"b1afgd4BC3Y81ni3ds2YETikEkprG9Bxo98",
		"b1bgU3pRjrW2AXZ5DtumFJwrh69QTsErhAD",
	}
	amounts := []uint64{1000, 4000}
	// read hash from terminal
	reqT := &rpcpb.MakeTokenTransferTxReq{
		From:       from,
		To:         to,
		Amounts:    amounts,
		GasPrice:   10,
		TokenHash:  hashStr,
		TokenIndex: 0,
	}
	//
	flow(t, func(ctx context.Context,
		client rpcpb.TransactionCommandClient) (string, makeTxResp) {
		resp, err := client.MakeUnsignedTokenTransferTx(ctx, reqT)
		if err != nil {
			t.Fatal(err)
		}
		return from, resp
	})
}

// NOTE: to run this test case needs to start a node
// port: 19111
func _TestContractTx(t *testing.T) {

	var (
		testFaucetContract = "608060405260f7806100126000396000f3fe6080604052600436106039576000357c0100000000000000000000000000000000000000000000000000000000900480632e1a7d4d14603b575b005b348015604657600080fd5b50607060048036036020811015605b57600080fd5b81019080803590602001909291905050506072565b005b6127108111151515608257600080fd5b3373ffffffffffffffffffffffffffffffffffffffff166108fc829081150290604051600060405180830381858888f1935050505015801560c7573d6000803e3d6000fd5b505056fea165627a7a7230582041951f9857bb67cda6bccbb59f6fdbf38eeddc244530e577d8cad6194941d38c0029"
		// withdraw 2000
		testFaucetCall = "2e1a7d4d00000000000000000000000000000000000000000000000000000000000007d0"
	)

	// contract deploy
	from := "b1fc1Vzz73WvBtzNQNbBSrxNCUC1Zrbnq4m"
	req := &rpcpb.MakeContractTxReq{
		From:     from,
		Amount:   10000,
		GasPrice: 2,
		GasLimit: 100000,
		Nonce:    1,
		IsDeploy: true,
		Data:     testFaucetContract,
	}
	bytes, _ := json.MarshalIndent(req, "", "  ")
	logger.Infof("make contract tx request: %s", string(bytes))
	//
	hashStr := flow(t, func(ctx context.Context,
		client rpcpb.TransactionCommandClient) (string, makeTxResp) {
		resp, err := client.MakeUnsignedContractTx(ctx, req)
		if err != nil {
			t.Fatal(err)
		}
		return from, resp
	})
	logger.Infof("contract deploy tx hash: %s", hashStr)

	// contract call
	senderHash, _ := types.NewAddress(from)
	contractAddr, _ := types.MakeContractAddress(senderHash, 1)
	req = &rpcpb.MakeContractTxReq{
		From:     from,
		To:       contractAddr.String(),
		Amount:   0,
		GasPrice: 2,
		GasLimit: 20000,
		Nonce:    2,
		Data:     testFaucetCall,
	}
	//
	flow(t, func(ctx context.Context,
		client rpcpb.TransactionCommandClient) (string, makeTxResp) {
		resp, err := client.MakeUnsignedContractTx(ctx, req)
		if err != nil {
			t.Fatal(err)
		}
		return from, resp
	})
}
