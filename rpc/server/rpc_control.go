// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package rpc

import (
	"context"
	"fmt"

	"github.com/BOXFoundation/boxd/boxd/eventbus"
	corepb "github.com/BOXFoundation/boxd/core/pb"
	"github.com/BOXFoundation/boxd/crypto"
	"github.com/BOXFoundation/boxd/p2p"
	"github.com/BOXFoundation/boxd/p2p/pstore"
	rpcpb "github.com/BOXFoundation/boxd/rpc/pb"
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

func (s *ctlserver) AddNode(ctx context.Context, req *rpcpb.AddNodeRequest) (*rpcpb.BaseResponse, error) {
	out := make(chan error)
	s.server.GetEventBus().Send(eventbus.TopicP2PAddPeer, req.Node, out)
	if err := <-out; err != nil {
		return &rpcpb.BaseResponse{
			Code:    -1,
			Message: err.Error(),
		}, err
	}
	return &rpcpb.BaseResponse{
		Code:    0,
		Message: "ok",
	}, nil
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

// GetNetworkID returns
func (s *ctlserver) GetNetworkID(ctx context.Context, req *rpcpb.GetNetworkIDRequest) (*rpcpb.GetNetworkIDResponse, error) {
	ch := make(chan uint32)
	s.server.GetEventBus().Send(eventbus.TopicGetNetworkID, ch)
	current := <-ch
	var literal string
	if current == p2p.Mainnet {
		literal = "Mainnet"
	} else if current == p2p.Testnet {
		literal = "Testnet"
	} else {
		literal = "Unknown"
	}
	return &rpcpb.GetNetworkIDResponse{
		Id:      current,
		Literal: literal,
	}, nil
}

// UpdateNetworkID implements UpdateNetworkID
// NOTE: should be remove in product env
func (s *ctlserver) UpdateNetworkID(ctx context.Context, in *rpcpb.UpdateNetworkIDRequest) (*rpcpb.BaseResponse, error) {
	bus := s.server.GetEventBus()
	ch := make(chan bool)
	bus.Send(eventbus.TopicUpdateNetworkID, in.Id, ch)
	if <-ch {
		var info = fmt.Sprintf("Update NetworkID: %d", in.Id)
		return &rpcpb.BaseResponse{Code: 0, Message: info}, nil
	}
	var info = fmt.Sprintf("Wrong NetworkID: %d", in.Id)
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
	block, _, err := s.server.GetChainReader().ReadBlockFromDB(hash)
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
	block, _, err := s.server.GetChainReader().ReadBlockFromDB(hash)
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
func (s *ctlserver) GetBlockByHeight(ctx context.Context, req *rpcpb.GetBlockByHeightReq) (*rpcpb.GetBlockResponse, error) {
	hash, err := s.server.GetChainReader().GetBlockHash(req.Height)
	if err != nil {
		return &rpcpb.GetBlockResponse{
			Code:    -1,
			Message: err.Error(),
		}, err
	}
	block, _, err := s.server.GetChainReader().ReadBlockFromDB(hash)
	if err != nil {
		return &rpcpb.GetBlockResponse{
			Code:    -1,
			Message: "Invalid hash",
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

func (s *ctlserver) GetBlockTransactionCountByHash(ctx context.Context,
	req *rpcpb.GetBlockTransactionCountByHashReq) (*rpcpb.GetBlockTxCountResp, error) {
	var num uint32
	hash := &crypto.HashType{}
	err := hash.SetString(req.BlockHash)
	if err != nil {
		return &rpcpb.GetBlockTxCountResp{
			Code:    -1,
			Message: fmt.Sprintf("Invalid hash: %s", req.BlockHash),
		}, err
	}
	block, _, err := s.server.GetChainReader().ReadBlockFromDB(hash)
	if err != nil {
		return &rpcpb.GetBlockTxCountResp{
			Code:    -1,
			Message: fmt.Sprintf("Error searching block: %s", req.BlockHash),
		}, err
	}
	msg, err := block.ToProtoMessage()
	if err != nil {
		return &rpcpb.GetBlockTxCountResp{
			Code:    -1,
			Message: err.Error(),
		}, err
	}
	blockPb, ok := msg.(*corepb.Block)

	if !ok {
		return &rpcpb.GetBlockTxCountResp{
			Code:    0,
			Message: "can't convert proto message",
		}, fmt.Errorf("Error converting proto message")
	}
	for i := 0; i < len(blockPb.Txs); i++ {
		num++
	}
	return &rpcpb.GetBlockTxCountResp{
		Code:    0,
		Message: "ok",
		Count:   num,
	}, nil

}
func (s *ctlserver) GetBlockTransactionCountByHeight(ctx context.Context,
	req *rpcpb.GetBlockTransactionCountByHeightReq) (*rpcpb.GetBlockTxCountResp, error) {
	var num uint32
	hash, err := s.server.GetChainReader().GetBlockHash(req.Height)
	if err != nil {
		return &rpcpb.GetBlockTxCountResp{
			Code:    -1,
			Message: err.Error(),
		}, err
	}
	block, _, err := s.server.GetChainReader().ReadBlockFromDB(hash)
	if err != nil {
		return &rpcpb.GetBlockTxCountResp{
			Code:    -1,
			Message: "Invalid hash",
		}, err
	}
	msg, err := block.ToProtoMessage()
	if err != nil {
		return &rpcpb.GetBlockTxCountResp{
			Code:    -1,
			Message: err.Error(),
		}, err
	}
	blockPb, ok := msg.(*corepb.Block)

	if !ok {
		return &rpcpb.GetBlockTxCountResp{
			Code:    0,
			Message: "can't convert proto message",
		}, fmt.Errorf("Error converting proto message")
	}
	for i := 0; i < len(blockPb.Txs); i++ {
		num++
	}
	return &rpcpb.GetBlockTxCountResp{
		Code:    0,
		Message: "ok",
		Count:   num,
	}, nil

}

func (s *ctlserver) GetTransactionByBlockHashAndIndex(ctx context.Context,
	req *rpcpb.GetTransactionByBlockHashAndIndexReq) (*rpcpb.GetTxResp, error) {
	hash := &crypto.HashType{}
	err := hash.SetString(req.BlockHash)
	if err != nil {
		return &rpcpb.GetTxResp{
			Code:    -1,
			Message: fmt.Sprintf("Invalid hash: %s", req.BlockHash),
		}, err
	}
	block, _, err := s.server.GetChainReader().ReadBlockFromDB(hash)
	if err != nil {
		return &rpcpb.GetTxResp{
			Code:    -1,
			Message: fmt.Sprintf("Error searching block: %s", req.BlockHash),
		}, err
	}
	msg, err := block.ToProtoMessage()
	if err != nil {
		return &rpcpb.GetTxResp{
			Code:    -1,
			Message: err.Error(),
		}, err
	}
	blockPb, ok := msg.(*corepb.Block)

	if !ok {
		return &rpcpb.GetTxResp{
			Code:    0,
			Message: "can't convert proto message",
		}, fmt.Errorf("Error converting proto message")
	}
	for k, v := range blockPb.Txs {
		if int32(k) == req.Index {
			return &rpcpb.GetTxResp{
				Code:    0,
				Message: "ok",
				Tx:      v,
			}, nil
		}
	}
	return &rpcpb.GetTxResp{
			Code:    -1,
			Message: "can't find Tx",
		}, fmt.Errorf("Can't find the index is %d Tx in hash %v block",
			req.Index, req.BlockHash)
}

func (s *ctlserver) GetTransactionByBlockHeightAndIndex(ctx context.Context,
	req *rpcpb.GetTransactionByBlockHeightAndIndexReq) (*rpcpb.GetTxResp, error) {
	hash, err := s.server.GetChainReader().GetBlockHash(req.Height)
	if err != nil {
		return &rpcpb.GetTxResp{
			Code:    -1,
			Message: err.Error(),
		}, err
	}
	block, _, err := s.server.GetChainReader().ReadBlockFromDB(hash)
	if err != nil {
		return &rpcpb.GetTxResp{
			Code:    -1,
			Message: "Invalid hash",
		}, err
	}
	msg, err := block.ToProtoMessage()
	if err != nil {
		return &rpcpb.GetTxResp{
			Code:    -1,
			Message: err.Error(),
		}, err
	}
	blockPb, ok := msg.(*corepb.Block)

	if !ok {
		return &rpcpb.GetTxResp{
			Code:    0,
			Message: "can't convert proto message",
		}, fmt.Errorf("Error converting proto message")
	}
	for k, v := range blockPb.Txs {
		if int32(k) == req.Index {
			return &rpcpb.GetTxResp{
				Code:    0,
				Message: "ok",
				Tx:      v,
			}, nil
		}
	}
	return &rpcpb.GetTxResp{
			Code:    -1,
			Message: "can't find Tx",
		}, fmt.Errorf("Can't find the index is %d Tx in height %d block",
			req.Index, req.Height)

}
