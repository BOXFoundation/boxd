// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package rpc

import (
	"fmt"
	"sort"

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

func (s *webapiServer) GetPendingTransaction(context.Context, *rpcpb.GetPendingTransactionRequest) (*rpcpb.GetTransactionsInfoResponse, error) {
	panic("implement me")
}

func (s *webapiServer) GetTransactionHistory(context.Context, *rpcpb.GetTransactionHistoryRequest) (*rpcpb.GetTransactionsInfoResponse, error) {
	panic("implement me")
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

func (s *webapiServer) calcMiningFee(block *types.Block, blockInfo *rpcpb.BlockInfo) error {
	generated := make(map[types.OutPoint]*types.UtxoWrap)
	for i, tx := range block.Txs {
		hash, err := tx.TxHash()
		if err != nil {
			return err
		}
		for idx, out := range tx.Vout {
			outpoint := types.OutPoint{
				Hash:  *hash,
				Index: uint32(idx),
			}
			wrap := &types.UtxoWrap{
				Output:      out,
				BlockHeight: block.Height,
				IsCoinBase:  i == 0,
				IsSpent:     false,
				IsModified:  false,
			}
			generated[outpoint] = wrap
		}
	}
	var missing []types.OutPoint
	for i, tx := range block.Txs {
		if i == 0 {
			continue
		}
		for _, txIn := range tx.Vin {
			if _, ok := generated[txIn.PrevOutPoint]; ok {
				continue
			}
			missing = append(missing, txIn.PrevOutPoint)
		}
	}
	stored, err := s.server.GetChainReader().LoadSpentUtxos(missing)
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
				if wrap, ok := stored[vin.PrevOutPoint]; ok {
					totalIn += wrap.Value()
					continue
				}
				if wrap, ok := generated[vin.PrevOutPoint]; ok {
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
	out := &rpcpb.TransactionInfo{
		Version:  tx.Version,
		Vin:      txPb.Vin,
		Vout:     txPb.Vout,
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
