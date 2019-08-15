// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package rpc

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"reflect"
	"strings"

	"github.com/BOXFoundation/boxd/core"
	corepb "github.com/BOXFoundation/boxd/core/pb"
	"github.com/BOXFoundation/boxd/core/txlogic"
	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/crypto"
	rpcpb "github.com/BOXFoundation/boxd/rpc/pb"
	"github.com/BOXFoundation/boxd/rpc/rpcutil"
)

const maxDecimal = 8

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

func newGetBalanceResp(code int32, msg string, balances ...uint64) *rpcpb.GetBalanceResp {
	return &rpcpb.GetBalanceResp{
		Code:     code,
		Message:  msg,
		Balances: balances,
	}
}

func (s *txServer) GetBalance(
	ctx context.Context, req *rpcpb.GetBalanceReq,
) (*rpcpb.GetBalanceResp, error) {
	balances := make([]uint64, len(req.GetAddrs()))
	statedb := s.server.GetChainReader().TailState()
	for i, addr := range req.Addrs {
		address, err := types.NewAddress(addr)
		if err != nil {
			logger.Warn(err)
			return newGetBalanceResp(-1, err.Error()), nil
		}
		amount := statedb.GetBalance(*address.Hash160()).Uint64()
		balances[i] = amount
	}
	return newGetBalanceResp(0, "ok", balances...), nil
}

func (s *txServer) GetTokenBalance(
	ctx context.Context, req *rpcpb.GetTokenBalanceReq,
) (*rpcpb.GetBalanceResp, error) {
	walletAgent := s.server.GetWalletAgent()
	if walletAgent == nil {
		logger.Warn("get token balance error ", ErrAPINotSupported)
		return newGetBalanceResp(-1, ErrAPINotSupported.Error()), nil
	}
	balances := make([]uint64, len(req.GetAddrs()))
	// parse tid
	thash := new(crypto.HashType)
	if err := thash.SetString(req.GetTokenHash()); err != nil {
		logger.Warn("get token balance error, invalid token hash %s", req.GetTokenHash())
		return newGetBalanceResp(-1, "invalid token hash"), nil
	}
	tid := txlogic.NewTokenID(thash, req.GetTokenIndex())
	// valide addrs
	if err := types.ValidateAddr(req.Addrs...); err != nil {
		logger.Warn(err)
		return newGetBalanceResp(-1, err.Error()), nil
	}
	for i, addr := range req.Addrs {
		amount, err := walletAgent.Balance(addr, tid)
		if err != nil {
			logger.Warnf("get token balance for %s token id: %+v error: %s", addr, tid, err)
			return newGetBalanceResp(-1, err.Error()), nil
		}
		balances[i] = amount
	}
	logger.Infof("get token balance for %v %+v result: %v", req.Addrs, tid, balances)
	return newGetBalanceResp(0, "ok", balances...), nil
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
	ctx context.Context, req *rpcpb.FetchUtxosReq,
) (resp *rpcpb.FetchUtxosResp, err error) {

	defer func() {
		if resp.Code != 0 {
			logger.Warnf("fetch utxos: %s error: %s", tolog(req), resp.Message)
		} else {
			total := uint64(0)
			for _, u := range resp.GetUtxos() {
				amount, _, err := txlogic.ParseUtxoAmount(u)
				if err != nil {
					logger.Warnf("parse utxo %+v amout error %s", u, err)
					continue
				}
				total += amount
			}
			logger.Infof("fetch utxos: %s succeeded, return %d utxos total %d",
				tolog(req), len(resp.GetUtxos()), total)
		}
	}()

	walletAgent := s.server.GetWalletAgent()
	if walletAgent == nil || reflect.ValueOf(walletAgent).IsNil() {
		return newFetchUtxosResp(-1, ErrAPINotSupported.Error()), nil
	}
	var tid *types.TokenID
	tHashStr, tIdx := req.GetTokenHash(), req.GetTokenIndex()
	if tHashStr != "" {
		tHash := new(crypto.HashType)
		if err := tHash.SetString(tHashStr); err != nil {
			return newFetchUtxosResp(-1, err.Error()), nil
		}
		tid = txlogic.NewTokenID(tHash, tIdx)
	}
	addr := req.GetAddr()
	if err := types.ValidateAddr(addr); err != nil {
		return newFetchUtxosResp(-1, err.Error()), nil
	}
	utxos, err := walletAgent.Utxos(addr, tid, req.GetAmount())
	if err != nil {
		return newFetchUtxosResp(-1, err.Error()), nil
	}
	return newFetchUtxosResp(0, "ok", utxos...), nil
}

