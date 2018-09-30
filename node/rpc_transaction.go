// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package node

import (
	"context"

	"github.com/BOXFoundation/Quicksilver/core/types"
	"github.com/BOXFoundation/Quicksilver/crypto"
	"github.com/BOXFoundation/Quicksilver/rpc/pb"
	"google.golang.org/grpc"
)

func registerTransaction(s *grpc.Server) {
	rpcpb.RegisterTransactionCommandServer(s, &txServer{})
}

type txServer struct{}

func (s *txServer) ListUtxos(ctx context.Context, req *rpcpb.ListUtxosRequest) (*rpcpb.ListUtxosResponse, error) {
	utxos, err := nodeServer.bc.LoadUtxoByPubkey(req.GetPubKey())
	if err != nil {
		return &rpcpb.ListUtxosResponse{Code: 1, Message: err.Error()}, nil
	}
	res := &rpcpb.ListUtxosResponse{Code: 0, Message: "ok", Count: uint32(len(utxos))}
	res.Utxos = make([]*rpcpb.UtxoWrap, len(utxos))
	// for out, utxo := range utxos {
	// 	wrap := &rpcpb.UtxoWrap{}
	// }
	return res, nil
}

func (s *txServer) FundTransaction(ctx context.Context, req *rpcpb.FundTransactionRequest) (*rpcpb.ListUtxosResponse, error) {
	return &rpcpb.ListUtxosResponse{}, nil
}

func (s *txServer) SendTransaction(ctx context.Context, req *rpcpb.SendTransactionRequest) (*rpcpb.BaseResponse, error) {
	tx := &types.MsgTx{}
	tx.Version = req.Tx.Version
	tx.Magic = req.Tx.Magic
	tx.LockTime = req.Tx.LockTime
	tx.Vin = make([]*types.TxIn, len(req.Tx.GetVin()))
	for _, vin := range req.Tx.GetVin() {
		prevHash := crypto.HashType{}
		if err := prevHash.SetBytes(vin.PrevOutPoint.Hash); err != nil {
			return nil, err
		}
		in := &types.TxIn{
			PrevOutPoint: types.OutPoint{
				Hash:  prevHash,
				Index: vin.PrevOutPoint.Index,
			},
			ScriptSig: vin.ScriptSig,
			Sequence:  vin.Sequence,
		}
		tx.Vin = append(tx.Vin, in)
	}
	tx.Vout = make([]*types.TxOut, len(req.Tx.GetVout()))
	for _, vout := range req.Tx.GetVout() {
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
	err = nodeServer.bc.ProcessTx(transaction, true)
	return &rpcpb.BaseResponse{}, err
}
