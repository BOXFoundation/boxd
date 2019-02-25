// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package rpc

import (
	"container/list"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/BOXFoundation/boxd/boxd/eventbus"
	"github.com/BOXFoundation/boxd/core/pb"
	"github.com/BOXFoundation/boxd/core/txlogic"
	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/crypto"
	"github.com/BOXFoundation/boxd/rpc/pb"
	"github.com/BOXFoundation/boxd/script"
	"github.com/btcsuite/btcutil/base58"
	"github.com/jbenet/goprocess"
	"golang.org/x/net/context"
)

const (
	newBlockMsgSize = 60
)

func init() {
	RegisterServiceWithGatewayHandler(
		"web",
		registerWebapi,
		rpcpb.RegisterWebApiHandlerFromEndpoint,
	)
}

func registerWebapi(s *Server) {
	was := newWebAPIServer(s)
	rpcpb.RegisterWebApiServer(s.server, was)
	s.eventBus.Subscribe(eventbus.TopicRPCSendNewBlock, was.receiveNewBlockMsg)
}

type webapiServer struct {
	ChainBlockReader
	TxPoolReader
	proc           goprocess.Process
	newBlockMutex  sync.RWMutex
	newBlocksQueue *list.List
}

// ChainTxReader defines chain tx reader interface
type ChainTxReader interface {
	LoadTxByHash(crypto.HashType) (*types.Transaction, error)
}

// ChainBlockReader defines chain block reader interface
type ChainBlockReader interface {
	ChainTxReader
	LoadBlockInfoByTxHash(crypto.HashType) (*types.Block, uint32, error)
	LoadBlockByHash(crypto.HashType) (*types.Block, error)
	EternalBlock() *types.Block
}

// TxPoolReader defines tx pool reader interface
type TxPoolReader interface {
	GetTxByHash(hash *crypto.HashType) (*types.TxWrap, bool)
}

func newWebAPIServer(s *Server) *webapiServer {
	return &webapiServer{
		ChainBlockReader: s.GetChainReader(),
		TxPoolReader:     s.GetTxHandler(),
		proc:             s.Proc(),
		newBlocksQueue:   list.New(),
	}
}

func (s *webapiServer) Closing() bool {
	select {
	case <-s.proc.Closing():
		return true
	default:
		return false
	}
}

func (s *webapiServer) receiveNewBlockMsg(msg *types.Block) {
	s.newBlockMutex.Lock()
	if s.newBlocksQueue.Len() == newBlockMsgSize {
		s.newBlocksQueue.Remove(s.newBlocksQueue.Front())
	}
	s.newBlocksQueue.PushBack(msg)
	s.newBlockMutex.Unlock()
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
	var tx *types.Transaction
	br, tr := s.ChainBlockReader, s.TxPoolReader
	if block, index, err := br.LoadBlockInfoByTxHash(*hash); err == nil {
		tx = block.Txs[index]
		// calc tx status
		if blockConfirmed(block, br) {
			resp.Status = rpcpb.ViewTxDetailResp_confirmed
		} else {
			resp.Status = rpcpb.ViewTxDetailResp_onchain
		}
		resp.BlockTime = block.Header.TimeStamp
		resp.BlockHeight = block.Height
	} else {
		logger.Warnf("view tx detail load block by tx hash %s error: %s,"+
			" try get it from tx pool", hash, err)
		txWrap, _ := tr.GetTxByHash(hash)
		if txWrap == nil {
			return newViewTxDetailResp(-1, "tx not found"), nil
		}
		tx = txWrap.Tx
		resp.Status = rpcpb.ViewTxDetailResp_pending
		resp.BlockTime = txWrap.AddedTimestamp
		resp.BlockHeight = txWrap.Height
	}
	resp.Version = tx.Version
	// fetch tx details
	detail, err := detailTx(tx, br, tr)
	if err != nil {
		logger.Warn("view tx detail error: ", err)
		return newViewTxDetailResp(-1, err.Error()), nil
	}
	//
	resp.Detail = detail
	logger.Infof("view tx detail resp: %+v", resp)
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
	block, err := br.LoadBlockByHash(*hash)
	if err != nil {
		logger.Warn("view block detail error: ", err)
		return newViewBlockDetailResp(-1, err.Error()), nil
	}
	detail, err := detailBlock(block, br, tr)
	if err != nil {
		logger.Warn("view block detail error: ", err)
		return newViewBlockDetailResp(-1, err.Error()), nil
	}
	resp := newViewBlockDetailResp(0, "")
	resp.Detail = detail
	return resp, nil
}