func (s *txServer) GetFeePrice(ctx context.Context, req *rpcpb.GetFeePriceRequest) (*rpcpb.GetFeePriceResponse, error) {
	return &rpcpb.GetFeePriceResponse{BoxPerByte: 1}, nil
}

func newSendTransactionResp(code int32, msg, hash string) *rpcpb.SendTransactionResp {
	return &rpcpb.SendTransactionResp{
		Code:    code,
		Message: msg,
		Hash:    hash,
	}
}

func (s *txServer) SendRawTransaction(
	ctx context.Context, req *rpcpb.SendRawTransactionReq,
) (resp *rpcpb.SendTransactionResp, err error) {
	txByte, err := hex.DecodeString(req.Tx)
	if err != nil {
		return newSendTransactionResp(-1, err.Error(), ""), nil
	}
	tx := new(types.Transaction)
	if err = tx.Unmarshal(txByte); err != nil {
		return newSendTransactionResp(-1, err.Error(), ""), nil
	}
	hash, _ := tx.TxHash()
	defer func() {
		if resp.Code != 0 {
			logger.Warnf("send raw tx %s error: %s", hash, resp.Message)
		} else {
			logger.Infof("send raw tx %s succeeded", hash)
		}
	}()
	txpool := s.server.GetTxHandler()
	if err := txpool.ProcessTx(tx, core.BroadcastMode); err != nil {
		if err == core.ErrOrphanTransaction {
			return newSendTransactionResp(0, err.Error(), hash.String()), nil
		}
		return newSendTransactionResp(-1, err.Error(), ""), nil
	}
	return newSendTransactionResp(0, "success", hash.String()), nil
}

func (s *txServer) SendTransaction(
	ctx context.Context, req *rpcpb.SendTransactionReq,
) (resp *rpcpb.SendTransactionResp, err error) {

	tx := new(types.Transaction)
	if err := tx.FromProtoMessage(req.Tx); err != nil {
		return newSendTransactionResp(-1, err.Error(), ""), nil
	}

	hash, _ := tx.TxHash()
	defer func() {

		if resp.Code != 0 {
			logger.Warnf("send tx req: %s error: %s", tolog(types.ConvPbTx(req.GetTx())), resp.Message)
		} else {
			logger.Infof("send tx req: %s succeeded, response: %s", tolog(types.ConvPbTx(req.GetTx())), resp)
		}
	}()
	txpool := s.server.GetTxHandler()
	if err := txpool.ProcessTx(tx, core.BroadcastMode); err != nil {
		if err == core.ErrOrphanTransaction {
			return newSendTransactionResp(0, err.Error(), hash.String()), nil
		}
		return newSendTransactionResp(-1, err.Error(), ""), nil
	}
	return newSendTransactionResp(0, "success", hash.String()), nil
}

func (s *txServer) GetRawTransaction(
	ctx context.Context, req *rpcpb.GetRawTransactionRequest,
) (*rpcpb.GetRawTransactionResponse, error) {
	hash := crypto.HashType{}
	if err := hash.SetBytes(req.Hash); err != nil {
		return &rpcpb.GetRawTransactionResponse{}, err
	}
	_, tx, err := s.server.GetChainReader().LoadBlockInfoByTxHash(hash)
	if err != nil {
		logger.Debug(err)
		return &rpcpb.GetRawTransactionResponse{Code: -1, Message: err.Error()}, nil
	}
	rpcTx, err := tx.ToProtoMessage()
	if err != nil {
		return &rpcpb.GetRawTransactionResponse{Code: -1, Message: err.Error()}, nil
	}
	return &rpcpb.GetRawTransactionResponse{Code: 0, Message: "success", Tx: rpcTx.(*corepb.Transaction)}, nil
}

