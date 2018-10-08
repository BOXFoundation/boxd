// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package rpc

import (
	"context"

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
	return &rpcpb.ListTransactionsResponse{Code: 0, Message: "Ok"}, nil
}
func (s *wltServer) GetTransactionCount(context.Context, *rpcpb.GetTransactionCountRequest) (*rpcpb.GetTransactionCountResponse, error) {
	return &rpcpb.GetTransactionCountResponse{}, nil
}
