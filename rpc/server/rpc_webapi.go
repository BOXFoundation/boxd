// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package rpc

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"math/big"
	"strings"
	"time"

	"github.com/BOXFoundation/boxd/core"
	"github.com/BOXFoundation/boxd/core/abi"
	"github.com/BOXFoundation/boxd/core/chain"
	"github.com/BOXFoundation/boxd/core/txlogic"
	"github.com/BOXFoundation/boxd/core/types"
	state "github.com/BOXFoundation/boxd/core/worldstate"
	"github.com/BOXFoundation/boxd/crypto"
	rpcpb "github.com/BOXFoundation/boxd/rpc/pb"
	"github.com/BOXFoundation/boxd/script"
	"github.com/BOXFoundation/boxd/vm"
	"github.com/jbenet/goprocess"
	uuid "github.com/satori/go.uuid"
)

type webapiServer struct {
	ChainBlockReader
	TxPoolReader
	TableReader
	proc goprocess.Process

	endpoints      map[uint32]Endpoint
	connController map[string]map[string]chan<- bool
}

// ChainTxReader defines chain tx reader interface
type ChainTxReader interface {
	LoadBlockInfoByTxHash(crypto.HashType) (*types.Block, *types.Transaction, types.TxType, error)
	GetDataFromDB([]byte) ([]byte, error)
	GetTxReceipt(*crypto.HashType) (*types.Receipt, *types.Transaction, error)
}

// ChainBlockReader defines chain block reader interface
type ChainBlockReader interface {
	ChainTxReader
	// LoadBlockInfoByTxHash(crypto.HashType) (*types.Block, *types.Transaction, error)
	ReadBlockFromDB(*crypto.HashType) (*types.Block, int, error)
	EternalBlock() *types.Block
	NewEvmContextForLocalCallByHeight(msg types.Message, height uint32) (*vm.EVM, func() error, error)
	GetStateDbByHeight(height uint32) (*state.StateDB, error)
	TailBlock() *types.Block
	GetLogs(from, to uint32, topicslist [][][]byte) ([]*types.Log, error)
	FilterLogs(logs []*types.Log, topicslist [][][]byte) ([]*types.Log, error)
	TailState() *state.StateDB
}

// TxPoolReader defines tx pool reader interface
type TxPoolReader interface {
	GetTxByHash(hash *crypto.HashType) (*types.TxWrap, bool)
}

// TableReader defines basic operations routing table exposes
type TableReader interface {
	ConnectingPeers() []string
	PeerID() string
	Miners() []*types.PeerInfo
}

func newWebAPIServer(s *Server) *webapiServer {
	server := &webapiServer{
		ChainBlockReader: s.GetChainReader(),
		TxPoolReader:     s.GetTxHandler(),
		TableReader:      s.GetTableReader(),
		proc:             s.Proc(),
		endpoints:        make(map[uint32]Endpoint),
		connController:   make(map[string]map[string]chan<- bool),
	}

	if s.cfg.SubScribeBlocks {
		server.endpoints[BlockEp] = NewBlockEndpoint(s.eventBus, s.GetChainReader(), s.GetTxHandler())
	}
	if s.cfg.SubScribeLogs {
		server.endpoints[LogEp] = NewLogEndpoint(s.eventBus)
	}

	return server
}

func (s *webapiServer) Closing() bool {
	select {
	case <-s.proc.Closing():
		return true
	default:
		return false
	}
}

func (s *webapiServer) CloseConn(uid string) {
	if subs, ok := s.connController[uid]; ok {
		for _, ch := range subs {
			if ch != nil {
				ch <- true
			}
		}
		s.connController[uid] = nil
		delete(s.connController, uid)
	}
}

