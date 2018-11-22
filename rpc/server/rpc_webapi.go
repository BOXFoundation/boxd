// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package rpc

import (
	"fmt"

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
	return blockInfo, nil
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
	out := &rpcpb.TransactionInfo{
		Version:  tx.Version,
		Vin:      txPb.Vin,
		Vout:     txPb.Vout,
		Data:     txPb.Data,
		Magic:    txPb.Magic,
		LockTime: txPb.LockTime,
		Hash:     hash.String(),
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
	txsPb, err := convertTransactions(block.Txs)
	if err != nil {
		return nil, err
	}
	out := &rpcpb.BlockInfo{
		Header:    headerPb,
		Txs:       txsPb,
		Height:    block.Height,
		Signature: block.Signature,
		Hash:      block.Hash.String(),
	}
	return out, nil
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
