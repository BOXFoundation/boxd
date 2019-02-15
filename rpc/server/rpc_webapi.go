// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package rpc

import (
	"container/list"
	"fmt"
	"sync"
	"time"

	"github.com/BOXFoundation/boxd/boxd/eventbus"
	"github.com/BOXFoundation/boxd/core/pb"
	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/crypto"
	"github.com/BOXFoundation/boxd/rpc/pb"
	"github.com/BOXFoundation/boxd/script"
	"github.com/BOXFoundation/boxd/util"
	"golang.org/x/net/context"
)

const (
	newBlockMsgSize = 60
)

func registerWebapi(s *Server) {
	was := newWebAPIServer(s)
	rpcpb.RegisterWebApiServer(s.server, was)
	s.eventBus.Subscribe(eventbus.TopicRPCSendNewBlock, was.receiveNewBlockMsg)
}

func init() {
	RegisterServiceWithGatewayHandler(
		"web",
		registerWebapi,
		rpcpb.RegisterWebApiHandlerFromEndpoint,
	)
}

type webapiServer struct {
	GRPCServer
	newBlockMutex  sync.RWMutex
	newBlocksQueue *list.List
}

func newWebAPIServer(s *Server) *webapiServer {
	return &webapiServer{
		GRPCServer:     s,
		newBlocksQueue: list.New(),
	}
}