func (s *webapiServer) subscribe(uids ...string) (string, chan bool, error) {
	ch := make(chan bool, 1)
	if len(uids) < 1 {
		return "", nil, fmt.Errorf("Invalid uids")
	}
	connuid := uids[0]
	var uid string

	if len(uids) > 1 {
		uid = uids[1]
	} else {
		uid = uuid.NewV1().String()
	}

	if subs, ok := s.connController[connuid]; ok {
		subs[uid] = chan<- bool(ch)
	} else {
		subs = make(map[string]chan<- bool)
		subs[uid] = ch
		s.connController[connuid] = subs
	}
	return uid, ch, nil
}

func (s *webapiServer) Unsubscribe(connuid, uid string) {
	logger.Debugf("Unsubscribe webapiServer connuid: %v, uid: %v", connuid, uid)
	if subs, ok := s.connController[connuid]; ok {
		if ch, ok := subs[uid]; ok {
			ch <- true
		}
		delete(subs, uid)
		if len(subs) == 0 {
			delete(s.connController, connuid)
		}
	}
}

func newViewTxDetailResp(code int32, msg string) *rpcpb.ViewTxDetailResp {
	return &rpcpb.ViewTxDetailResp{
		Code:    code,
		Message: msg,
	}
}

func (s *webapiServer) ViewTxDetail(
	ctx context.Context, req *rpcpb.ViewTxDetailReq,
) (*rpcpb.ViewTxDetailResp, error) {

	logger.Infof("view tx detail req: %+v", req)
	// fetch hash from request
	hash := new(crypto.HashType)
	if err := hash.SetString(req.Hash); err != nil {
		logger.Warn("view tx detail error: ", err)
		return newViewTxDetailResp(-1, err.Error()), nil
	}
	// new resp
	resp := new(rpcpb.ViewTxDetailResp)
	// fetch tx from chain and set status
	br, tr := s.ChainBlockReader, s.TxPoolReader
	block, tx, txType, err := br.LoadBlockInfoByTxHash(*hash)
	if err == nil {
		// calc tx status
		if blockConfirmed(block, br) {
			resp.Status = rpcpb.TxStatus_confirmed
		} else {
			resp.Status = rpcpb.TxStatus_onchain
		}
		resp.BlockTime = block.Header.TimeStamp
		resp.BlockHeight = block.Header.Height
	} else {
		logger.Warnf("view tx detail load block by tx hash %s error: %s,"+
			" try get it from tx pool", hash, err)
		txWrap, _ := tr.GetTxByHash(hash)
		if txWrap == nil {
			return newViewTxDetailResp(-1, "tx not found"), nil
		}
		tx = txWrap.Tx
		resp.Status = rpcpb.TxStatus_pending
		resp.BlockTime = txWrap.AddedTimestamp
		resp.BlockHeight = txWrap.Height
	}
	resp.Version = tx.Version
	// fetch tx details
	detail, err := detailTx(tx, br, tr, req.GetSpreadSplit(), true)
	if err != nil {
		logger.Warn("view tx detail error: ", err)
		return newViewTxDetailResp(-1, err.Error()), nil
	}
	if txType == types.InternalTxType {
		detail.Fee = 0
	}
	//
	resp.Detail = detail
	return resp, nil
}

func newViewBlockDetailResp(code int32, msg string) *rpcpb.ViewBlockDetailResp {
	return &rpcpb.ViewBlockDetailResp{
		Code:    code,
		Message: msg,
	}
}

func (s *webapiServer) ViewBlockDetail(
	ctx context.Context, req *rpcpb.ViewBlockDetailReq,
) (*rpcpb.ViewBlockDetailResp, error) {

	logger.Infof("view block detail req: %+v", req)
	hash := new(crypto.HashType)
	if err := hash.SetString(req.Hash); err != nil {
		logger.Warn("view block detail error: ", err)
		return newViewBlockDetailResp(-1, err.Error()), nil
	}

	br, tr := s.ChainBlockReader, s.TxPoolReader
	block, n, err := br.ReadBlockFromDB(hash)
	if err != nil {
		logger.Warn("view block detail error: ", err)
		return newViewBlockDetailResp(-1, err.Error()), nil
	}
	detail, err := detailBlock(block, br, tr, true)
	if err != nil {
		logger.Warn("view block detail error: ", err)
		return newViewBlockDetailResp(-1, err.Error()), nil
	}
	resp := newViewBlockDetailResp(0, "")
	resp.Detail = detail
	resp.Detail.Size_ = uint32(n)
	return resp, nil
}