func (s *webapiServer) GetTokenInfo(
	ctx context.Context, req *rpcpb.GetTokenInfoRequest,
) (*rpcpb.GetTokenInfoResponse, error) {

	tokenAddr := types.Token{}
	if err := tokenAddr.SetString(req.Addr); err != nil {
		return nil, err
	}
	br := s.ChainBlockReader
	tx, err := br.LoadTxByHash(tokenAddr.OutPoint().Hash)
	if err != nil {
		return nil, err
	}
	block, _, err := br.LoadBlockInfoByTxHash(tokenAddr.OutPoint().Hash)
	if err != nil {
		return nil, err
	}
	if uint32(len(tx.Vout)) <= tokenAddr.OutPoint().Index {
		return nil, fmt.Errorf("invalid token index")
	}
	out := tx.Vout[tokenAddr.OutPoint().Index]
	sc := script.NewScriptFromBytes(out.ScriptPubKey)
	if !sc.IsTokenIssue() {
		return nil, fmt.Errorf("invalid token id")
	}
	param, err := sc.GetIssueParams()
	if err != nil {
		return nil, err
	}
	addr, err := sc.ExtractAddress()
	if err != nil {
		return nil, err
	}
	return &rpcpb.GetTokenInfoResponse{
		Info: &rpcpb.TokenBasicInfo{
			Token: &rpcpb.Token{
				Hash:  tokenAddr.OutPoint().Hash.String(),
				Index: tokenAddr.OutPoint().Index,
			},
			Name:        param.Name,
			TotalSupply: param.TotalSupply,
			CreatorAddr: addr.String(),
			CreatorTime: uint64(block.Header.TimeStamp),
			Decimals:    uint32(param.Decimals),
			Symbol:      param.Symbol,
			Addr:        tokenAddr.String(),
		},
	}, nil
}

func (s *webapiServer) ListenAndReadNewBlock(
	req *rpcpb.ListenBlocksReq,
	stream rpcpb.WebApi_ListenAndReadNewBlockServer,
) error {
	var (
		elm  *list.Element
		exit bool
	)
	for {
		if s.Closing() {
			logger.Info("receive closing signal, exit ListenAndReadNewBlock ...")
			return nil
		}
		s.newBlockMutex.RLock()
		if s.newBlocksQueue.Len() != 0 {
			elm = s.newBlocksQueue.Front()
			s.newBlockMutex.RUnlock()
			break
		}
		s.newBlockMutex.RUnlock()
		time.Sleep(100 * time.Millisecond)
	}
	for {
		// get block
		block := elm.Value.(*types.Block)
		logger.Debugf("webapiServer receives a block, hash: %s, height: %d",
			block.BlockHash(), block.Height)
		// detail block
		blockDetail, err := detailBlock(block, s.ChainBlockReader, s.TxPoolReader)
		if err != nil {
			logger.Warnf("detail block %s height %d error: %s",
				block.BlockHash(), block.Height, err)
			if elm, exit = s.moveToNextElem(elm); exit {
				return nil
			}
			continue
		}
		// send block info
		if err := stream.Send(blockDetail); err != nil {
			logger.Warnf("webapi send block error %s, exit listen connection!", err)
			return err
		}
		logger.Debugf("webapi server sent a block, hash: %s, height: %d",
			block.BlockHash(), block.Height)
		if elm, exit = s.moveToNextElem(elm); exit {
			return nil
		}
	}
}

