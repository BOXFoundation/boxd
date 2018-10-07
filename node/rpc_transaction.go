// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package node

import (
	"context"

	"github.com/BOXFoundation/Quicksilver/core"

	"github.com/BOXFoundation/Quicksilver/core/pb"
	"github.com/BOXFoundation/Quicksilver/core/types"
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
	tx := &types.MsgTx{}
	if err := tx.Deserialize(txMsg); err != nil {
		return nil, err
	}

	return types.NewTx(tx)
}

func generateTxIn(msgTxIn *corepb.TxIn) (*types.TxIn, error) {
	txin := &types.TxIn{}
	if err := txin.Deserialize(msgTxIn); err != nil {
		return nil, err
	}

	return txin, nil
}