func newCallResp(code int32, msg string, output string, height uint64) *rpcpb.CallResp {
	return &rpcpb.CallResp{
		Code:    code,
		Message: msg,
		Output:  output,
		Height:  int32(height),
	}
}

func (s *webapiServer) DoCall(
	ctx context.Context, req *rpcpb.CallReq,
) (resp *rpcpb.CallResp, err error) {

	output, _, height, err := callContract(req.GetFrom(), req.GetTo(), req.GetData(),
		req.GetHeight(), s.ChainBlockReader)
	if err != nil {
		vmerrmsg, err2 := abi.UnpackErrMsg(output)
		if err2 != nil {
			logger.Debugf("UnpackErrMsg failed. Err: %v", err2)
		}
		return newCallResp(-1, err.Error(), vmerrmsg, height), nil
	}

	// Make sure we have a contract to operate on, and bail out otherwise.
	if err == nil && len(output) == 0 {
		contractAddr, _ := types.ParseAddress(req.To)
		conHash := contractAddr.Hash160()
		// Make sure we have a contract to operate on, and bail out otherwise.
		statedb, err := s.ChainBlockReader.GetStateDbByHeight(req.GetHeight())
		if err != nil {
			return newCallResp(-1, err.Error(), "", height), nil
		}
		if !statedb.Exist(*conHash) {
			return newCallResp(-1, core.ErrContractNotFound.Error(), "", height), nil
		}
	}

	return newCallResp(0, "ok", hex.EncodeToString(output), height), nil
}

func callContract(
	from, to, data string, height uint32, chainReader ChainBlockReader,
) (ret []byte, usedGas uint64, h uint64, err error) {

	fromHash, err := types.ParseAddress(from)
	if err != nil || !strings.HasPrefix(from, types.AddrTypeP2PKHPrefix) {
		return nil, 0, 0, errors.New("invalid from address")
	}
	contractAddr, err := types.ParseAddress(to)
	if err != nil || !strings.HasPrefix(to, types.AddrTypeContractPrefix) {
		return nil, 0, 0, errors.New("invalid contract address")

	}
	input, err := hex.DecodeString(data)
	if err != nil || len(data) == 0 {
		return nil, 0, 0, errors.New("invalid contract data")
	}

	msg := types.NewVMTransaction(new(big.Int), math.MaxUint64/2, 0, 0, nil,
		types.ContractCallType, input).
		WithFrom(fromHash.Hash160()).WithTo(contractAddr.Hash160())

	// Setup context so it may be cancelled the call has completed
	// or, in case of unmetered gas, setup a context with a timeout.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	// Make sure the context is cancelled when the call has completed
	// this makes sure resources are cleaned up.
	defer cancel()

	// Get a new instance of the EVM.
	evm, dbErr, err := chainReader.NewEvmContextForLocalCallByHeight(msg, height)
	if err != nil {
		return
	}
	// Wait for the context to be done and cancel the evm. Even if the
	// EVM has finished, cancelling may be done (repeatedly)
	go func() {
		<-ctx.Done()
		evm.Cancel()
	}()

	// Setup the gas pool (also for unmetered requests)
	// and apply the message.
	failed := false
	ret, usedGas, _, failed, _, err = chain.ApplyMessage(evm, msg)
	if err != nil {
		return nil, 0, evm.Context.BlockNumber.Uint64(), err
	}
	if failed {
		return ret, usedGas, evm.Context.BlockNumber.Uint64(), fmt.Errorf("contract execution failed, db error: %v", dbErr())
	}
	if dbErr() != nil {
		return nil, 0, evm.Context.BlockNumber.Uint64(), dbErr()
	}
	return ret, usedGas, evm.Context.BlockNumber.Uint64(), nil
}

