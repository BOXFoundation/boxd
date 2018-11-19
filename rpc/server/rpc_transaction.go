// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package rpc

import (
	"context"
	"fmt"
	"github.com/BOXFoundation/boxd/core/pb"
	"github.com/BOXFoundation/boxd/script"

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

func (s *txServer) GetTransactionPool(ctx context.Context, req *rpcpb.GetTransactionPoolRequest) (*rpcpb.GetTransactionsResponse, error) {
	txs := s.server.GetTxHandler().GetTransactionsInPool()
	respTxs := []*corepb.Transaction{}
	for _, tx := range txs {
		respTx, err := tx.ToProtoMessage()
		if err != nil {
			return &rpcpb.GetTransactionsResponse{}, err
		}

		respTxs = append(respTxs, respTx.(*corepb.Transaction))
	}
	return &rpcpb.GetTransactionsResponse{Txs: respTxs}, nil
}

func (s *txServer) GetFeePrice(ctx context.Context, req *rpcpb.GetFeePriceRequest) (*rpcpb.GetFeePriceResponse, error) {
	return &rpcpb.GetFeePriceResponse{BoxPerByte: 1}, nil
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

func (s *txServer) GetTokenBalance(ctx context.Context, req *rpcpb.GetTokenBalanceRequest) (*rpcpb.GetTokenBalanceResponse, error) {
	balances := make(map[string]uint64)
	token := &types.OutPoint{}
	if err := token.FromProtoMessage(req.Token); err != nil {
		return &rpcpb.GetTokenBalanceResponse{
			Code:    -1,
			Message: err.Error(),
		}, err
	}
	for _, addrStr := range req.Addrs {
		addr, err := types.NewAddress(addrStr)
		if err != nil {
			return &rpcpb.GetTokenBalanceResponse{
				Code:    -1,
				Message: err.Error(),
			}, err
		}
		amount, err := s.getTokenBalance(ctx, addr, token)
		if err != nil {
			return &rpcpb.GetTokenBalanceResponse{Code: -1, Message: err.Error()}, err
		}
		balances[addrStr] = amount
	}
	return &rpcpb.GetTokenBalanceResponse{
		Code:     0,
		Message:  "ok",
		Balances: balances,
	}, nil
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

func (s *txServer) getTokenBalance(ctx context.Context, addr types.Address, token *types.OutPoint) (uint64, error) {
	utxos, err := s.server.GetChainReader().LoadUtxoByAddress(addr)
	if err != nil {
		return 0, err
	}
	var amount uint64
	for outpoint, value := range utxos {
		s := script.NewScriptFromBytes(value.Output.ScriptPubKey)
		if s.IsTokenIssue() {
			logger.Debug("Token issue utxo found")
			if outpoint != *token {
				// token type not match
				continue
			}
			issueParam, err := s.GetIssueParams()
			if err != nil {
				return 0, err
			}
			amount += issueParam.TotalSupply
		}
		if s.IsTokenTransfer() {
			logger.Debug("Token transfer utxo found")
			transferParam, err := s.GetTransferParams()
			if err != nil {
				return 0, err
			}
			if transferParam.OutPoint != *token {
				// token type not match
				continue
			}
			amount += transferParam.Amount
		}
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
	usedInMemoryPool := s.server.GetTxHandler().GetOutPointLockedByPool()
	for _, o := range usedInMemoryPool {
		delete(utxos, o)
	}
	if err != nil {
		return &rpcpb.ListUtxosResponse{Code: 1, Message: err.Error()}, nil
	}
	res := &rpcpb.ListUtxosResponse{
		Code:    0,
		Message: "ok",
		Count:   uint32(len(utxos)),
	}
	res.Utxos = []*rpcpb.Utxo{}
	var current uint64
	tokenAmount := make(map[types.OutPoint]uint64)
	if req.TokenBudgets != nil && len(req.TokenBudgets) > 0 {
		for _, budget := range req.TokenBudgets {
			outpoint := &types.OutPoint{}
			outpoint.FromProtoMessage(budget.Token)
			tokenAmount[*outpoint] = budget.Amount
		}
	}
	logger.Debugf("Requested token Info %v", tokenAmount)
	for out, utxo := range utxos {
		token, amount, isToken := getTokenInfo(out, utxo)
		if isToken {
			if val, ok := tokenAmount[token]; ok && val > 0 {
				logger.Debugf("Found utxo with token amount: %d", val)
				if val > amount {
					tokenAmount[token] = val - amount
				} else {
					delete(tokenAmount, token)
				}
				current += utxo.Value()
				res.Utxos = append(res.Utxos, generateUtxoMessage(&out, utxo))
			} else {
				// Do not include token utxos not needed
				continue
			}
		} else if current < req.GetAmount() {
			res.Utxos = append(res.Utxos, generateUtxoMessage(&out, utxo))
			current += utxo.Value()
		}
		if current >= req.GetAmount() && len(tokenAmount) == 0 {
			break
		}
	}
	if current < req.GetAmount() || len(tokenAmount) > 0 {
		errMsg := "Not enough balance"
		return &rpcpb.ListUtxosResponse{
			Code:    -1,
			Message: errMsg,
		}, fmt.Errorf(errMsg)
	}
	return res, nil
}

func getTokenInfo(outpoint types.OutPoint, wrap *types.UtxoWrap) (types.OutPoint, uint64, bool) {
	s := script.NewScriptFromBytes(wrap.Output.ScriptPubKey)
	if issueParam, err := s.GetIssueParams(); err == nil {
		return outpoint, issueParam.TotalSupply, true
	}
	if transferParam, err := s.GetTransferParams(); err == nil {
		return transferParam.OutPoint, transferParam.Amount, true
	}
	return types.OutPoint{}, 0, false
}

func (s *txServer) SendTransaction(ctx context.Context, req *rpcpb.SendTransactionRequest) (*rpcpb.BaseResponse, error) {
	logger.Debugf("receive transaction: %+v", req.Tx)

	for _, v := range req.Tx.Vin {
		hash := new(crypto.HashType)
		copy(hash[:], v.PrevOutPoint.Hash[:])
		logger.Debugf("receive transaction vin hash: %s", hash)
	}
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
		logger.Debug(err)
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