func (s *webapiServer) moveToNextElem(elm *list.Element) (elmA *list.Element, exit bool) {
	// move to next element
	for {
		if s.Closing() {
			logger.Info("receive closing signal, exit ListenAndReadNewBlock ...")
			return nil, true
		}
		s.newBlockMutex.RLock()
		next := elm.Next()
		if next != nil {
			elmA = next
			s.newBlockMutex.RUnlock()
			return elmA, false
		} else if elm.Prev() == nil {
			// if this element is removed, move to the fromt of list
			elmA = s.newBlocksQueue.Front()
			s.newBlockMutex.RUnlock()
			return elmA, false
		}
		s.newBlockMutex.RUnlock()
		time.Sleep(100 * time.Millisecond)
	}
}

func detailTx(
	tx *types.Transaction, br ChainTxReader, tr TxPoolReader,
) (*rpcpb.TxDetail, error) {

	detail := new(rpcpb.TxDetail)
	hash, _ := tx.TxHash()
	detail.Hash = hash.String()
	// parse vin
	for _, in := range tx.Vin {
		inDetail, err := detailTxIn(in, br, tr)
		if err != nil {
			return nil, err
		}
		detail.Vin = append(detail.Vin, inDetail)
	}
	// parse vout
	for i := range tx.Vout {
		txHash, _ := tx.TxHash()
		outDetail, err := detailTxOut(txHash, tx.Vout[i], uint32(i))
		if err != nil {
			return nil, err
		}
		detail.Vout = append(detail.Vout, outDetail)
	}
	return detail, nil
}

func detailBlock(
	block *types.Block, r ChainBlockReader, tr TxPoolReader,
) (*rpcpb.BlockDetail, error) {

	if block == nil || block.Header == nil {
		return nil, fmt.Errorf("detailBlock error for block: %+v", block)
	}
	detail := new(rpcpb.BlockDetail)
	detail.Version = block.Header.Version
	detail.Height = block.Height
	detail.TimeStamp = block.Header.TimeStamp
	detail.Hash = block.BlockHash().String()
	detail.PrevBlockHash = block.Header.PrevBlockHash.String()
	// coin base is miner address or block fee receiver
	coinBase, err := getCoinbaseAddr(block)
	if err != nil {
		return nil, err
	}
	detail.CoinBase = coinBase
	detail.Confirmed = blockConfirmed(block, r)
	detail.Signature = hex.EncodeToString(block.Signature)
	for _, tx := range block.Txs {
		txDetail, err := detailTx(tx, r, tr)
		if err != nil {
			return nil, err
		}
		detail.Txs = append(detail.Txs, txDetail)
	}
	return detail, nil
}

func detailTxIn(
	txIn *types.TxIn, r ChainTxReader, tr TxPoolReader,
) (*rpcpb.TxInDetail, error) {

	detail := new(rpcpb.TxInDetail)
	// if tx in is coin base
	if txlogic.IsCoinBaseTxIn(txIn) {
		return detail, nil
	}
	//
	hash := &txIn.PrevOutPoint.Hash
	prevTx, err := r.LoadTxByHash(*hash)
	if err != nil {
		logger.Infof("load tx by hash %s from chain error: %s, try tx pool", hash, err)
		txWrap, _ := tr.GetTxByHash(hash)
		if txWrap == nil {
			return nil, fmt.Errorf("tx not found for detail txIn %+v", txIn.PrevOutPoint)
		}
		prevTx = txWrap.Tx
	}
	detail.ScriptSig = hex.EncodeToString(txIn.ScriptSig)
	detail.Sequence = txIn.Sequence
	detail.PrevOutPoint = convOutPoint(txlogic.ConvOutPoint(&txIn.PrevOutPoint))
	index := txIn.PrevOutPoint.Index
	prevTxHash, _ := prevTx.TxHash()
	detail.PrevOutDetail, err = detailTxOut(prevTxHash, prevTx.Vout[index],
		txIn.PrevOutPoint.Index)
	if err != nil {
		return nil, err
	}
	return detail, nil
}