func newNonceResp(code int32, msg string, nonce uint64) *rpcpb.NonceResp {
	return &rpcpb.NonceResp{
		Code:    code,
		Message: msg,
		Nonce:   nonce,
	}
}

func (s *webapiServer) Nonce(
	ctx context.Context, req *rpcpb.NonceReq,
) (*rpcpb.NonceResp, error) {

	address, err := types.ParseAddress(req.GetAddr())
	if err != nil {
		return newNonceResp(-1, err.Error(), 0), nil
	}
	switch address.(type) {
	default:
		return newNonceResp(-1, "only allow eoa and contract address", 0), nil
	case *types.AddressContract, *types.AddressPubKeyHash:
	}
	return newNonceResp(0, "", s.TailState().GetNonce(*address.Hash160())), nil
}

func newLogsResp(code int32, msg string, logs []*types.Log) *rpcpb.Logs {

	return &rpcpb.Logs{
		Code:    code,
		Message: msg,
		Logs:    ToPbLogs(logs),
	}
}

func (s *webapiServer) GetLogs(ctx context.Context, req *rpcpb.LogsReq) (logs *rpcpb.Logs, err error) {

	from, to := req.From, req.To

	if len(req.Hash) != 0 {
		hash, err := hex.DecodeString(req.Hash)
		if err != nil {
			return newLogsResp(-1, err.Error(), nil), nil
		}
		hh := crypto.BytesToHash(hash)
		b, _, err := s.ChainBlockReader.ReadBlockFromDB(&hh)
		if err != nil {
			return newLogsResp(-1, err.Error(), nil), nil
		}
		from, to = b.Header.Height, b.Header.Height
	} else {
		tail := s.ChainBlockReader.TailBlock()
		if from > tail.Header.Height {
			return newLogsResp(0, "", nil), nil
		}
		if to > tail.Header.Height {
			to = tail.Header.Height
		}
		if from > to {
			return newLogsResp(-1, fmt.Sprint("From not allowed to be greater than To"), nil), nil
		}
	}

	topicslist := [][][]byte{[][]byte{}}
	for _, addr := range req.Addresses {

		contractAddress, err := types.NewContractAddress(addr)
		if err != nil {
			return newLogsResp(-1, err.Error(), nil), nil
		}
		topicslist[0] = append(topicslist[0], contractAddress.Hash())
	}

	for i, topiclist := range req.Topics {
		topicslist = append(topicslist, [][]byte{})
		for _, topic := range topiclist.Topics {
			t, err := hex.DecodeString(topic)
			if err != nil {
				return newLogsResp(-1, err.Error(), nil), nil
			}
			topicslist[i+1] = append(topicslist[i+1], t)
		}
	}

	blocklogs, err := s.ChainBlockReader.GetLogs(from, to, topicslist)
	if err != nil {
		return newLogsResp(-1, err.Error(), nil), nil
	}
	return newLogsResp(0, "", blocklogs), nil
}

