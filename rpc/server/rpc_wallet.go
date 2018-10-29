// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package rpc

import (
	"context"

	"github.com/BOXFoundation/boxd/core/pb"
	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/rpc/pb"
)

func registerWallet(s *Server) {
	rpcpb.RegisterWalletCommandServer(s.server, &wltServer{server: s})
}

func init() {
	RegisterServiceWithGatewayHandler(
		"wlt",
		registerWallet,
		rpcpb.RegisterWalletCommandHandlerFromEndpoint,
	)
}

type wltServer struct {
	server GRPCServer
}

func (s *wltServer) ListTransactions(ctx context.Context, req *rpcpb.ListTransactionsRequest) (*rpcpb.ListTransactionsResponse, error) {
	addr := &types.AddressPubKeyHash{}
	if err := addr.SetString(req.Addr); err != nil {
		return &rpcpb.ListTransactionsResponse{Code: -1, Message: "Invalid Address"}, err
	}
	logger.Infof("Search Transaction related to address: %s", addr.String())
	txs, err := s.server.GetChainReader().GetTransactions(addr)
	if err != nil {
		return &rpcpb.ListTransactionsResponse{Code: -1, Message: "Error Searching Transactions"}, err
	}
	transactions := make([]*corepb.Transaction, len(txs))
	for i, tx := range txs {
		txProto, err := tx.ToProtoMessage()
		if err != nil {
			return &rpcpb.ListTransactionsResponse{Code: -1, Message: "Error Searching Transactions"}, err
		}
		transactions[i] = txProto.(*corepb.Transaction)
	}
	return &rpcpb.ListTransactionsResponse{Code: 0, Message: "Ok", Transactions: transactions}, nil
}
func (s *wltServer) GetTransactionCount(context.Context, *rpcpb.GetTransactionCountRequest) (*rpcpb.GetTransactionCountResponse, error) {
	return &rpcpb.GetTransactionCountResponse{}, nil
}
