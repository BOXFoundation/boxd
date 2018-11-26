// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package rpc

import (
	"fmt"
	"sort"

	"github.com/BOXFoundation/boxd/util"

	"github.com/BOXFoundation/boxd/script"

	"github.com/BOXFoundation/boxd/core/pb"
	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/crypto"
	"github.com/BOXFoundation/boxd/rpc/pb"
	"golang.org/x/net/context"
)

func registerWebapi(s *Server) {
	rpcpb.RegisterWebApiServer(s.server, &webapiServer{server: s})
}

func init() {
	RegisterServiceWithGatewayHandler(
		"web",
		registerWebapi,
		rpcpb.RegisterWebApiHandlerFromEndpoint,
	)
}

type webapiServer struct {
	server GRPCServer
}

func (s *webapiServer) ListTokens(ctx context.Context, req *rpcpb.ListTokensRequest) (*rpcpb.ListTokensResponse, error) {
	tokenIssueTransactions, err := s.server.GetChainReader().ListTokenIssueTransactions()
	if err != nil {
		return nil, err
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
	for _, tx := range txInRange {
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
				tokenInfo := &rpcpb.TokenBasicInfo{
					Token: &rpcpb.Token{
						Hash:  hash.String(),
						Index: uint32(idx),
					},
					Name:        params.Name,
					TotalSupply: params.TotalSupply,
					CreatorAddr: addr.String(),
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

func (s *webapiServer) GetTokenHolders(ctx context.Context, req *rpcpb.GetTokenHoldersRequest) (*rpcpb.GetTokenHoldersResponse, error) {
	utxos, err := s.server.GetChainReader().ListAllUtxos()
	if err != nil {
		return nil, err
	}
	hash := &crypto.HashType{}
	if err := hash.SetString(req.Token.Hash); err != nil {
		return nil, err
	}
	tokenID := &script.TokenID{
		OutPoint: types.OutPoint{
			Hash:  *hash,
			Index: req.Token.Index,
		},
	}
	distribute, err := s.analyzeTokenDistribute(utxos, tokenID)
	if err != nil {
		return nil, err
	}
	var holders, targetHolders []*rpcpb.AddressAmount
	for addr, val := range distribute {
		holders = append(holders, &rpcpb.AddressAmount{
			Addr:   addr,
			Amount: val,
		})
	}
	sort.Slice(holders, func(i, j int) bool {
		return holders[i].Amount > holders[j].Amount
	})
	total := uint32(len(holders))
	if total <= req.Offset {
		targetHolders = []*rpcpb.AddressAmount{}
	} else if total < req.Offset+req.Limit {
		targetHolders = holders[req.Offset:]
	} else {
		targetHolders = holders[req.Offset : req.Offset+req.Limit]
	}
	return &rpcpb.GetTokenHoldersResponse{
		Token: req.Token,
		Count: total,
		Data:  targetHolders,
	}, nil
}

func (s *webapiServer) GetTokenTransactions(ctx context.Context, req *rpcpb.GetTokenTransactionsRequest) (*rpcpb.GetTransactionsInfoResponse, error) {
	hash := &crypto.HashType{}
	if err := hash.SetString(req.Token.Hash); err != nil {
		return nil, err
	}
	tokenID := &script.TokenID{
		OutPoint: types.OutPoint{
			Hash:  *hash,
			Index: req.Token.Index,
		},
	}
	allTxs, err := s.server.GetChainReader().GetTokenTransactions(tokenID)
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
		txInRange = allTxs[req.Offset:]
	} else {
		txInRange = allTxs[req.Offset : req.Offset+req.Limit]
	}
	utxos, err := s.loadUtxoForTx(txInRange)
	if err != nil {
		return nil, err
	}
	logger.Infof("%v txs found related to token %v", len(txInRange))
	txInfos, err := convertTransactionInfos(txInRange, utxos)
	if err != nil {
		return nil, err
	}
	return &rpcpb.GetTransactionsInfoResponse{
		Total: total,
		Txs:   txInfos,
	}, nil
}

func (s *webapiServer) GetPendingTransaction(ctx context.Context, req *rpcpb.GetPendingTransactionRequest) (*rpcpb.GetTransactionsInfoResponse, error) {
	txs := s.server.GetTxHandler().GetTransactionsInPool()
	var txInRange []*types.Transaction
	if len(txs) <= int(req.Offset) {
		txInRange = []*types.Transaction{}
	} else if len(txs) < int(req.Offset+req.Limit) {
		txInRange = txs[req.Offset:]
	} else {
		txInRange = txs[req.Offset : req.Offset+req.Limit]
	}
	utxos, err := s.loadUtxoForTx(txInRange)
	if err != nil {
		return nil, err
	}
	logger.Debugf("utxos %v", utxos)
	var txInfos []*rpcpb.TransactionInfo
	for _, tx := range txInRange {
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
				totalIn += wrap.Output.Value
				logger.Debugf("input value %v", wrap.Output.Value)
			} else {
				return nil, fmt.Errorf("previous input not found %v", in)
			}
		}
		for _, out := range tx.Vout {
			totalOut += out.Value
		}
		fee := totalIn - totalOut
		var outInfos []*rpcpb.TxOutInfo
		for _, o := range txPb.Vout {
			outInfo, err := convertVout(o)
			if err != nil {
				return nil, err
			}
			outInfos = append(outInfos, outInfo)
		}
		txInfo := &rpcpb.TransactionInfo{
			Version:  tx.Version,
			Vin:      txPb.Vin,
			Vout:     outInfos,
			Data:     txPb.Data,
			Magic:    tx.Magic,
			LockTime: tx.LockTime,
			Hash:     hash.String(),
			Fee:      fee,
			Size_:    0,
		}
		txInfos = append(txInfos, txInfo)
	}
	return &rpcpb.GetTransactionsInfoResponse{
		Total: uint32(len(txs)),
		Txs:   txInfos,
	}, nil
}

func convertVout(vout *corepb.TxOut) (*rpcpb.TxOutInfo, error) {
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
		out.IssueInfo = &rpcpb.TokenIssueInfo{
			Name:        params.Name,
			TotalSupply: params.TotalSupply,
		}
	} else if sc.IsTokenTransfer() {
		params, err := sc.GetTransferParams()
		if err != nil {
			return nil, err
		}
		tokenID := &rpcpb.Token{
			Hash:  params.Hash.String(),
			Index: params.Index,
		}
		out.TransferInfo = &rpcpb.TokenTransferInfo{
			Token:  tokenID,
			Amount: params.Amount,
		}
	}
	return out, nil
}