func detailTx(
	tx *types.Transaction, br ChainTxReader, tr TxPoolReader, spread bool, detailVin bool,
) (*rpcpb.TxDetail, error) {

	detail := new(rpcpb.TxDetail)
	needSpread := false
	// parse vout
	txHash, _ := tx.TxHash()
	var totalFee uint64
	for i := range tx.Vout {
		outDetail, fee, err := detailTxOut(txHash, tx.Vout[i], uint32(i), tx.Data, br)
		if err != nil {
			logger.Warnf("detail tx vout for %s %d %+v error: %s", txHash, i, tx.Vout[i], err)
			return nil, err
		}
		if strings.HasPrefix(outDetail.Addr, types.AddrTypeSplitAddrPrefix) &&
			outDetail.Type != rpcpb.TxOutDetail_new_split_addr {
			needSpread = true
			if spread {
				break
			}
		}
		totalFee += fee
		detail.Vout = append(detail.Vout, outDetail)
	}
	if spread && needSpread {
		logger.Infof("spread split addr original tx: %s", txHash)
		buf, err := br.GetDataFromDB(chain.SplitTxHashKey(txHash))
		if err != nil {
			return nil, err
		}
		if len(buf) == 0 {
			return nil, fmt.Errorf("split tx for %s not found in db", txHash)
		}
		tx = new(types.Transaction)
		if err := tx.Unmarshal(buf); err != nil {
			return nil, err
		}
		detail.Vout = nil
		hash, _ := tx.TxHash()
		logger.Infof("spread split addr tx: %s", hash)
		for i := range tx.Vout {
			outDetail, _, err := detailTxOut(hash, tx.Vout[i], uint32(i), tx.Data, br)
			if err != nil {
				logger.Warnf("detail split tx vout for %s %d %+v error: %s", hash, i, tx.Vout[i], err)
				return nil, err
			}
			detail.Vout = append(detail.Vout, outDetail)
		}
	}
	// hash
	detail.Hash = txHash.String()
	// parse vin
	for i, in := range tx.Vin {
		inDetail, err := detailTxIn(in, br, tr, detailVin)
		if err != nil {
			logger.Warnf("detail tx vin for %s %d %+v error: %s", txHash, i, in, err)
			return nil, err
		}
		detail.Vin = append(detail.Vin, inDetail)
	}
	// calc fee
	if totalFee == 0 && tx.Vin[0].PrevOutPoint.Hash != crypto.ZeroHash {
		// non-contract tx
		totalFee = core.TransferFee
	}
	detail.Fee = totalFee + tx.ExtraFee()
	return detail, nil
}

func detailBlock(
	block *types.Block, r ChainBlockReader, tr TxPoolReader, detailVin bool,
) (*rpcpb.BlockDetail, error) {

	if block == nil || block.Header == nil {
		return nil, fmt.Errorf("detailBlock error for block: %+v", block)
	}
	detail := new(rpcpb.BlockDetail)
	detail.Version = block.Header.Version
	detail.Height = block.Header.Height
	detail.TimeStamp = block.Header.TimeStamp
	detail.Hash = block.BlockHash().String()
	detail.PrevBlockHash = block.Header.PrevBlockHash.String()
	coinBase, err := types.NewAddressPubKeyHash(block.Header.BookKeeper[:])
	if err != nil {
		return nil, err
	}
	detail.CoinBase = coinBase.String()
	detail.Confirmed = r.EternalBlock().Header.Height >= block.Header.Height
	detail.Signature = hex.EncodeToString(block.Signature)
	for _, tx := range block.Txs {
		txDetail, err := detailTx(tx, r, tr, false, detailVin)
		if err != nil {
			hash, _ := tx.TxHash()
			logger.Warnf("detail tx %s error: %s", hash, err)
			return nil, err
		}
		detail.Txs = append(detail.Txs, txDetail)
	}
	for _, tx := range block.InternalTxs {
		// internal tx have no tx in detail since tx in is from contract utxo
		txDetail, err := detailTx(tx, r, tr, false, false)
		if err != nil {
			hash, _ := tx.TxHash()
			logger.Warnf("detail tx %s error: %s", hash, err)
			return nil, err
		}
		txDetail.Fee = 0 // fee is 0 in internal txs
		detail.InternalTxs = append(detail.InternalTxs, txDetail)
	}
	return detail, nil
}

