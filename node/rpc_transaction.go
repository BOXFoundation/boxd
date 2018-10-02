// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package node

import (
	"context"

	"github.com/BOXFoundation/Quicksilver/core"

	"github.com/BOXFoundation/Quicksilver/core/pb"
	"github.com/BOXFoundation/Quicksilver/core/types"
	"github.com/BOXFoundation/Quicksilver/crypto"
	"github.com/BOXFoundation/Quicksilver/rpc/pb"
	rpcserver "github.com/BOXFoundation/Quicksilver/rpc/server"
	"google.golang.org/grpc"
)

func registerTransaction(s *grpc.Server) {
	rpcpb.RegisterTransactionCommandServer(s, &txServer{})
}

func init() {
	rpcserver.RegisterServiceWithGatewayHandler(
		"tx",
		registerTransaction,
		rpcpb.RegisterTransactionCommandHandlerFromEndpoint,
	)
}

type txServer struct{}

func (s *txServer) ListUtxos(ctx context.Context, req *rpcpb.ListUtxosRequest) (*rpcpb.ListUtxosResponse, error) {
	utxos := nodeServer.bc.ListAllUtxos()
	res := &rpcpb.ListUtxosResponse{
		Code:    0,
		Message: "ok",
		Count:   uint32(len(utxos)),
	}
	res.Utxos = []*rpcpb.Utxo{}
	for out, utxo := range utxos {
		res.Utxos = append(res.Utxos, generateUtxoMessage(&out, utxo))
	}
	return res, nil
}

func (s *txServer) FundTransaction(ctx context.Context, req *rpcpb.FundTransactionRequest) (*rpcpb.ListUtxosResponse, error) {
	utxos, err := nodeServer.bc.LoadUtxoByPubKey(req.ScriptPubKey)
	if err != nil {
		return &rpcpb.ListUtxosResponse{Code: 1, Message: err.Error()}, nil
	}
	res := &rpcpb.ListUtxosResponse{
		Code:    0,
		Message: "ok",
		Count:   uint32(len(utxos)),
	}
	res.Utxos = []*rpcpb.Utxo{}
	for out, utxo := range utxos {
		res.Utxos = append(res.Utxos, generateUtxoMessage(&out, utxo))
	}
	return res, nil
}

func (s *txServer) SendTransaction(ctx context.Context, req *rpcpb.SendTransactionRequest) (*rpcpb.BaseResponse, error) {
	logger.Debugf("receive transaction: %+v", req.Tx)
	tx, err := generateTransaction(req.Tx)
	if err != nil {
		return nil, err
	}
	err = nodeServer.bc.ProcessTx(tx, true)
	return &rpcpb.BaseResponse{}, err
}

func generateUtxoMessage(outPoint *types.OutPoint, entry *core.UtxoEntry) *rpcpb.Utxo {
	return &rpcpb.Utxo{
		BlockHeight: entry.BlockHeight,
		IsCoinbase:  entry.IsCoinBase,
		IsSpent:     entry.IsSpent,
		OutPoint: &corepb.OutPoint{
			Hash:  outPoint.Hash.GetBytes(),
			Index: outPoint.Index,
		},
		TxOut: &corepb.TxOut{
			Value:        entry.Value(),
			ScriptPubKey: entry.Output.ScriptPubKey,
		},
	}
}

func generateTransaction(txMsg *corepb.MsgTx) (*types.Transaction, error) {
	tx := &types.MsgTx{
		Version:  txMsg.Version,
		Magic:    txMsg.Magic,
		LockTime: txMsg.LockTime,
	}
	tx.Vin = []*types.TxIn{}
	for _, vin := range txMsg.GetVin() {
		in, err := generateTxIn(vin)
		if err != nil {
			return nil, err
		}
		tx.Vin = append(tx.Vin, in)
	}
	logger.Debugf("tx.Vin info : %+v", tx.Vin)
	tx.Vout = []*types.TxOut{}
	for _, vout := range txMsg.GetVout() {
		out := &types.TxOut{
			Value:        vout.Value,
			ScriptPubKey: vout.ScriptPubKey,
		}
		tx.Vout = append(tx.Vout, out)
	}
	txHash, err := tx.MsgTxHash()
	if err != nil {
		return nil, err
	}
	transaction := &types.Transaction{
		MsgTx: tx,
		Hash:  txHash,
	}
	return transaction, nil
}

func generateTxIn(msgTxIn *corepb.TxIn) (*types.TxIn, error) {
	prevHash := crypto.HashType{}
	if err := prevHash.SetBytes(msgTxIn.PrevOutPoint.Hash); err != nil {
		return nil, err
	}
	logger.Debugf("Generate TxIn hash: %s, index: %d", prevHash, msgTxIn.PrevOutPoint.Index)
	return &types.TxIn{
		PrevOutPoint: types.OutPoint{
			Hash:  prevHash,
			Index: msgTxIn.PrevOutPoint.Index,
		},
		ScriptSig: msgTxIn.ScriptSig,
		Sequence:  msgTxIn.Sequence,
	}, nil
}
