// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package rpc

import (
	"context"

	"github.com/BOXFoundation/boxd/core/pb"
	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/crypto"
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
	bc := s.server.GetChainReader()
	utxos, err := bc.ListAllUtxos()
	if err != nil {
		return &rpcpb.ListUtxosResponse{
			Code:    1,
			Message: err.Error(),
		}, err
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
	logger.Debugf("List Utxo called, utxos: %+v", utxos)
	return res, nil
}

func (s *txServer) GetBalance(ctx context.Context, req *rpcpb.GetBalanceRequest) (*rpcpb.GetBalanceResponse, error) {
	balances := make(map[string]uint64)
	for _, addrStr := range req.Addrs {
		addr, err := types.NewAddress(addrStr)
		if err != nil {
			return &rpcpb.GetBalanceResponse{Code: -1, Message: err.Error()}, err
		}
		amount, err := s.getbalance(ctx, addr)
		if err != nil {
			return &rpcpb.GetBalanceResponse{Code: -1, Message: err.Error()}, err
		}
		balances[addrStr] = amount
	}
	return &rpcpb.GetBalanceResponse{Code: 0, Message: "ok", Balances: balances}, nil
}

func (s *txServer) getbalance(ctx context.Context, addr types.Address) (uint64, error) {
	utxos, err := s.server.GetChainReader().LoadUtxoByAddress(addr)
	if err != nil {
		return 0, err
	}
	var amount uint64
	for _, value := range utxos {
		amount += value.Output.Value
	}
	return amount, nil
}

func (s *txServer) FundTransaction(ctx context.Context, req *rpcpb.FundTransactionRequest) (*rpcpb.ListUtxosResponse, error) {
	bc := s.server.GetChainReader()
	addr, err := types.NewAddress(req.Addr)
	if err != nil {
		return &rpcpb.ListUtxosResponse{Code: 1, Message: err.Error()}, nil
	}
	utxos, err := bc.LoadUtxoByAddress(addr)
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
	txpool := s.server.GetTxHandler()
	tx, err := generateTransaction(req.Tx)
	if err != nil {
		return nil, err
	}
	err = txpool.ProcessTx(tx, true /* relay */)
	return &rpcpb.BaseResponse{}, err
}

func (s *txServer) GetRawTransaction(ctx context.Context, req *rpcpb.GetRawTransactionRequest) (*rpcpb.GetRawTransactionResponse, error) {
	hash := crypto.HashType{}
	if err := hash.SetBytes(req.Hash); err != nil {
		return &rpcpb.GetRawTransactionResponse{}, err
	}
	tx, err := s.server.GetChainReader().LoadTxByHash(hash)
	if err != nil {
		return &rpcpb.GetRawTransactionResponse{}, err
	}
	rpcTx, err := tx.ToProtoMessage()
	return &rpcpb.GetRawTransactionResponse{Tx: rpcTx.(*corepb.Transaction)}, err
}

func generateUtxoMessage(outPoint *types.OutPoint, entry *types.UtxoWrap) *rpcpb.Utxo {
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