func detailTxIn(
	txIn *types.TxIn, r ChainTxReader, tr TxPoolReader, detailVin bool,
) (*rpcpb.TxInDetail, error) {

	detail := new(rpcpb.TxInDetail)
	detail.ScriptSig = hex.EncodeToString(txIn.ScriptSig)
	detail.Sequence = txIn.Sequence
	detail.PrevOutPoint = txlogic.EncodeOutPoint(txlogic.ConvOutPoint(&txIn.PrevOutPoint))
	// if tx in is coin base
	if txIn.PrevOutPoint.Hash == crypto.ZeroHash {
		return detail, nil
	}

	if detailVin {
		hash := &txIn.PrevOutPoint.Hash
		_, prevTx, _, err := r.LoadBlockInfoByTxHash(*hash)
		if err != nil {
			logger.Infof("load tx by hash %s from chain error: %s, try tx pool", hash, err)
			txWrap, _ := tr.GetTxByHash(hash)
			if txWrap == nil {
				return nil, fmt.Errorf("tx not found for detail txIn %+v", txIn.PrevOutPoint)
			}
			prevTx = txWrap.Tx
		}
		index := txIn.PrevOutPoint.Index
		prevTxHash, _ := prevTx.TxHash()
		detail.PrevOutDetail, _, err = detailTxOut(prevTxHash, prevTx.Vout[index],
			index, prevTx.Data, r)
		if err != nil {
			logger.Warnf("detail prev tx vout for %s %d %+v error: %s", prevTxHash,
				index, prevTx.Vout[index], err)
			return nil, err
		}
	} else if script.IsContractSig(txIn.ScriptSig) { // internal contract tx
		contractAddr, err := types.NewContractAddressFromHash(
			types.NewAddressHash(txIn.PrevOutPoint.Hash[:])[:])
		if err != nil {
			return nil, fmt.Errorf("new contract address from PrevOutpoint %s error: %s",
				txIn.PrevOutPoint.Hash, err)
		}
		detail.PrevOutDetail = &rpcpb.TxOutDetail{
			Addr: contractAddr.String(),
			Type: rpcpb.TxOutDetail_unknown,
		}
	}
	return detail, nil
}

