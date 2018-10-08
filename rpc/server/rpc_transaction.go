// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package rpc

import (
	"context"

	"github.com/BOXFoundation/boxd/core/chain"
	"github.com/BOXFoundation/boxd/core/pb"
	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/rpc/pb"
)

func registerTransaction(s *Server) {
	rpcpb.RegisterTransactionCommandServer(s.server, &txServer{server: s})
}

func init() {
	RegisterServiceWithGatewayHandler(
		"tx",
		registerTransaction,
		rpcpb.RegisterTransactionCommandHandlerFromEndpoint,
	)
}

type txServer struct {
	server GRPCServer
}

func (s *txServer) ListUtxos(ctx context.Context, req *rpcpb.ListUtxosRequest) (*rpcpb.ListUtxosResponse, error) {
	bc := s.server.BoxdServer().BlockChain()
	utxos := bc.ListAllUtxos()
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
	bc := s.server.BoxdServer().BlockChain()
	utxos, err := bc.LoadUtxoByPubKey(req.ScriptPubKey)
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
	bc := s.server.BoxdServer().BlockChain()
	tx, err := generateTransaction(req.Tx)
	if err != nil {
		return nil, err
	}
	err = bc.ProcessTx(tx, true)
	return &rpcpb.BaseResponse{}, err
}

func generateUtxoMessage(outPoint *types.OutPoint, entry *chain.UtxoEntry) *rpcpb.Utxo {
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

func generateTransaction(txMsg *corepb.Transaction) (*types.Transaction, error) {
	tx := &types.Transaction{}
	if err := tx.FromProtoMessage(txMsg); err != nil {
		return nil, err
	}

	return tx, nil
}

func generateTxIn(txIn *corepb.TxIn) (*types.TxIn, error) {
	txin := &types.TxIn{}
	if err := txin.FromProtoMessage(txIn); err != nil {
		return nil, err
	}

	return txin, nil
}