func newMakeTxResp(
	code int32, msg string, tx *corepb.Transaction, rawMsgs [][]byte,
) *rpcpb.MakeTxResp {
	return &rpcpb.MakeTxResp{
		Code:    code,
		Message: msg,
		Tx:      tx,
		RawMsgs: rawMsgs,
	}
}

func (s *txServer) MakeUnsignedTx(
	ctx context.Context, req *rpcpb.MakeTxReq,
) (resp *rpcpb.MakeTxResp, err error) {

	defer func() {

		if resp.Code != 0 {
			logger.Warnf("make unsigned tx: %s error: %s", tolog(req), resp.Message)
		} else {
			logger.Infof("make unsigned tx: %s succeeded, response: %s", tolog(req), tolog(types.ConvPbTx(resp.GetTx())))
		}
	}()
	wa := s.server.GetWalletAgent()
	if wa == nil || reflect.ValueOf(wa).IsNil() {
		return newMakeTxResp(-1, ErrAPINotSupported.Error(), nil, nil), nil
	}
	from, to := req.GetFrom(), req.GetTo()
	// check address
	if err := types.ValidateAddr(append(to, from)...); err != nil {
		logger.Warn(err)
		return newMakeTxResp(-1, err.Error(), nil, nil), nil
	}
	amounts, gasPrice := req.GetAmounts(), req.GetGasPrice()
	gasUsed := gasPrice * core.TransferGasLimit
	tx, utxos, err := rpcutil.MakeUnsignedTx(wa, from, to, amounts, gasUsed)
	if err != nil {
		return newMakeTxResp(-1, err.Error(), nil, nil), nil
	}
	pbTx, err := tx.ConvToPbTx()
	if err != nil {
		return newMakeTxResp(-1, err.Error(), nil, nil), nil
	}
	rawMsgs, err := MakeTxRawMsgsForSign(tx, utxos...)
	if err != nil {
		return newMakeTxResp(-1, err.Error(), nil, nil), nil
	}
	return newMakeTxResp(0, "", pbTx, rawMsgs), nil
}

func (s *txServer) MakeUnsignedSplitAddrTx(
	ctx context.Context, req *rpcpb.MakeSplitAddrTxReq,
) (resp *rpcpb.MakeTxResp, err error) {

	defer func() {
		if resp.Code != 0 {
			logger.Warnf("make unsigned split addr tx: %s error: %s", tolog(req), resp.Message)
		} else {
			logger.Infof("make unsigned split addr tx: %s succeeded, response: %s", tolog(req), tolog(types.ConvPbTx(resp.GetTx())))
		}
	}()
	wa := s.server.GetWalletAgent()
	if wa == nil || reflect.ValueOf(wa).IsNil() {
		return newMakeTxResp(-1, ErrAPINotSupported.Error(), nil, nil), nil
	}
	from, addrs := req.GetFrom(), req.GetAddrs()
	if err := types.ValidateAddr(append(addrs, from)...); err != nil {
		logger.Warn(err)
		return newMakeTxResp(-1, err.Error(), nil, nil), nil
	}
	weights, gasPrice := req.GetWeights(), req.GasPrice
	gasUsed := gasPrice * core.TransferGasLimit
	// make tx without sign
	tx, utxos, err := rpcutil.MakeUnsignedSplitAddrTx(wa, from, addrs, weights, gasUsed)
	if err != nil {
		return newMakeTxResp(-1, err.Error(), nil, nil), nil
	}
	pbTx, err := tx.ConvToPbTx()
	if err != nil {
		return newMakeTxResp(-1, err.Error(), nil, nil), nil
	}
	// calc raw msgs
	rawMsgs, err := MakeTxRawMsgsForSign(tx, utxos...)
	if err != nil {
		return newMakeTxResp(-1, err.Error(), nil, nil), nil
	}
	return newMakeTxResp(0, "success", pbTx, rawMsgs), nil
}

