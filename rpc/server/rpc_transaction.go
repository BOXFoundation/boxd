// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package rpc

import (
	"bytes"
	"context"
	"math"
	"reflect"

	"github.com/BOXFoundation/boxd/core"
	"github.com/BOXFoundation/boxd/core/pb"
	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/crypto"
	"github.com/BOXFoundation/boxd/rpc/pb"
	"github.com/BOXFoundation/boxd/script"
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

func newGetBalance(code int32, msg string, balances ...uint64) *rpcpb.GetBalanceResp {
	return &rpcpb.GetBalanceResp{
		Code:     code,
		Message:  msg,
		Balances: balances,
	}
}

func (s *txServer) GetBalance(
	ctx context.Context, req *rpcpb.GetBalanceReq) (
	*rpcpb.GetBalanceResp, error,
) {
	logger.Infof("get balance req: %+v", req)
	walletAgent := s.server.GetWalletAgent()
	if walletAgent == nil || reflect.ValueOf(walletAgent).IsNil() {
		logger.Warn("get balance error ", ErrAPINotSupported)
		return newGetBalance(-1, ErrAPINotSupported.Error()), ErrAPINotSupported
	}
	balances := make([]uint64, len(req.GetAddrs()))
	for i, addr := range req.Addrs {
		amount, err := walletAgent.Balance(addr)
		if err != nil {
			logger.Warnf("get balance for %s error %s", addr, err)
			return newGetBalance(-1, err.Error()), nil
		}
		balances[i] = amount
	}
	logger.Infof("get balance for %v result: %v", req.Addrs, balances)
	return newGetBalance(0, "ok", balances...), nil
}

func newFetchUtxosResp(code int32, msg string, utxos ...*rpcpb.Utxo) *rpcpb.FetchUtxosResp {
	return &rpcpb.FetchUtxosResp{
		Code:    code,
		Message: msg,
		Utxos:   utxos,
	}
}

// FetchUtxos fetch utxos from chain
func (s *txServer) FetchUtxos(
	ctx context.Context, req *rpcpb.FetchUtxosReq) (
	*rpcpb.FetchUtxosResp, error,
) {
	logger.Infof("fetch utxos req: %+v", req)
	walletAgent := s.server.GetWalletAgent()
	if walletAgent == nil || reflect.ValueOf(walletAgent).IsNil() {
		logger.Warn("fetch utxos error ", ErrAPINotSupported)
		return newFetchUtxosResp(-1, ErrAPINotSupported.Error()), ErrAPINotSupported
	}
	utxos, err := walletAgent.Utxos(req.GetAddr(), req.GetAmount())
	if err != nil {
		logger.Warnf("fetch utxos for %s error %s", req.GetAddr(), err)
		return newFetchUtxosResp(-1, err.Error()), nil
	}
	total := uint64(0)
	for _, u := range utxos {
		total += u.TxOut.Value
	}
	logger.Infof("fetch utxos for %s return %d utxos total %d",
		req.Addr, len(utxos), total)
	return newFetchUtxosResp(0, "ok", utxos...), nil
}

func (s *txServer) GetTransactionPool(ctx context.Context, req *rpcpb.GetTransactionPoolRequest) (*rpcpb.GetTransactionsResponse, error) {
	txs, _ := s.server.GetTxHandler().GetTransactionsInPool()
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
	return res, nil
}

func (s *txServer) GetTokenBalance(
	ctx context.Context, req *rpcpb.GetTokenBalanceRequest) (
	*rpcpb.GetTokenBalanceResponse, error) {

	token := &types.OutPoint{}
	if err := token.FromProtoMessage(req.Token); err != nil {
		return &rpcpb.GetTokenBalanceResponse{
			Code:    -1,
			Message: err.Error(),
		}, err
	}
	balances := make([]uint64, 0, len(req.Addrs))
	for i, addr := range req.Addrs {
		amount, err := s.getTokenBalance(ctx, addr, token)
		if err != nil {
			return &rpcpb.GetTokenBalanceResponse{Code: -1, Message: err.Error()}, err
		}
		balances[i] = amount
	}
	return &rpcpb.GetTokenBalanceResponse{
		Code:     0,
		Message:  "ok",
		Balances: balances,
	}, nil
}

func (s *txServer) getTokenBalance(
	ctx context.Context, addr string, token *types.OutPoint) (
	uint64, error) {
	walletAgent := s.server.GetWalletAgent()
	if walletAgent == nil || reflect.ValueOf(walletAgent).IsNil() {
		logger.Warn("get balance error ", ErrAPINotSupported)
		return 0, ErrAPINotSupported
	}
	utxos, err := walletAgent.Utxos(addr, 0)
	if err != nil {
		return 0, err
	}
	var amount uint64
	for _, utxo := range utxos {
		s := script.NewScriptFromBytes(utxo.TxOut.ScriptPubKey)
		if s.IsTokenIssue() {
			if !bytes.Equal(utxo.OutPoint.Hash, token.Hash[:]) || utxo.OutPoint.Index != token.Index {
				// token type not match
				continue
			}
			issueParam, err := s.GetIssueParams()
			if err != nil {
				return 0, err
			}
			amount += issueParam.TotalSupply * uint64(math.Pow10(int(issueParam.Decimals)))
		}
		if s.IsTokenTransfer() {
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

//func (s *txServer) FundTransaction(ctx context.Context, req *rpcpb.FundTransactionRequest) (*rpcpb.ListUtxosResponse, error) {
//	if s.server.GetWalletAgent() == nil {
//		return nil, fmt.Errorf("api fund transaction not supported")
//	}
//	addr, err := types.NewAddress(req.Addr)
//	payToPubKeyHashScript := *script.PayToPubKeyHashScript(addr.Hash())
//	if err != nil {
//		return &rpcpb.ListUtxosResponse{Code: 1, Message: err.Error()}, nil
//	}
//	utxos, err := s.server.GetWalletAgent().Utxos(addr)
//	if err != nil {
//		return &rpcpb.ListUtxosResponse{Code: 1, Message: err.Error()}, nil
//	}
//
//	nextHeight := s.server.GetChainReader().GetBlockHeight() + 1
//
//	// apply mempool txs as if they were mined into a block with 0 confirmation
//	utxoSet := chain.NewUtxoSetFromMap(utxos)
//	memPoolTxs, _ := s.server.GetTxHandler().GetTransactionsInPool()
//	// Note: we add utxo first and spend them later to maintain tx topological order within mempool. Since memPoolTxs may not
//	// be topologically ordered, if tx1 spends tx2 but tx1 comes after tx2, tx1's output is mistakenly marked as unspent
//	// Add utxos first
//	for _, tx := range memPoolTxs {
//		for txOutIdx, txOut := range tx.Vout {
//			// utxo for this address
//			if util.IsPrefixed(txOut.ScriptPubKey, payToPubKeyHashScript) {
//				if err := utxoSet.AddUtxo(tx, uint32(txOutIdx), nextHeight); err != nil {
//					return &rpcpb.ListUtxosResponse{Code: 1, Message: err.Error()}, nil
//				}
//			}
//		}
//	}
//	// Then spend
//	for _, tx := range memPoolTxs {
//		for _, txIn := range tx.Vin {
//			utxoSet.SpendUtxo(txIn.PrevOutPoint)
//		}
//	}
//	utxos = utxoSet.GetUtxos()
//
//	res := &rpcpb.ListUtxosResponse{
//		Code:    0,
//		Message: "ok",
//		Count:   uint32(len(utxos)),
//	}
//	res.Utxos = []*rpcpb.Utxo{}
//	var current uint64
//	tokenAmount := make(map[types.OutPoint]uint64)
//	if req.TokenBudgets != nil && len(req.TokenBudgets) > 0 {
//		for _, budget := range req.TokenBudgets {
//			outpoint := &types.OutPoint{}
//			outpoint.FromProtoMessage(budget.Token)
//			tokenAmount[*outpoint] = budget.Amount
//		}
//	}
//	for out, utxo := range utxos {
//		token, amount, isToken := getTokenInfo(out, utxo)
//		if isToken {
//			if val, ok := tokenAmount[token]; ok && val > 0 {
//				if val > amount {
//					tokenAmount[token] = val - amount
//				} else {
//					delete(tokenAmount, token)
//				}
//				current += utxo.Value()
//				res.Utxos = append(res.Utxos, generateUtxoMessage(&out, utxo))
//			} else {
//				// Do not include token utxos not needed
//				continue
//			}
//		} else if current < req.GetAmount() {
//			res.Utxos = append(res.Utxos, generateUtxoMessage(&out, utxo))
//			current += utxo.Value()
//		}
//		if current >= req.GetAmount() && len(tokenAmount) == 0 {
//			break
//		}
//	}
//	if current < req.GetAmount() || len(tokenAmount) > 0 {
//		errMsg := "Not enough balance"
//		return &rpcpb.ListUtxosResponse{
//			Code:    -1,
//			Message: errMsg,
//		}, fmt.Errorf(errMsg)
//	}
//	return res, nil
//}

func getTokenInfo(outpoint types.OutPoint, wrap *types.UtxoWrap) (types.OutPoint, uint64, bool) {
	s := script.NewScriptFromBytes(wrap.Output.ScriptPubKey)
	if s.IsTokenIssue() {
		if issueParam, err := s.GetIssueParams(); err == nil {
			return outpoint, issueParam.TotalSupply * uint64(math.Pow10(int(issueParam.Decimals))), true
		}
	}
	if s.IsTokenTransfer() {
		if transferParam, err := s.GetTransferParams(); err == nil {
			return transferParam.OutPoint, transferParam.Amount, true
		}
	}
	return types.OutPoint{}, 0, false
}

func (s *txServer) SendTransaction(
	ctx context.Context, req *rpcpb.SendTransactionRequest) (
	*rpcpb.BaseResponse, error) {

	tx := &types.Transaction{}
	if err := tx.FromProtoMessage(req.Tx); err != nil {
		return nil, err
	}

	txpool := s.server.GetTxHandler()
	err := txpool.ProcessTx(tx, core.BroadcastMode)
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