func (s *webapiServer) Closing() bool {
	select {
	case <-s.Proc().Closing():
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

func (s *webapiServer) ListTokens(ctx context.Context, req *rpcpb.ListTokensRequest) (*rpcpb.ListTokensResponse, error) {
	tokenIssueTransactions, headers, err := s.GetChainReader().ListTokenIssueTransactions()
	if err != nil {
		return nil, err
	}
	if len(tokenIssueTransactions) != len(headers) {
		return nil, fmt.Errorf("missing block header")
	}
	var tokenInfos []*rpcpb.TokenBasicInfo
	var txInRange []*types.Transaction
	total := uint32(len(tokenIssueTransactions))
	logger.Infof("%v transactions found related to token issue", total)
	if total < req.Offset {
		return &rpcpb.ListTokensResponse{
			Count:  total,
			Tokens: []*rpcpb.TokenBasicInfo{},
		}, nil
	} else if total < req.Offset+req.Limit {
		txInRange = tokenIssueTransactions[req.Offset:]
	} else {
		txInRange = tokenIssueTransactions[req.Offset : req.Offset+req.Limit]
	}
	for i, tx := range txInRange {
		hash, err := tx.TxHash()
		if err != nil {
			return nil, err
		}
		for idx, vout := range tx.Vout {
			sc := script.NewScriptFromBytes(vout.ScriptPubKey)
			if sc.IsTokenIssue() {
				params, err := sc.GetIssueParams()
				if err != nil {
					return nil, err
				}
				addr, err := sc.ExtractAddress()
				if err != nil {
					return nil, err
				}
				tokenAddr := types.NewTokenFromOutpoint(types.OutPoint{
					Hash:  *hash,
					Index: uint32(idx),
				})
				tokenInfo := &rpcpb.TokenBasicInfo{
					Addr:        tokenAddr.String(),
					Name:        params.Name,
					TotalSupply: params.TotalSupply,
					CreatorAddr: addr.String(),
					CreatorTime: uint64(headers[int(req.Offset)+i].TimeStamp),
					Decimals:    uint32(params.Decimals),
					Symbol:      params.Symbol,
				}
				tokenInfos = append(tokenInfos, tokenInfo)
				break
			}
		}
	}
	return &rpcpb.ListTokensResponse{
		Count:  total,
		Tokens: tokenInfos,
	}, nil
}

func (s *webapiServer) GetTokenInfo(ctx context.Context, req *rpcpb.GetTokenInfoRequest) (*rpcpb.GetTokenInfoResponse, error) {
	tokenAddr := types.Token{}
	if err := tokenAddr.SetString(req.Addr); err != nil {
		return nil, err
	}
	tx, err := s.GetChainReader().LoadTxByHash(tokenAddr.OutPoint().Hash)
	if err != nil {
		return nil, err
	}
	block, _, err := s.GetChainReader().LoadBlockInfoByTxHash(tokenAddr.OutPoint().Hash)
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

func (s *webapiServer) GetTokenTransactions(ctx context.Context, req *rpcpb.GetTokenTransactionsRequest) (*rpcpb.GetTransactionsInfoResponse, error) {
	tokenAddr := new(types.Token)
	if err := tokenAddr.SetString(req.Addr); err != nil {
		return nil, err
	}
	tokenID := &script.TokenID{OutPoint: tokenAddr.OutPoint()}
	allTxs, err := s.GetChainReader().GetTokenTransactions(tokenID)
	if err != nil {
		return nil, err
	}
	total := uint32(len(allTxs))
	logger.Infof("%v txs found related to token %v", total, tokenID)
	var txInRange []*types.Transaction
	if total <= req.Offset {
		return &rpcpb.GetTransactionsInfoResponse{
			Total: total,
			Txs:   []*rpcpb.TransactionInfo{},
		}, nil
	} else if total < req.Offset+req.Limit {
		txInRange = allTxs[:total-req.Offset]
	} else {
		txInRange = allTxs[total-req.Offset-req.Limit : total-req.Offset]
	}
	utxos, err := s.loadUtxoForTx(txInRange)
	if err != nil {
		return nil, err
	}
	logger.Infof("%v txs found related to token %v", len(txInRange))
	txInfos, err := s.convertTransactionInfos(txInRange, utxos)
	if err != nil {
		return nil, err
	}
	for i := 0; i < len(txInfos)/2; i++ {
		j := len(txInfos) - i - 1
		txInfos[i], txInfos[j] = txInfos[j], txInfos[i]
	}
	return &rpcpb.GetTransactionsInfoResponse{
		Total: total,
		Txs:   txInfos,
	}, nil
}

func (s *webapiServer) GetPendingTransaction(ctx context.Context, req *rpcpb.GetPendingTransactionRequest) (*rpcpb.GetTransactionsInfoResponse, error) {
	txs, addedTime := s.GetTxHandler().GetTransactionsInPool()
	var txInRange []*types.Transaction
	if len(txs) <= int(req.Offset) {
		txInRange = []*types.Transaction{}
	} else if len(txs) < int(req.Offset+req.Limit) {
		txInRange = txs[req.Offset:]
	} else {
		txInRange = txs[req.Offset : req.Offset+req.Limit]
	}
	generatedUtxo, err := s.loadGeneratedUtxoForTx(txs)
	if err != nil {
		return nil, err
	}
	utxos, err := s.loadUsedUtxoForTx(generatedUtxo, txInRange)
	if err != nil {
		return nil, err
	}
	logger.Debugf("utxos %v", utxos)
	var txInfos []*rpcpb.TransactionInfo
	for idx, tx := range txInRange {
		msg, err := tx.ToProtoMessage()
		if err != nil {
			return nil, err
		}
		txPb, ok := msg.(*corepb.Transaction)
		if !ok {
			return nil, fmt.Errorf("invalid transacton format")
		}
		hash, err := tx.TxHash()
		if err != nil {
			return nil, err
		}
		var totalIn, totalOut uint64
		for _, in := range tx.Vin {
			if wrap, ok := utxos[in.PrevOutPoint]; ok && wrap != nil {
				totalIn += wrap.Value()
				logger.Debugf("input value %v", wrap.Value())
			} else {
				return nil, fmt.Errorf("previous input not found %v", in)
			}
		}
		for _, out := range tx.Vout {
			totalOut += out.Value
		}
		fee := totalIn - totalOut
		var outInfos []*rpcpb.TxOutInfo
		for idx, o := range txPb.Vout {
			outInfo, err := convertVout(*hash, uint32(idx), o)
			if err != nil {
				return nil, err
			}
			outInfos = append(outInfos, outInfo)
		}
		var inInfos []*rpcpb.TxInInfo
		for _, i := range tx.Vin {
			opInfo := &rpcpb.OutPointInfo{
				Hash:  i.PrevOutPoint.Hash.String(),
				Index: i.PrevOutPoint.Index,
			}
			utxo, ok := utxos[i.PrevOutPoint]
			if !ok {
				return nil, fmt.Errorf("previous input not found %v", i.PrevOutPoint)
			}
			info := &rpcpb.TxInInfo{
				PrevOutPoint: opInfo,
				ScriptSig:    i.ScriptSig,
				Sequence:     i.Sequence,
				Value:        utxo.Value(),
			}
			sc := *script.NewScriptFromBytes(utxo.Script())
			if addr, err := sc.ExtractAddress(); err == nil {
				info.Addr = addr.String()
			}
			inInfos = append(inInfos, info)
		}
		txInfo := &rpcpb.TransactionInfo{
			Version:   tx.Version,
			Vin:       inInfos,
			Vout:      outInfos,
			Data:      txPb.Data,
			Magic:     tx.Magic,
			LockTime:  tx.LockTime,
			Hash:      hash.String(),
			Fee:       fee,
			Size_:     0,
			AddedTime: uint64(addedTime[int(req.Offset)+idx]),
		}
		txInfos = append(txInfos, txInfo)
	}
	return &rpcpb.GetTransactionsInfoResponse{
		Total: uint32(len(txs)),
		Txs:   txInfos,
	}, nil
}

func convertVout(hash crypto.HashType, index uint32, vout *corepb.TxOut) (*rpcpb.TxOutInfo, error) {
	sc := script.NewScriptFromBytes(vout.ScriptPubKey)
	out := &rpcpb.TxOutInfo{
		Value:        vout.Value,
		ScriptPubKey: vout.ScriptPubKey,
		ScriptDisasm: sc.Disasm(),
	}
	if sc.IsTokenIssue() {
		params, err := sc.GetIssueParams()
		if err != nil {
			return nil, err
		}
		tokenAddr := types.NewTokenFromOutpoint(types.OutPoint{
			Hash:  hash,
			Index: index,
		})
		out.IssueInfo = &rpcpb.TokenIssueInfo{
			Name:        params.Name,
			TotalSupply: params.TotalSupply,
			Symbol:      params.Symbol,
			Decimals:    uint32(params.Decimals),
			Addr:        tokenAddr.String(),
		}
	} else if sc.IsTokenTransfer() {
		params, err := sc.GetTransferParams()
		if err != nil {
			return nil, err
		}
		tokenAddr := types.NewTokenFromOutpoint(types.OutPoint{
			Hash:  params.Hash,
			Index: params.Index,
		})
		out.TransferInfo = &rpcpb.TokenTransferInfo{
			Addr:   tokenAddr.String(),
			Amount: params.Amount,
		}
	}
	if addr, err := sc.ExtractAddress(); err == nil {
		out.Addr = addr.String()
	}
	return out, nil
}

func (s *webapiServer) GetTransactionHistory(ctx context.Context, req *rpcpb.GetTransactionHistoryRequest) (*rpcpb.GetTransactionsInfoResponse, error) {
	addr := &types.AddressPubKeyHash{}
	if err := addr.SetString(req.Addr); err != nil {
		return nil, err
	}
	txs, err := s.GetChainReader().GetTransactionsByAddr(addr)
	if err != nil {
		return nil, err
	}
	var txInRange []*types.Transaction
	if len(txs) <= int(req.Offset) {
		txInRange = []*types.Transaction{}
	} else if len(txs) < int(req.Offset+req.Limit) {
		txInRange = txs[:len(txs)-int(req.Offset)]
	} else {
		txInRange = txs[len(txs)-int(req.Offset)-int(req.Limit) : len(txs)-int(req.Offset)]
	}
	logger.Infof("transactions inf range: %v", len(txInRange))
	utxos, err := s.loadUtxoForTx(txInRange)
	logger.Debugf("utxos %v", util.PrettyPrint(utxos))
	if err != nil {
		return nil, err
	}
	txInfos, err := s.convertTransactionInfos(txInRange, utxos)
	for i := 0; i < len(txInfos)/2; i++ {
		j := len(txInfos) - i - 1
		txInfos[i], txInfos[j] = txInfos[j], txInfos[i]
	}
	return &rpcpb.GetTransactionsInfoResponse{
		Total: uint32(len(txs)),
		Txs:   txInfos,
	}, nil
}

func (s *webapiServer) GetTransaction(ctx context.Context, req *rpcpb.GetTransactionInfoRequest) (*rpcpb.GetTransactionInfoResponse, error) {
	hash := &crypto.HashType{}
	if err := hash.SetString(req.Hash); err != nil {
		return nil, err
	}
	block, index, err := s.GetChainReader().LoadBlockInfoByTxHash(*hash)
	//tx, err := s.GetChainReader().LoadTxByHash(*hash)
	if err != nil {
		return nil, err
	}
	tx := block.Txs[index]
	utxos, err := s.loadUtxoForTx([]*types.Transaction{tx})
	if err != nil {
		return nil, err
	}
	txInfo, err := s.convertTransaction(tx, utxos)
	if err != nil {
		return nil, err
	}
	return &rpcpb.GetTransactionInfoResponse{
		TxInfo: txInfo,
		ExtraInfo: &rpcpb.TransactionExtraInfo{
			BlockTime:   block.Header.TimeStamp,
			BlockHeight: block.Height,
		},
	}, nil
}

func (s *webapiServer) GetBlock(ctx context.Context, req *rpcpb.GetBlockInfoRequest) (*rpcpb.BlockInfo, error) {
	hash := &crypto.HashType{}
	if err := hash.SetString(req.Hash); err != nil {
		return nil, err
	}

	eternalBlock := s.GetChainReader().EternalBlock()
	block, err := s.GetChainReader().LoadBlockByHash(*hash)
	if err != nil {
		return nil, err
	}
	blockInfo, err := s.convertBlock(block)
	if err != nil {
		return nil, err
	}
	blockInfo.Confirmed = eternalBlock.Height >= block.Height
	if err != nil {
		return nil, err
	}
	return blockInfo, nil
}

func (s *webapiServer) ListenAndReadNewBlock(
	req *rpcpb.ListenBlockRequest,
	stream rpcpb.WebApi_ListenAndReadNewBlockServer,
) error {
	var elm *list.Element
	for {
		if s.Closing() {
			logger.Info("blocks queue is empty, exit ListenAndReadNewBlock ...")
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
		// move to next element
		for {
			if s.Closing() {
				logger.Info("exit ListenAndReadNewBlock ...")
				return nil
			}
			s.newBlockMutex.RLock()
			next := elm.Next()
			if next != nil {
				elm = next
				s.newBlockMutex.RUnlock()
				break
			}
			s.newBlockMutex.RUnlock()
			time.Sleep(100 * time.Millisecond)
		}
		// get block
		block := elm.Value.(*types.Block)
		logger.Debugf("webapiServer receives a block, hash: %s, height: %d",
			block.BlockHash(), block.Height)
		// convert block to block info
		// NOTE: need refine to reduce db access and to improve performance
		blockInfo, err := s.convertBlock(block)
		if err != nil {
			logger.Warnf("convert block %s error: %s", block.BlockHash(), err)
			continue
		}
		// assign Confirmed field
		eternalBlock := s.GetChainReader().EternalBlock()
		if eternalBlock == nil {
			logger.Warnf("get EternalBlock is nil")
			continue
		}
		blockInfo.Confirmed = eternalBlock.Height >= block.Height
		// send block info
		if err := stream.Send(blockInfo); err != nil {
			logger.Warnf("webapi send block error %s, exit listen connection!", err)
			return err
		}
		logger.Debugf("webapi server sent a block, hash: %s, height: %d",
			block.BlockHash(), block.Height)
	}
}

func (s *webapiServer) countAddresses(utxos map[types.OutPoint]*types.UtxoWrap) uint32 {
	addrs := make(map[string]bool)
	for _, wrap := range utxos {
		sc := script.NewScriptFromBytes(wrap.Script())
		addr, err := sc.ExtractAddress()
		if err != nil || addr == nil {
			continue
		}
		addrs[addr.String()] = true
	}
	return uint32(len(addrs))
}

func (s *webapiServer) analyzeDistribute(utxos map[types.OutPoint]*types.UtxoWrap) map[string]uint64 {
	distribute := make(map[string]uint64)
	for _, wrap := range utxos {
		sc := script.NewScriptFromBytes(wrap.Script())
		addr, err := sc.ExtractAddress()
		if err != nil || addr == nil {
			continue
		}
		addrStr := addr.String()
		if val, ok := distribute[addrStr]; ok {
			distribute[addrStr] = val + wrap.Value()
		} else {
			distribute[addrStr] = wrap.Value()
		}
	}
	return distribute
}

func (s *webapiServer) analyzeTokenDistribute(utxos map[types.OutPoint]*types.UtxoWrap, token *script.TokenID) (map[string]uint64, error) {
	distribute := make(map[string]uint64)
	for op, wrap := range utxos {
		sc := script.NewScriptFromBytes(wrap.Script())
		if sc.IsTokenIssue() {
			addr, err := sc.ExtractAddress()
			if err != nil {
				return nil, err
			}
			param, err := sc.GetIssueParams()
			if err != nil {
				return nil, err
			}
			if op.Hash.IsEqual(&token.Hash) && op.Index == token.Index {
				distribute[addr.String()] = param.TotalSupply
				// an utxo of issue yet implies no transfer tx yet
				return distribute, nil
			}
		} else if sc.IsTokenTransfer() {
			param, err := sc.GetTransferParams()
			if err != nil {
				return nil, err
			}
			addr, err := sc.ExtractAddress()
			if err != nil {
				return nil, err
			}
			if param.Hash.IsEqual(&token.Hash) && param.Index == token.Index {
				if val, ok := distribute[addr.String()]; ok {
					distribute[addr.String()] = val + param.Amount
				} else {
					distribute[addr.String()] = param.Amount
				}
			}
		}
	}
	return distribute, nil
}

func (s *webapiServer) loadGeneratedUtxoForTx(txs []*types.Transaction) (map[types.OutPoint]*types.UtxoWrap, error) {
	generated := make(map[types.OutPoint]*types.UtxoWrap)
	for i, tx := range txs {
		hash, err := tx.TxHash()
		if err != nil {
			return nil, err
		}
		for idx, out := range tx.Vout {
			outpoint := types.OutPoint{
				Hash:  *hash,
				Index: uint32(idx),
			}
			wrap := types.NewUtxoWrap(out.Value, out.ScriptPubKey, 0)
			if i == 0 {
				wrap.SetCoinBase()
			}
			// wrap := &types.UtxoWrap{
			// 	Output:      out,
			// 	BlockHeight: 0,
			// 	IsCoinBase:  i == 0,
			// 	IsSpent:     false,
			// 	IsModified:  false,
			// }
			generated[outpoint] = wrap
		}
	}
	return generated, nil
}

func (s *webapiServer) loadUsedUtxoForTx(memUtxoSource map[types.OutPoint]*types.UtxoWrap, txs []*types.Transaction) (map[types.OutPoint]*types.UtxoWrap, error) {
	var missing []types.OutPoint
	final := make(map[types.OutPoint]*types.UtxoWrap)
	for k, v := range memUtxoSource {
		final[k] = v
	}
	for _, tx := range txs {
		if s.GetChainReader().IsCoinBase(tx) {
			continue
		}
		for _, txIn := range tx.Vin {
			if _, ok := memUtxoSource[txIn.PrevOutPoint]; ok {
				continue
			}
			missing = append(missing, txIn.PrevOutPoint)
		}
	}
	stored, err := s.GetChainReader().LoadSpentUtxos(missing)
	if err != nil {
		return nil, err
	}
	for k, v := range stored {
		final[k] = v
	}
	return final, nil
}

func (s *webapiServer) loadUtxoForTx(txs []*types.Transaction) (map[types.OutPoint]*types.UtxoWrap, error) {
	generated := make(map[types.OutPoint]*types.UtxoWrap)
	for i, tx := range txs {
		hash, err := tx.TxHash()
		if err != nil {
			return nil, err
		}
		for idx, out := range tx.Vout {
			outpoint := types.OutPoint{
				Hash:  *hash,
				Index: uint32(idx),
			}
			wrap := types.NewUtxoWrap(out.Value, out.ScriptPubKey, 0)
			if i == 0 {
				wrap.SetCoinBase()
			}
			// wrap := &types.UtxoWrap{
			// 	Output:      out,
			// 	BlockHeight: 0,
			// 	IsCoinBase:  i == 0,
			// 	IsSpent:     false,
			// 	IsModified:  false,
			// }
			generated[outpoint] = wrap
		}
	}
	var missing []types.OutPoint
	for _, tx := range txs {
		if s.GetChainReader().IsCoinBase(tx) {
			continue
		}
		for _, txIn := range tx.Vin {
			if _, ok := generated[txIn.PrevOutPoint]; ok {
				continue
			}
			missing = append(missing, txIn.PrevOutPoint)
		}
	}
	stored, err := s.GetChainReader().LoadSpentUtxos(missing)
	if err != nil {
		return nil, err
	}
	for k, v := range stored {
		generated[k] = v
	}
	return generated, nil
}

func (s *webapiServer) convertTransactionInfos(txs []*types.Transaction, utxos map[types.OutPoint]*types.UtxoWrap) ([]*rpcpb.TransactionInfo, error) {
	var result []*rpcpb.TransactionInfo
	for _, tx := range txs {
		txPb, err := s.convertTransaction(tx, utxos)
		if err != nil {
			return nil, err
		}
		result = append(result, txPb)
	}
	return result, nil
}

func (s *webapiServer) convertTransaction(tx *types.Transaction, utxos map[types.OutPoint]*types.UtxoWrap) (*rpcpb.TransactionInfo, error) {
	hash, err := tx.TxHash()
	if err != nil {
		return nil, err
	}
	protoMsg, err := tx.ToProtoMessage()
	if err != nil {
		return nil, err
	}
	txPb, ok := protoMsg.(*corepb.Transaction)
	if !ok {
		return nil, fmt.Errorf("invalid transaction message")
	}
	bts, err := tx.Marshal()
	if err != nil {
		return nil, err
	}
	var outInfos []*rpcpb.TxOutInfo
	for idx, o := range txPb.Vout {
		outInfo, err := convertVout(*hash, uint32(idx), o)
		if err != nil {
			return nil, err
		}
		outInfos = append(outInfos, outInfo)
	}
	var inInfos []*rpcpb.TxInInfo
	var totalIn, totalOut, fee uint64
	for _, i := range tx.Vin {
		opInfo := &rpcpb.OutPointInfo{
			Hash:  i.PrevOutPoint.Hash.String(),
			Index: i.PrevOutPoint.Index,
		}
		if !s.GetChainReader().IsCoinBase(tx) {
			utxo, ok := utxos[i.PrevOutPoint]
			if !ok {
				return nil, fmt.Errorf("previous input not found %v", i.PrevOutPoint)
			}
			info := &rpcpb.TxInInfo{
				PrevOutPoint: opInfo,
				ScriptSig:    i.ScriptSig,
				Sequence:     i.Sequence,
				Value:        utxo.Value(),
			}
			sc := *script.NewScriptFromBytes(utxo.Script())
			if addr, err := sc.ExtractAddress(); err == nil {
				info.Addr = addr.String()
			}
			totalIn += utxo.Value()
			inInfos = append(inInfos, info)
		} else {
			info := &rpcpb.TxInInfo{
				PrevOutPoint: opInfo,
				ScriptSig:    i.ScriptSig,
				Sequence:     i.Sequence,
				Value:        0,
			}
			inInfos = append(inInfos, info)
		}
	}
	for _, o := range tx.Vout {
		totalOut += o.Value
	}
	if s.GetChainReader().IsCoinBase(tx) {
		fee = 0
	} else {
		fee = totalIn - totalOut
	}
	out := &rpcpb.TransactionInfo{
		Version:  tx.Version,
		Vin:      inInfos,
		Vout:     outInfos,
		Data:     txPb.Data,
		Magic:    txPb.Magic,
		LockTime: txPb.LockTime,
		Hash:     hash.String(),
		Fee:      fee,
		Size_:    uint64(len(bts)),
	}
	return out, nil
}

func (s *webapiServer) convertBlock(block *types.Block) (*rpcpb.BlockInfo, error) {

	headerPb, err := convertHeader(block.Header)
	if err != nil {
		return nil, err
	}
	bts, err := block.Marshal()
	if err != nil {
		return nil, err
	}
	utxos, err := s.loadUtxoForTx(block.Txs)
	if err != nil {
		return nil, err
	}
	txsPb, err := s.convertTransactionInfos(block.Txs, utxos)
	if err != nil {
		return nil, err
	}
	coinbase, err := getCoinbaseAddr(block)
	if err != nil {
		return nil, err
	}
	out := &rpcpb.BlockInfo{
		Header:    headerPb,
		Txs:       txsPb,
		Height:    block.Height,
		Signature: block.Signature,
		Hash:      block.Hash.String(),
		Size_:     uint64(len(bts)),
		CoinBase:  coinbase.String(),
	}
	return out, nil
}

func getCoinbaseAddr(block *types.Block) (types.Address, error) {
	if block.Txs == nil || len(block.Txs) == 0 {
		return nil, fmt.Errorf("coinbase transaction does not exist")
	}
	tx := block.Txs[0]
	sc := *script.NewScriptFromBytes(tx.Vout[0].ScriptPubKey)
	return sc.ExtractAddress()
}

func convertHeader(header *types.BlockHeader) (*rpcpb.HeaderInfo, error) {
	if header == nil {
		return nil, nil
	}
	return &rpcpb.HeaderInfo{
		Version:        header.Version,
		PrevBlockHash:  header.PrevBlockHash.String(),
		TxsRoot:        header.TxsRoot.String(),
		TimeStamp:      header.TimeStamp,
		Magic:          header.Magic,
		PeriodHash:     header.PeriodHash.String(),
		CandidatesHash: header.CandidatesHash.String(),
	}, nil
}