func newMakeTokenIssueTxResp(code int32, msg string) *rpcpb.MakeTokenIssueTxResp {
	return &rpcpb.MakeTokenIssueTxResp{
		Code:    code,
		Message: msg,
	}
}

func (s *txServer) MakeUnsignedTokenIssueTx(
	ctx context.Context, req *rpcpb.MakeTokenIssueTxReq,
) (resp *rpcpb.MakeTokenIssueTxResp, err error) {
	defer func() {
		if resp.Code != 0 {
			logger.Warnf("make unsigned token issue tx: %s error: %s", tolog(req), resp.Message)
		} else {
			logger.Infof("make unsigned token issue tx: %s succeeded, response: %s", tolog(req), tolog(types.ConvPbTx(resp.GetTx())))
		}
	}()
	if req.GetTag().GetDecimal() > maxDecimal {
		return newMakeTokenIssueTxResp(-1, "The range of decimal must be between 0 and 8"), nil
	}
	if req.Tag.Supply > math.MaxUint64/uint64(math.Pow10(int(req.Tag.Decimal))) {
		return newMakeTokenIssueTxResp(-1, "the value is too bigger"), nil
	}
	wa := s.server.GetWalletAgent()
	if wa == nil || reflect.ValueOf(wa).IsNil() {
		return newMakeTokenIssueTxResp(-1, ErrAPINotSupported.Error()), nil
	}
	issuer, owner, tag, gasPrice := req.GetIssuer(), req.GetOwner(), req.GetTag(), req.GasPrice
	if err := types.ValidateAddr(issuer, owner); err != nil {
		logger.Warn(err)
		return newMakeTokenIssueTxResp(-1, err.Error()), nil
	}
	gasUsed := gasPrice * core.TransferGasLimit
	// make tx without sign
	tx, issueOutIndex, utxos, err := rpcutil.MakeUnsignedTokenIssueTx(wa, issuer,
		owner, tag, gasUsed)
	if err != nil {
		return newMakeTokenIssueTxResp(-1, err.Error()), nil
	}
	pbTx, err := tx.ConvToPbTx()
	if err != nil {
		return newMakeTokenIssueTxResp(-1, err.Error()), nil
	}
	// calc raw msgs
	rawMsgs, err := MakeTxRawMsgsForSign(tx, utxos...)
	if err != nil {
		return newMakeTokenIssueTxResp(-1, err.Error()), nil
	}
	resp = newMakeTokenIssueTxResp(0, "success")
	resp.IssueOutIndex, resp.Tx, resp.RawMsgs = issueOutIndex, pbTx, rawMsgs
	return resp, nil
}

func (s *txServer) MakeUnsignedTokenTransferTx(
	ctx context.Context, req *rpcpb.MakeTokenTransferTxReq,
) (resp *rpcpb.MakeTxResp, err error) {

	defer func() {
		if resp.Code != 0 {
			logger.Warnf("make unsigned token transfer tx: %s error: %s", tolog(req), resp.Message)
		} else {
			logger.Infof("make unsigned token transfer tx: %s succeeded, response: %s", tolog(req), tolog(types.ConvPbTx(resp.GetTx())))
		}
	}()
	wa := s.server.GetWalletAgent()
	if wa == nil || reflect.ValueOf(wa).IsNil() {
		return newMakeTxResp(-1, ErrAPINotSupported.Error(), nil, nil), nil
	}
	from, gasPrice := req.GetFrom(), req.GasPrice
	gasUsed := gasPrice * core.TransferGasLimit
	to, amounts := req.GetTo(), req.GetAmounts()
	if err := types.ValidateAddr(append(to, from)...); err != nil {
		logger.Warn(err)
		return newMakeTxResp(-1, err.Error(), nil, nil), nil
	}
	// parse token id
	tHashStr, tIdx := req.GetTokenHash(), req.GetTokenIndex()
	tHash := new(crypto.HashType)
	if err := tHash.SetString(tHashStr); err != nil {
		logger.Warn(err)
		return newMakeTxResp(-1, err.Error(), nil, nil), nil
	}
	op := types.NewOutPoint(tHash, tIdx)
	//
	tx, utxos, err := rpcutil.MakeUnsignedTokenTransferTx(wa, from, to, amounts,
		(*types.TokenID)(op), gasUsed)
	if err != nil {
		return newMakeTxResp(-1, err.Error(), nil, nil), nil
	}
	pbTx, err := tx.ConvToPbTx()
	if err != nil {
		return newMakeTxResp(-1, err.Error(), nil, nil), nil
	}
	rawMsgs, err := MakeTxRawMsgsForSign(tx, utxos...)
	if err != nil {
		return newMakeTxResp(-1, err.Error(), nil, nil), nil
	}
	return newMakeTxResp(0, "", pbTx, rawMsgs), nil
}

