// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/BOXFoundation/boxd/core/pb"
	"github.com/BOXFoundation/boxd/core/txlogic"
	"github.com/BOXFoundation/boxd/crypto"
	"github.com/BOXFoundation/boxd/integration_tests/utils"
	"github.com/BOXFoundation/boxd/rpc/pb"
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
	t.Logf("resp: %s", string(respBytes))

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
	t.Logf("send tx succeed, return hash: %+v", sendTxResp)

	time.Sleep(3 * time.Second)

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
		//"b1YMx5kufN2qELzKaoaBWzks2MZknYqqPnh",
		//"b1b1ncaV56DBPkSjhUVHePDErSESrBRUnyU",
		//"b1nfRQofEAHAkayCvwAfr4EVxhZEpQWhp8N",
		"b2ir3paoqze6Ft7GqKAdCk1ePkt4cbE2mZ9",
		//"b1oKfcV9tsiBjaTT4DxwANsrPi6br76vjqc",
	}
	//amounts := []uint64{100, 200, 300, 400}
	amounts := []uint64{1000}
	req := &rpcpb.MakeTxReq{
		From:    from,
		To:      to,
		Amounts: amounts,
		Fee:     50,
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
		From:    from,
		Addrs:   addrs,
		Weights: weights,
		Fee:     100,
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

	issuer := "b1fc1Vzz73WvBtzNQNbBSrxNCUC1Zrbnq4m"
	issuee := issuer
	tag := txlogic.NewTokenTag("box test token", "XOX", 4, 20000)
	req := &rpcpb.MakeTokenIssueTxReq{
		Issuer: issuer,
		Issuee: issuee,
		Tag:    tag,
		Fee:    10,
	}
	//
	hashStr := flow(t, func(ctx context.Context,
		client rpcpb.TransactionCommandClient) (string, makeTxResp) {
		resp, err := client.MakeUnsignedTokenIssueTx(ctx, req)
		if err != nil {
			t.Fatal(err)
		}
		return issuer, resp
	})
	// test token transfer
	tokenTxTest(t, hashStr)
}

// NOTE: to run this test case needs to start a node
// port: 19111
func tokenTxTest(t *testing.T, hashStr string) {

	from := "b1fc1Vzz73WvBtzNQNbBSrxNCUC1Zrbnq4m"
	to := []string{
		"b1afgd4BC3Y81ni3ds2YETikEkprG9Bxo98",
		"b1bgU3pRjrW2AXZ5DtumFJwrh69QTsErhAD",
	}
	amounts := []uint64{1000, 4000}
	// read hash from terminal
	hash, idx := new(crypto.HashType), uint32(0)
	hash.SetString(hashStr)
	tid := txlogic.NewPbOutPoint(hash, idx)
	req := &rpcpb.MakeTokenTransferTxReq{
		From:    from,
		To:      to,
		Amounts: amounts,
		Fee:     50,
		TokenID: tid,
	}
	//
	flow(t, func(ctx context.Context,
		client rpcpb.TransactionCommandClient) (string, makeTxResp) {
		resp, err := client.MakeUnsignedTokenTransferTx(ctx, req)
		if err != nil {
			t.Fatal(err)
		}
		return from, resp
	})
}
