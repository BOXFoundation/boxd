// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package rpc

import (
	"context"
	"fmt"
	"github.com/BOXFoundation/boxd/p2p/pstore"

	"github.com/BOXFoundation/boxd/boxd/eventbus"
	"github.com/BOXFoundation/boxd/core/pb"
	"github.com/BOXFoundation/boxd/crypto"
	"github.com/BOXFoundation/boxd/rpc/pb"
)

func registerControl(s *Server) {
	rpcpb.RegisterContorlCommandServer(s.server, &ctlserver{server: s})
}

func init() {
	RegisterServiceWithGatewayHandler(
		"control",
		registerControl,
		rpcpb.RegisterContorlCommandHandlerFromEndpoint,
	)
}

type ctlserver struct {
	server GRPCServer
}

func (s *ctlserver) GetNodeInfo(ctx context.Context, req *rpcpb.GetNodeInfoRequest) (*rpcpb.GetNodeInfoResponse, error) {
	bus := s.server.GetEventBus()
	ch := make(chan []pstore.NodeInfo)
	bus.Send(eventbus.TopicGetAddressBook, ch)
	defer close(ch)
	nodes := <-ch
	resp := &rpcpb.GetNodeInfoResponse{}
	for _, n := range nodes {
		resp.Nodes = append(resp.Nodes, &rpcpb.Node{
			Id:    n.PeerID.Pretty(),
			Addrs: n.Addr,
			Ttl:   n.TTL.String(),
		})
	}
	return resp, nil
}

// SetDebugLevel implements SetDebugLevel
func (s *ctlserver) SetDebugLevel(ctx context.Context, in *rpcpb.DebugLevelRequest) (*rpcpb.BaseResponse, error) {
	bus := s.server.GetEventBus()
	ch := make(chan bool)
	bus.Send(eventbus.TopicSetDebugLevel, in.Level, ch)
	if <-ch {
		var info = fmt.Sprintf("Set debug level: %s", logger.LogLevel())
		return &rpcpb.BaseResponse{Code: 0, Message: info}, nil
	}
	var info = fmt.Sprintf("Wrong debug level: %s", in.Level)
	return &rpcpb.BaseResponse{Code: 1, Message: info}, nil
}

func (s *ctlserver) GetBlockHeight(ctx context.Context, req *rpcpb.GetBlockHeightRequest) (*rpcpb.GetBlockHeightResponse, error) {
	height := s.server.GetChainReader().GetBlockHeight()
	return &rpcpb.GetBlockHeightResponse{
		Code:    0,
		Message: "ok",
		Height:  height,
	}, nil
}

func (s *ctlserver) GetBlockHash(ctx context.Context, req *rpcpb.GetBlockHashRequest) (*rpcpb.GetBlockHashResponse, error) {
	hash, err := s.server.GetChainReader().GetBlockHash(req.Height)
	if err != nil {
		return &rpcpb.GetBlockHashResponse{
			Code:    -1,
			Message: err.Error(),
		}, err
	}
	return &rpcpb.GetBlockHashResponse{
		Code:    0,
		Message: "ok",
		Hash:    hash.String(),
	}, nil
}

func (s *ctlserver) GetBlockHeader(ctx context.Context, req *rpcpb.GetBlockRequest) (*rpcpb.GetBlockHeaderResponse, error) {
	hash := &crypto.HashType{}
	err := hash.SetString(req.BlockHash)
	if err != nil {
		return &rpcpb.GetBlockHeaderResponse{
			Code:    -1,
			Message: fmt.Sprintf("Invalid hash: %s", req.BlockHash),
		}, err
	}
	block, err := s.server.GetChainReader().LoadBlockByHash(*hash)
	if err != nil {
		return &rpcpb.GetBlockHeaderResponse{
			Code:    -1,
			Message: err.Error(),
		}, err
	}
	msg, err := block.Header.ToProtoMessage()
	if err != nil {
		return &rpcpb.GetBlockHeaderResponse{
			Code:    -1,
			Message: err.Error(),
		}, err
	}
	if header, ok := msg.(*corepb.BlockHeader); ok {
		return &rpcpb.GetBlockHeaderResponse{
			Code:    0,
			Message: "ok",
			Header:  header,
		}, nil
	}
	return &rpcpb.GetBlockHeaderResponse{
		Code:    -1,
		Message: "Internal Error",
	}, fmt.Errorf("Error converting proto message")
}

func (s *ctlserver) GetBlock(ctx context.Context, req *rpcpb.GetBlockRequest) (*rpcpb.GetBlockResponse, error) {
	hash := &crypto.HashType{}
	err := hash.SetString(req.BlockHash)
	if err != nil {
		return &rpcpb.GetBlockResponse{
			Code:    -1,
			Message: fmt.Sprintf("Invalid hash: %s", req.BlockHash),
		}, err
	}
	block, err := s.server.GetChainReader().LoadBlockByHash(*hash)
	if err != nil {
		return &rpcpb.GetBlockResponse{
			Code:    -1,
			Message: fmt.Sprintf("Error searching block: %s", req.BlockHash),
		}, err
	}
	msg, err := block.ToProtoMessage()
	if err != nil {
		return &rpcpb.GetBlockResponse{
			Code:    -1,
			Message: err.Error(),
		}, err
	}
	if blockPb, ok := msg.(*corepb.Block); ok {
		return &rpcpb.GetBlockResponse{
			Code:    0,
			Message: "ok",
			Block:   blockPb,
		}, nil
	}
	return &rpcpb.GetBlockResponse{
		Code:    -1,
		Message: "Internal Error",
	}, fmt.Errorf("Error converting proto message")
}