//MakeUnsignedContractTx make contract tx
func (s *txServer) MakeUnsignedContractTx(
	ctx context.Context, req *rpcpb.MakeContractTxReq,
) (resp *rpcpb.MakeContractTxResp, err error) {
	defer func() {
		if resp.Code != 0 {
			logger.Warnf("make unsigned contract tx: %s error: %s", tolog(req), resp.Message)
		} else {
			logger.Debugf("make unsigned contract tx: %s succeeded, response: %s, contract addr: %s",
				tolog(req), tolog(types.ConvPbTx(resp.GetTx())), resp.ContractAddr)
		}
	}()
	wa := s.server.GetWalletAgent()
	from := req.GetFrom()
	amount, gasPrice, gasLimit := req.GetAmount(), req.GetGasPrice(), req.GetGasLimit()
	byteCode, err := hex.DecodeString(req.GetData())
	if err != nil {
		return newMakeContractTxResp(-1, err.Error(), nil, nil, ""), nil
	}
	tx := new(types.Transaction)
	utxos := make([]*rpcpb.Utxo, 0)

	if err := types.ValidateAddr(from); err != nil ||
		!strings.HasPrefix(from, types.AddrTypeP2PKHPrefix) {
		return newMakeContractTxResp(-1, "invalid from address", nil, nil, ""), nil
	}
	contractAddr := req.GetTo()
	if req.IsDeploy {
		if contractAddr != "" {
			eStr := "contract addr must be empty when deploy contract"
			return newMakeContractTxResp(-1, eStr, nil, nil, ""), nil
		}
		fromHash, err := types.NewAddress(from)
		if err != nil {
			return newMakeContractTxResp(-1, err.Error(), nil, nil, ""), nil
		}
		nonce := s.server.GetChainReader().TailState().GetNonce(*fromHash.Hash160())
		if req.GetNonce() <= nonce {
			eStr := fmt.Sprintf("mismatch nonce(%d, %d on chain)", req.GetNonce(), nonce)
			return newMakeContractTxResp(-1, eStr, nil, nil, ""), nil
		}
		contractAddress, _ := types.MakeContractAddress(fromHash, req.GetNonce())
		tx, utxos, err = rpcutil.MakeUnsignedContractDeployTx(wa, from, amount,
			gasLimit, gasPrice, req.GetNonce(), byteCode)
		if err != nil {
			return newMakeContractTxResp(-1, err.Error(), nil, nil, ""), err
		}
		contractAddr = contractAddress.String()
	} else {
		if err := types.ValidateAddr(); err != nil ||
			!strings.HasPrefix(contractAddr, types.AddrTypeContractPrefix) {
			return newMakeContractTxResp(-1, "invalid contract address", nil, nil, ""), nil
		}
		tx, utxos, err = rpcutil.MakeUnsignedContractCallTx(wa, from, amount,
			gasLimit, gasPrice, req.GetNonce(), contractAddr, byteCode)
		if err != nil {
			return newMakeContractTxResp(-1, err.Error(), nil, nil, ""), err
		}
		contractAddr = ""
	}
	pbTx, err := tx.ConvToPbTx()
	if err != nil {
		return newMakeContractTxResp(-1, err.Error(), nil, nil, ""), err
	}
	rawMsgs, err := MakeTxRawMsgsForSign(tx, utxos...)
	if err != nil {
		return newMakeContractTxResp(-1, err.Error(), nil, nil, ""), err
	}
	return newMakeContractTxResp(0, "success", pbTx, rawMsgs, contractAddr), nil
}

