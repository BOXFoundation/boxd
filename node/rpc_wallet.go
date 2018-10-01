// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package node

import (
	"context"

	"github.com/BOXFoundation/Quicksilver/rpc/pb"
	rpcserver "github.com/BOXFoundation/Quicksilver/rpc/server"
	"google.golang.org/grpc"
)

func registerWallet(s *grpc.Server) {
	rpcpb.RegisterWalletCommandServer(s, &wltServer{})
}

func init() {
	rpcserver.RegisterServiceWithGatewayHandler(
		"wlt",
		registerWallet,
		rpcpb.RegisterWalletCommandHandlerFromEndpoint,
	)
}

type wltServer struct{}

func (s *wltServer) ListTransactions(ctx context.Context, req *rpcpb.ListTransactionsRequest) (*rpcpb.ListTransactionsResponse, error) {
	return &rpcpb.ListTransactionsResponse{Code: 0, Message: "Ok"}, nil
}
func (s *wltServer) GetTransactionCount(context.Context, *rpcpb.GetTransactionCountRequest) (*rpcpb.GetTransactionCountResponse, error) {
	return &rpcpb.GetTransactionCountResponse{}, nil
}