func detailTxOut(
	txHash *crypto.HashType, txOut *types.TxOut, index uint32, data *types.Data,
	br ChainTxReader,
) (*rpcpb.TxOutDetail, uint64, error) {

	detail := new(rpcpb.TxOutDetail)
	sc := script.NewScriptFromBytes(txOut.ScriptPubKey)
	// addr
	addr, err := ParseAddrFrom(sc, txHash, index)
	if err != nil {
		return nil, 0, err
	}
	detail.Addr = addr
	// value
	op := types.NewOutPoint(txHash, index)
	// height 0 is unused
	wrap := types.NewUtxoWrap(txOut.Value, txOut.ScriptPubKey, 0)
	utxo := txlogic.MakePbUtxo(op, wrap)
	amount, _, err := txlogic.ParseUtxoAmount(utxo)
	if err != nil {
		return nil, 0, err
	}
	detail.Value = amount
	// script disasm
	detail.ScriptDisasm = sc.Disasm()
	// type
	detail.Type = parseVoutType(txOut)
	var fee uint64
	//  appendix
	switch detail.Type {
	default:
	case rpcpb.TxOutDetail_token_issue:
		param, err := sc.GetIssueParams()
		if err != nil {
			return nil, 0, err
		}
		issueInfo := &rpcpb.TxOutDetail_TokenIssueInfo{
			TokenIssueInfo: &rpcpb.TokenIssueInfo{
				TokenTag: &rpcpb.TokenTag{
					Name:    param.Name,
					Symbol:  param.Symbol,
					Supply:  param.TotalSupply,
					Decimal: uint32(param.Decimals),
				},
			},
		}
		detail.Appendix = issueInfo
	case rpcpb.TxOutDetail_token_transfer:
		param, err := sc.GetTransferParams()
		if err != nil {
			return nil, 0, err
		}
		transferInfo := &rpcpb.TxOutDetail_TokenTransferInfo{
			TokenTransferInfo: &rpcpb.TokenTransferInfo{
				TokenId: txlogic.EncodeOutPoint(txlogic.ConvOutPoint(&param.TokenID.OutPoint)),
			},
		}
		detail.Appendix = transferInfo
	case rpcpb.TxOutDetail_new_split_addr:
		addresses, weights, err := sc.ParseSplitAddrScript()
		if err != nil {
			return nil, 0, err
		}
		addrs := make([]string, 0, len(addresses))
		for _, a := range addresses {
			addrs = append(addrs, a.String())
		}
		splitContractInfo := &rpcpb.TxOutDetail_SplitContractInfo{
			SplitContractInfo: &rpcpb.SplitContractInfo{
				Addrs:   addrs,
				Weights: weights,
			},
		}
		detail.Appendix = splitContractInfo
	case rpcpb.TxOutDetail_contract_call:
		var content []byte
		if data != nil {
			content = data.Content
		}
		sc := script.NewScriptFromBytes(txOut.ScriptPubKey)
		params, typ, err := sc.ParseContractParams()
		if err != nil {
			return nil, 0, err
		}
		failed, gasUsed := false, uint64(0)
		var internalTxs []string
		var logs []*rpcpb.LogDetail
		var errMsg string
		if content != nil { // not an internal contract tx
			receipt, tx, err := br.GetTxReceipt(txHash)
			if err != nil {
				logger.Warnf("get receipt for %s error: %s", txHash, err)
				// not to return error to make the status of contract transactions
				// that is in txpool is pending value
			}
			// receipt may be nil if the contract tx is not brought on chain
			if receipt != nil {
				fee, failed, gasUsed, errMsg = receipt.GasUsed*core.FixedGasPrice,
					receipt.Failed, receipt.GasUsed, hex.EncodeToString(receipt.ErrMsg)
				if tx.Vin[0].PrevOutPoint.Hash == crypto.ZeroHash {
					// this contraction is internal chain transaction without fee
					fee = 0
				}
				for _, h := range receipt.InternalTxs {
					internalTxs = append(internalTxs, h.String())
				}
				logs = ToPbLogs(receipt.Logs)
			}
		}
		contractInfo := &rpcpb.TxOutDetail_ContractInfo{
			ContractInfo: &rpcpb.ContractInfo{
				Failed:      failed,
				GasLimit:    params.GasLimit,
				GasUsed:     gasUsed,
				Nonce:       params.Nonce,
				Data:        hex.EncodeToString(content),
				InternalTxs: internalTxs,
				Logs:        logs,
				ErrMsg:      errMsg,
			},
		}
		if typ == types.ContractCreationType {
			detail.Type = rpcpb.TxOutDetail_contract_create
		}
		detail.Appendix = contractInfo
	}
	//
	return detail, fee, nil
}

func parseVoutType(txOut *types.TxOut) rpcpb.TxOutDetail_TxOutType {
	sc := script.NewScriptFromBytes(txOut.ScriptPubKey)
	if sc.IsPayToPubKeyHash() {
		return rpcpb.TxOutDetail_pay_to_pubkey_hash
	} else if sc.IsPayToPubKeyHashCLTVScript() {
		return rpcpb.TxOutDetail_pay_to_pubkey_hash_cltv
	} else if sc.IsTokenIssue() {
		return rpcpb.TxOutDetail_token_issue
	} else if sc.IsTokenTransfer() {
		return rpcpb.TxOutDetail_token_transfer
	} else if sc.IsSplitAddrScript() {
		return rpcpb.TxOutDetail_new_split_addr
	} else if sc.IsPayToScriptHash() {
		return rpcpb.TxOutDetail_pay_to_script_hash
	} else if sc.IsContractPubkey() {
		// distinguish create and call after this.
		return rpcpb.TxOutDetail_contract_call
	}

	return rpcpb.TxOutDetail_unknown
}