func newMakeContractTxResp(
	code int32, msg string, tx *corepb.Transaction, rawMsgs [][]byte, contractAddr string,
) *rpcpb.MakeContractTxResp {
	return &rpcpb.MakeContractTxResp{
		Code:         code,
		Message:      msg,
		Tx:           tx,
		RawMsgs:      rawMsgs,
		ContractAddr: contractAddr,
	}
}

func (s *txServer) MakeUnsignedCombineTx(
	ctx context.Context, req *rpcpb.MakeCombineTx,
) (resp *rpcpb.MakeTxResp, err error) {

	defer func() {
		if resp.Code != 0 {
			logger.Warnf("make unsigned combine tx: %s error: %s", tolog(req), resp.Message)
		} else {
			logger.Infof("make unsigned combine tx: %s succeeded, response: %s", tolog(req), tolog(types.ConvPbTx(resp.GetTx())))
		}
	}()
	wa := s.server.GetWalletAgent()
	if wa == nil || reflect.ValueOf(wa).IsNil() {
		return newMakeTxResp(-1, ErrAPINotSupported.Error(), nil, nil), nil
	}
	from := req.GetAddr()
	// check address
	if err := types.ValidateAddr(from); err != nil {
		logger.Warn(err)
		return newMakeTxResp(-1, err.Error(), nil, nil), nil
	}
	// gasUsed
	gasPrice := req.GetGasPrice()
	gasUsed := gasPrice * core.TransferGasLimit
	//
	var (
		tx    *types.Transaction
		utxos []*rpcpb.Utxo
	)
	// make tx
	tokenHashStr, tokenIdx := req.GetTokenHash(), req.GetTokenIndex()
	if tokenHashStr != "" {
		tokenHash := new(crypto.HashType)
		if err := tokenHash.SetString(tokenHashStr); err != nil {
			logger.Warn("make unsigned combine tx error, invalid token hash %s", req.GetTokenHash())
			return newMakeTxResp(-1, "invalid token hash", nil, nil), nil
		}
		tid := (*types.TokenID)(types.NewOutPoint(tokenHash, tokenIdx))
		tx, utxos, err = rpcutil.MakeUnsignedCombineTokenTx(wa, from, tid, gasUsed)
	} else {
		tx, utxos, err = rpcutil.MakeUnsignedCombineTx(wa, from, gasUsed)
	}
	if err != nil {
		return newMakeTxResp(-1, err.Error(), nil, nil), nil
	}
	pbTx, err := tx.ConvToPbTx()
	if err != nil {
		return newMakeTxResp(-1, err.Error(), nil, nil), nil
	}
	// raw messages
	rawMsgs, err := MakeTxRawMsgsForSign(tx, utxos...)
	if err != nil {
		return newMakeTxResp(-1, err.Error(), nil, nil), nil
	}
	return newMakeTxResp(0, "", pbTx, rawMsgs), nil
}

// MakeTxRawMsgsForSign make tx raw msg for sign
func MakeTxRawMsgsForSign(tx *types.Transaction, utxos ...*rpcpb.Utxo) ([][]byte, error) {
	if len(tx.Vin) != len(utxos) {
		return nil, errors.New("invalid param")
	}
	msgs := make([][]byte, 0, len(tx.Vin))
	for i, u := range utxos {
		spk := u.GetTxOut().GetScriptPubKey()
		newTx := tx.Copy()
		for j, in := range newTx.Vin {
			if i != j {
				in.ScriptSig = nil
			} else {
				in.ScriptSig = spk
			}
		}
		data, err := newTx.Marshal()
		if err != nil {
			return nil, err
		}
		msgs = append(msgs, data)
	}
	return msgs, nil
}

func tolog(v interface{}) string {
	bytes, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	if len(bytes) > 4096 {
		bytes = bytes[:4096]
	}
	return string(bytes)
}