func detailTxOut(
	txHash *crypto.HashType, txOut *corepb.TxOut, index uint32,
) (*rpcpb.TxOutDetail, error) {

	detail := new(rpcpb.TxOutDetail)
	// addr
	sc := script.NewScriptFromBytes(txOut.ScriptPubKey)
	address, err := sc.ExtractAddress()
	if err != nil {
		return nil, err
	}
	detail.Addr = address.String()
	// value
	op := types.NewOutPoint(txHash, index)
	// height 0 is unused
	wrap := types.NewUtxoWrap(txOut.Value, txOut.ScriptPubKey, 0)
	utxo := txlogic.MakePbUtxo(op, wrap)
	amount, _, err := txlogic.ParseUtxoAmount(utxo)
	if err != nil {
		return nil, err
	}
	detail.Value = amount
	// script pubic key
	detail.ScriptPubKey = hex.EncodeToString(txOut.ScriptPubKey)
	// script disasm
	detail.ScriptDisasm = sc.Disasm()
	// type
	detail.Type = parseVoutType(txOut)
	//  appendix
	switch detail.Type {
	default:
	case rpcpb.TxOutDetail_token_issue:
		param, err := sc.GetIssueParams()
		if err != nil {
			return nil, err
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
			return nil, err
		}
		transferInfo := &rpcpb.TxOutDetail_TokenTransferInfo{
			TokenTransferInfo: &rpcpb.TokenTransferInfo{
				TokenId: convOutPoint(txlogic.ConvOutPoint(&param.TokenID.OutPoint)),
			},
		}
		detail.Appendix = transferInfo
	case rpcpb.TxOutDetail_pay_to_split_script:
		//addresses, amounts, err := sc.ParseSplitAddrScript()
	}
	//
	return detail, nil
}

func parseVoutType(txOut *corepb.TxOut) rpcpb.TxOutDetail_TxOutType {
	sc := script.NewScriptFromBytes(txOut.ScriptPubKey)
	if sc.IsTokenIssue() {
		return rpcpb.TxOutDetail_token_issue
	} else if sc.IsTokenTransfer() {
		return rpcpb.TxOutDetail_token_transfer
	} else if sc.IsPayToPubKeyHash() {
		return rpcpb.TxOutDetail_pay_to_pubkey
	} else if sc.IsSplitAddrScript() {
		return rpcpb.TxOutDetail_pay_to_split_script
	} else if sc.IsPayToPubKeyHashCLTVScript() {
		return rpcpb.TxOutDetail_pay_to_pubkey_cltv_script
	} else if sc.IsPayToScriptHash() {
		return rpcpb.TxOutDetail_pay_to_script
	} else {
		return rpcpb.TxOutDetail_unknown
	}
}

func blockConfirmed(b *types.Block, r ChainBlockReader) bool {
	if b == nil {
		return false
	}
	eternalHeight := r.EternalBlock().Height
	return eternalHeight >= b.Height
}

func getCoinbaseAddr(block *types.Block) (string, error) {
	if block.Txs == nil || len(block.Txs) == 0 {
		return "", fmt.Errorf("coinbase does not exist in block %s height %d",
			block.BlockHash(), block.Height)
	}
	tx := block.Txs[0]
	sc := *script.NewScriptFromBytes(tx.Vout[0].ScriptPubKey)
	address, err := sc.ExtractAddress()
	if err != nil {
		return "", err
	}
	return address.String(), nil
}

func convOutPoint(op *corepb.OutPoint) string {
	buf := op.GetHash()
	// reverse bytes
	for i, j := 0, len(buf)-1; i < j; i, j = i+1, j-1 {
		buf[i], buf[j] = buf[j], buf[i]
	}
	// append separator ':'
	buf = append(buf, ':')
	// put index
	b := make([]byte, 4)
	binary.LittleEndian.PutUint32(b, op.GetIndex())
	buf = append(buf, b...)

	return base58.Encode(buf)
}