func (s *webapiServer) GetTransactionHistory(ctx context.Context, req *rpcpb.GetTransactionHistoryRequest) (*rpcpb.GetTransactionsInfoResponse, error) {
	addr := &types.AddressPubKeyHash{}
	if err := addr.SetString(req.Addr); err != nil {
		return nil, err
	}
	txs, err := s.server.GetChainReader().GetTransactionsByAddr(addr)
	if err != nil {
		return nil, err
	}
	var txInRange []*types.Transaction
	if len(txs) <= int(req.Offset) {
		txInRange = []*types.Transaction{}
	} else if len(txs) < int(req.Offset+req.Limit) {
		txInRange = txs[req.Offset:]
	} else {
		txInRange = txs[req.Offset : req.Offset+req.Limit]
	}
	logger.Infof("transactions inf range: %v", len(txInRange))
	utxos, err := s.loadUtxoForTx(txInRange)
	logger.Debugf("utxos %v", util.PrettyPrint(utxos))
	if err != nil {
		return nil, err
	}
	txInfos, err := convertTransactionInfos(txInRange, utxos)
	return &rpcpb.GetTransactionsInfoResponse{
		Total: uint32(len(txs)),
		Txs:   txInfos,
	}, nil
}

func (s *webapiServer) GetTopHolders(ctx context.Context, req *rpcpb.GetTopHoldersRequest) (*rpcpb.GetTopHoldersResponse, error) {
	utxos, err := s.server.GetChainReader().ListAllUtxos()
	if err != nil {
		return nil, err
	}
	distribute := s.analyzeDistribute(utxos)
	var holders, targetHolders []*rpcpb.AddressAmount
	for addr, val := range distribute {
		holders = append(holders, &rpcpb.AddressAmount{
			Addr:   addr,
			Amount: val,
		})
	}
	sort.Slice(holders, func(i, j int) bool {
		return holders[i].Amount > holders[j].Amount
	})
	if len(holders) <= int(req.Offset) {
		targetHolders = []*rpcpb.AddressAmount{}
	} else if len(holders) < int(req.Offset+req.Limit) {
		targetHolders = holders[req.Offset:]
	} else {
		targetHolders = holders[req.Offset : req.Offset+req.Limit]
	}
	return &rpcpb.GetTopHoldersResponse{
		Total: uint32(len(holders)),
		Data:  targetHolders,
	}, nil
}

func (s *webapiServer) GetHolderCount(context.Context, *rpcpb.GetHolderCountRequest) (*rpcpb.GetHolderCountResponse, error) {
	utxos, err := s.server.GetChainReader().ListAllUtxos()
	if err != nil {
		return nil, err
	}
	total := s.countAddresses(utxos)
	return &rpcpb.GetHolderCountResponse{HolderCount: total}, nil
}

func (s *webapiServer) GetTransaction(ctx context.Context, req *rpcpb.GetTransactionInfoRequest) (*rpcpb.TransactionInfo, error) {
	hash := &crypto.HashType{}
	if err := hash.SetString(req.Hash); err != nil {
		return nil, err
	}
	tx, err := s.server.GetChainReader().LoadTxByHash(*hash)
	if err != nil {
		return nil, err
	}
	txInfo, err := convertTransaction(tx)
	if err != nil {
		return nil, err
	}
	return txInfo, nil
}