func blockConfirmed(b *types.Block, r ChainBlockReader) bool {
	if b == nil {
		return false
	}
	eternalHeight := r.EternalBlock().Header.Height
	return eternalHeight >= b.Header.Height
}

// ParseAddrFrom parse addr from scriptPubkey
func ParseAddrFrom(
	sc *script.Script, txHash *crypto.HashType, idx uint32,
) (string, error) {
	var (
		address types.Address
		err     error
	)
	switch {
	case sc.IsContractPubkey():
		address, err = sc.ParseContractAddr()
		if err == nil && (address == nil || len(address.Hash()) == 0) {
			// smart contract deploy
			from, err := sc.ParseContractFrom()
			if err != nil {
				return "", err
			}
			nonce, err := sc.ParseContractNonce()
			if err != nil {
				return "", err
			}
			address, _ = types.MakeContractAddress(from, nonce)
		}
	case sc.IsSplitAddrScript():
		addrs, weights, err := sc.ParseSplitAddrScript()
		if err != nil {
			return "", err
		}
		address = txlogic.MakeSplitAddress(txHash, idx, addrs, weights)
	default:
		if sc.IsOpReturnScript() {
			return "", nil
		}
		address, err = sc.ExtractAddress()
	}
	if err != nil {
		return "", err
	}
	return address.String(), nil
}

func (s *webapiServer) Table(
	ctx context.Context, req *rpcpb.TableReq,
) (*rpcpb.TableResp, error) {

	return &rpcpb.TableResp{
		Code:    0,
		Message: "",
		Table:   s.TableReader.ConnectingPeers(),
	}, nil
}

func newGetCodeResp(code int32, message string, data string) *rpcpb.GetCodeResp {
	return &rpcpb.GetCodeResp{
		Code:    code,
		Message: message,
		Data:    data,
	}
}

func (s *webapiServer) GetCode(
	ctx context.Context, req *rpcpb.GetCodeReq,
) (*rpcpb.GetCodeResp, error) {
	state := s.ChainBlockReader.TailState()
	contractAddress, err := types.NewContractAddress(req.Address)
	if err != nil {
		logger.Error(err)
		return newGetCodeResp(-1, err.Error(), ""), nil
	}

	addr := contractAddress.Hash160()
	code := state.GetCode(*addr)
	return newGetCodeResp(0, "ok", hex.EncodeToString(code)), nil
}

func (s *webapiServer) EstimateGas(
	ctx context.Context, req *rpcpb.CallReq,
) (*rpcpb.EstimateGasResp, error) {

	_, gas, _, err := callContract(req.GetFrom(), req.GetTo(), req.GetData(),
		req.GetHeight(), s.ChainBlockReader)
	if err != nil {
		return &rpcpb.EstimateGasResp{
			Code:    -1,
			Message: err.Error(),
			Gas:     0,
		}, nil
	}

	return &rpcpb.EstimateGasResp{
		Code:    0,
		Message: "ok",
		Gas:     int32(gas),
	}, nil
}

func newStorageAtResp(code int32, msg, data string) *rpcpb.StorageResp {
	return &rpcpb.StorageResp{
		Code:    code,
		Message: msg,
		Data:    data,
	}
}

func (s *webapiServer) GetStorageAt(
	ctx context.Context, req *rpcpb.StorageReq,
) (*rpcpb.StorageResp, error) {

	state, err := s.GetStateDbByHeight(req.Height)
	if err != nil {
		return newStorageAtResp(-1, err.Error(), ""), nil
	}

	contractAddress, err := types.NewContractAddress(req.Address)
	if err != nil {
		return newStorageAtResp(-1, err.Error(), ""), nil
	}

	addr := contractAddress.Hash160()

	var key crypto.HashType
	key.SetString(req.Position)

	val := state.GetState(*addr, key)
	err = state.Error()
	if err != nil {
		return newStorageAtResp(-1, err.Error(), ""), nil
	}
	return newStorageAtResp(0, "", val.String()), nil
}
