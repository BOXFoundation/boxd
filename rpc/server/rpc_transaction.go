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
	logger.Infof("get balance req: %+v", req)
	walletAgent := s.server.GetWalletAgent()
	if walletAgent == nil || reflect.ValueOf(walletAgent).IsNil() {
		logger.Warn("get balance error ", ErrAPINotSupported)
		return newGetBalanceResp(-1, ErrAPINotSupported.Error()), ErrAPINotSupported
	}
	balances := make([]uint64, len(req.GetAddrs()))
	for i, addr := range req.Addrs {
		if err := types.ValidateAddr(addr); err != nil {
			logger.Warn(err)
			return newGetBalanceResp(-1, err.Error()), nil
		}
		amount, err := walletAgent.Balance(addr, nil)
		if err != nil {
			logger.Warnf("get balance for %s error %s", addr, err)
			return newGetBalanceResp(-1, err.Error()), nil
		}
		balances[i] = amount
	}
	logger.Infof("get balance for %v result: %v", req.Addrs, balances)
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
	op := txlogic.NewPbOutPoint(thash, req.GetTokenIndex())
	tid := (*types.TokenID)(txlogic.ConvPbOutPoint(op))
	// valide addrs
	if err := types.ValidateAddr(req.Addrs...); err != nil {
		logger.Warn(err)
		return newGetBalanceResp(-1, err.Error()), nil
	}
	for i, addr := range req.Addrs {
		amount, err := walletAgent.Balance(addr, tid)
		if err != nil {
			logger.Warnf("get token balance for %s error %s", addr, err)
			return newGetBalanceResp(-1, err.Error()), nil
		}
		balances[i] = amount
	}
	logger.Infof("get balance for %v result: %v", req.Addrs, balances)
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
		bytes, _ := json.Marshal(req)
		if resp.Code != 0 {
			logger.Warnf("fetch utxos: %s error: %s", string(bytes), resp.Message)
		} else {
			total := uint64(0)
			for _, u := range resp.GetUtxos() {
				amount, _, err := txlogic.ParseUtxoAmount(u)
				if err != nil {
					logger.Warnf("fetch utxos for %+v error %s", req, err)
					continue
				}
				total += amount
			}
			//logger.Infof("fetch utxos: %s succeeded, return %d utxos total %d",
			//	string(bytes), len(resp.GetUtxos()), total)
		}
	}()

	walletAgent := s.server.GetWalletAgent()
	if walletAgent == nil || reflect.ValueOf(walletAgent).IsNil() {
		logger.Warn("fetch utxos error ", ErrAPINotSupported)
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
		logger.Warn(err)
		return newFetchUtxosResp(-1, err.Error()), nil
	}
	utxos, err := walletAgent.Utxos(addr, tid, req.GetAmount())
	if err != nil {
		logger.Warnf("fetch utxos for %+v error %s", req, err)
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

	defer func() {
		bytes, _ := json.Marshal(req)
		if resp.Code != 0 {
			logger.Warnf("send tx req: %s error: %s", string(bytes), resp.Message)
		} else {
			logger.Infof("send tx req: %s succeeded, response: %+v", string(bytes), resp)
		}
	}()

	tx := new(types.Transaction)
	if err := tx.FromProtoMessage(req.Tx); err != nil {
		return newSendTransactionResp(-1, err.Error(), ""), nil
	}

	hash, _ := tx.TxHash()
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
		return &rpcpb.GetRawTransactionResponse{}, err
	}
	rpcTx, err := tx.ToProtoMessage()
	return &rpcpb.GetRawTransactionResponse{Tx: rpcTx.(*corepb.Transaction)}, err
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
		bytes, _ := json.Marshal(req)
		if resp.Code != 0 {
			logger.Warnf("make unsigned tx: %s error: %s", string(bytes), resp.Message)
		} else {
			logger.Infof("make unsigned tx: %s succeeded, response: %+v", string(bytes), resp)
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
	amounts, gasPrice := req.GetAmounts(), req.GasPrice
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

func newMakeSplitAddrTxResp(code int32, msg string) *rpcpb.MakeSplitAddrTxResp {
	return &rpcpb.MakeSplitAddrTxResp{
		Code:    code,
		Message: msg,
	}
}

func (s *txServer) MakeUnsignedSplitAddrTx(
	ctx context.Context, req *rpcpb.MakeSplitAddrTxReq,
) (resp *rpcpb.MakeSplitAddrTxResp, err error) {

	defer func() {
		bytes, _ := json.Marshal(req)
		if resp.Code != 0 {
			logger.Warnf("make unsigned split addr tx: %s error: %s", string(bytes), resp.Message)
		} else {
			logger.Infof("make unsigned split addr tx: %s succeeded, response: %+v",
				string(bytes), resp)
		}
	}()
	//
	wa := s.server.GetWalletAgent()
	if wa == nil || reflect.ValueOf(wa).IsNil() {
		return newMakeSplitAddrTxResp(-1, ErrAPINotSupported.Error()), nil
	}
	from, addrs := req.GetFrom(), req.GetAddrs()
	if err := types.ValidateAddr(append(addrs, from)...); err != nil {
		logger.Warn(err)
		return newMakeSplitAddrTxResp(-1, err.Error()), nil
	}
	weights, gasPrice := req.GetWeights(), req.GasPrice
	gasUsed := gasPrice * core.TransferGasLimit
	// make tx without sign
	tx, splitAddr, utxos, err := rpcutil.MakeUnsignedSplitAddrTx(wa, from, addrs,
		weights, gasUsed)
	if err != nil {
		return newMakeSplitAddrTxResp(-1, err.Error()), nil
	}
	pbTx, err := tx.ConvToPbTx()
	if err != nil {
		return newMakeSplitAddrTxResp(-1, err.Error()), nil
	}
	// calc raw msgs
	rawMsgs, err := MakeTxRawMsgsForSign(tx, utxos...)
	if err != nil {
		return newMakeSplitAddrTxResp(-1, err.Error()), nil
	}
	resp = newMakeSplitAddrTxResp(0, "success")
	resp.SplitAddr, resp.Tx, resp.RawMsgs = splitAddr, pbTx, rawMsgs
	return resp, nil
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
		bytes, _ := json.Marshal(req)
		if resp.Code != 0 {
			logger.Warnf("make unsigned token issue tx: %s error: %s", string(bytes), resp.Message)
		} else {
			logger.Infof("make unsigned token issue tx: %s succeeded, response: %+v",
				string(bytes), resp)
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
		bytes, _ := json.Marshal(req)
		if resp.Code != 0 {
			logger.Warnf("make unsigned token transfer tx: %s error: %s", string(bytes),
				resp.Message)
		} else {
			logger.Infof("make unsigned token transfer tx: %s succeeded, response: %+v",
				string(bytes), resp)
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
		bytes, _ := json.Marshal(req)
		if resp.Code != 0 {
			logger.Warnf("make unsigned contract tx: %s error: %s", string(bytes), resp.Message)
		} else {
			logger.Debugf("make unsigned contract tx: %s succeeded, contract addr: %s",
				string(bytes), resp.ContractAddr)
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
		nonce, _ := s.server.GetChainReader().GetLatestNonce(fromHash.Hash160())
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