func (s *webapiServer) GetBlock(ctx context.Context, req *rpcpb.GetBlockInfoRequest) (*rpcpb.BlockInfo, error) {
	hash := &crypto.HashType{}
	if err := hash.SetString(req.Hash); err != nil {
		return nil, err
	}
	block, err := s.server.GetChainReader().LoadBlockByHash(*hash)
	if err != nil {
		return nil, err
	}
	blockInfo, err := convertBlock(block)
	if err != nil {
		return nil, err
	}
	if err := s.calcMiningFee(block, blockInfo); err != nil {
		return nil, err
	}
	return blockInfo, nil
}

func (s *webapiServer) countAddresses(utxos map[types.OutPoint]*types.UtxoWrap) uint32 {
	addrs := make(map[string]bool)
	for _, wrap := range utxos {
		sc := script.NewScriptFromBytes(wrap.Output.ScriptPubKey)
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
		sc := script.NewScriptFromBytes(wrap.Output.ScriptPubKey)
		addr, err := sc.ExtractAddress()
		if err != nil || addr == nil {
			continue
		}
		addrStr := addr.String()
		if val, ok := distribute[addrStr]; ok {
			distribute[addrStr] = val + wrap.Output.Value
		} else {
			distribute[addrStr] = wrap.Output.Value
		}
	}
	return distribute
}

func (s *webapiServer) analyzeTokenDistribute(utxos map[types.OutPoint]*types.UtxoWrap, token *script.TokenID) (map[string]uint64, error) {
	distribute := make(map[string]uint64)
	for op, wrap := range utxos {
		sc := script.NewScriptFromBytes(wrap.Output.ScriptPubKey)
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
			wrap := &types.UtxoWrap{
				Output:      out,
				BlockHeight: 0,
				IsCoinBase:  i == 0,
				IsSpent:     false,
				IsModified:  false,
			}
			generated[outpoint] = wrap
		}
	}
	var missing []types.OutPoint
	for _, tx := range txs {
		for _, txIn := range tx.Vin {
			if _, ok := generated[txIn.PrevOutPoint]; ok {
				continue
			}
			missing = append(missing, txIn.PrevOutPoint)
		}
	}
	stored, err := s.server.GetChainReader().LoadSpentUtxos(missing)
	if err != nil {
		return nil, err
	}
	for k, v := range stored {
		generated[k] = v
	}
	return generated, nil
}

func (s *webapiServer) calcMiningFee(block *types.Block, blockInfo *rpcpb.BlockInfo) error {
	utxos, err := s.loadUtxoForTx(block.Txs)
	if err != nil {
		return err
	}
	for i, tx := range block.Txs {
		var totalIn, totalOut uint64
		for _, vout := range tx.Vout {
			totalOut += vout.Value
		}
		if i != 0 {
			for _, vin := range tx.Vin {
				if wrap, ok := utxos[vin.PrevOutPoint]; ok {
					totalIn += wrap.Value()
					continue
				}
				return fmt.Errorf("previous outpoint not found for %v", vin.PrevOutPoint)
			}
			blockInfo.Txs[i].Fee = totalIn - totalOut
		} else {
			blockInfo.Txs[i].Fee = 0
		}
	}
	return nil
}

func convertTransactionInfos(txs []*types.Transaction, utxos map[types.OutPoint]*types.UtxoWrap) ([]*rpcpb.TransactionInfo, error) {
	var result []*rpcpb.TransactionInfo
	for _, tx := range txs {
		txPb, err := convertTransaction(tx)
		if err != nil {
			return nil, err
		}
		var totalIn, totalOut uint64
		for _, in := range tx.Vin {
			prev, ok := utxos[in.PrevOutPoint]
			if !ok {
				return nil, fmt.Errorf("previous transaction not found for outpoint: %v", in.PrevOutPoint)
			}
			totalIn += prev.Output.Value
		}
		for _, out := range tx.Vout {
			totalOut += out.Value
		}
		txPb.Fee = totalIn - totalOut
		result = append(result, txPb)
	}
	return result, nil
}

func convertTransaction(tx *types.Transaction) (*rpcpb.TransactionInfo, error) {
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
	for _, o := range txPb.Vout {
		outInfo, err := convertVout(o)
		if err != nil {
			return nil, err
		}
		outInfos = append(outInfos, outInfo)
	}
	out := &rpcpb.TransactionInfo{
		Version:  tx.Version,
		Vin:      txPb.Vin,
		Vout:     outInfos,
		Data:     txPb.Data,
		Magic:    txPb.Magic,
		LockTime: txPb.LockTime,
		Hash:     hash.String(),
		Size_:    uint64(len(bts)),
	}
	return out, nil
}

func convertTransactions(txs []*types.Transaction) ([]*rpcpb.TransactionInfo, error) {
	var txsPb []*rpcpb.TransactionInfo
	for _, tx := range txs {
		txPb, err := convertTransaction(tx)
		if err != nil {
			return nil, err
		}
		txsPb = append(txsPb, txPb)
	}
	return txsPb, nil
}

func convertBlock(block *types.Block) (*rpcpb.BlockInfo, error) {

	headerPb, err := convertHeader(block.Header)
	if err != nil {
		return nil, err
	}
	bts, err := block.Marshal()
	if err != nil {
		return nil, err
	}
	txsPb, err := convertTransactions(block.Txs)
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
