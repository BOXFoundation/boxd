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
		return &rpcpb.BaseResponse{Code: -1, Message: err.Error()}, nil
	}
	return &rpcpb.BaseResponse{Code: 0, Message: "ok"}, nil
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
	return &rpcpb.GetNetworkIDResponse{Id: current, Literal: literal}, nil
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
	return &rpcpb.GetBlockHeightResponse{Code: 0, Message: "ok", Height: height}, nil
}
func newGetBlockHashResponse(code int32, msg string, hash string) *rpcpb.GetBlockHashResponse {
	return &rpcpb.GetBlockHashResponse{
		Code:    code,
		Message: msg,
		Hash:    hash,
	}
}

func (s *ctlserver) GetBlockHash(ctx context.Context, req *rpcpb.GetBlockHashRequest) (*rpcpb.GetBlockHashResponse, error) {
	hash, err := s.server.GetChainReader().GetBlockHash(req.Height)
	if err != nil {
		return newGetBlockHashResponse(-1, err.Error(), ""), nil
	}
	return newGetBlockHashResponse(0, "ok", hash.String()), nil
}

func newGetBlockHeaderResponse(code int32, msg string, header *corepb.BlockHeader) *rpcpb.GetBlockHeaderResponse {
	return &rpcpb.GetBlockHeaderResponse{
		Code:    code,
		Message: msg,
		Header:  header,
	}
}

func (s *ctlserver) GetBlockHeader(ctx context.Context, req *rpcpb.GetBlockRequest) (*rpcpb.GetBlockHeaderResponse, error) {
	hash := &crypto.HashType{}
	err := hash.SetString(req.BlockHash)
	if err != nil {
		return newGetBlockHeaderResponse(-1, fmt.Sprintf("Invalid hash: %s", req.BlockHash), nil), nil
	}
	block, _, err := s.server.GetChainReader().ReadBlockFromDB(hash)
	if err != nil {
		return newGetBlockHeaderResponse(-1, err.Error(), nil), nil
	}
	msg, err := block.Header.ToProtoMessage()
	if err != nil {
		return newGetBlockHeaderResponse(-1, err.Error(), nil), nil
	}
	if header, ok := msg.(*corepb.BlockHeader); ok {
		return newGetBlockHeaderResponse(0, "ok", header), nil
	}
	return newGetBlockHeaderResponse(-1, "Internal Error", nil), fmt.Errorf("Error converting proto message")
}

func newGetBlockResponse(code int32, msg string, block *corepb.Block) *rpcpb.GetBlockResponse {
	return &rpcpb.GetBlockResponse{
		Code:    code,
		Message: msg,
		Block:   block,
	}
}

func (s *ctlserver) GetBlock(ctx context.Context, req *rpcpb.GetBlockRequest) (*rpcpb.GetBlockResponse, error) {
	hash := &crypto.HashType{}
	err := hash.SetString(req.BlockHash)
	if err != nil {
		return newGetBlockResponse(-1, fmt.Sprintf("Invalid hash: %s", req.BlockHash), nil), nil
	}
	block, _, err := s.server.GetChainReader().ReadBlockFromDB(hash)
	if err != nil {
		return newGetBlockResponse(-1, fmt.Sprintf("Error searching block: %s", req.BlockHash), nil), nil
	}
	msg, err := block.ToProtoMessage()
	if err != nil {
		return newGetBlockResponse(-1, err.Error(), nil), nil
	}
	if blockPb, ok := msg.(*corepb.Block); ok {
		return newGetBlockResponse(0, "ok", blockPb), nil
	}
	return newGetBlockResponse(-1, "Internal Error", nil), fmt.Errorf("Error converting proto message")
}

func (s *ctlserver) GetBlockByHeight(ctx context.Context, req *rpcpb.GetBlockByHeightReq) (*rpcpb.GetBlockResponse, error) {
	hash, err := s.server.GetChainReader().GetBlockHash(req.Height)
	if err != nil {
		return newGetBlockResponse(-1, err.Error(), nil), nil
	}
	block, _, err := s.server.GetChainReader().ReadBlockFromDB(hash)
	if err != nil {
		return newGetBlockResponse(-1, "Invalid hash", nil), nil
	}
	msg, err := block.ToProtoMessage()
	if err != nil {
		return newGetBlockResponse(-1, err.Error(), nil), nil
	}
	if blockPb, ok := msg.(*corepb.Block); ok {
		return newGetBlockResponse(0, "ok", blockPb), nil
	}
	return newGetBlockResponse(-1, "Internal Error", nil), fmt.Errorf("Error converting proto message")

}

func newGetBlockTxCountResp(code int32, message string, count uint32) *rpcpb.GetBlockTxCountResp {
	return &rpcpb.GetBlockTxCountResp{
		Code:    code,
		Message: message,
		Count:   count,
	}
}

func (s *ctlserver) GetBlockTransactionCountByHash(ctx context.Context,
	req *rpcpb.GetBlockTransactionCountByHashReq) (*rpcpb.GetBlockTxCountResp, error) {
	var num uint32
	hash := &crypto.HashType{}
	err := hash.SetString(req.BlockHash)
	if err != nil {
		return newGetBlockTxCountResp(-1, fmt.Sprintf("Invalid hash: %s", req.BlockHash), 0), nil
	}
	block, _, err := s.server.GetChainReader().ReadBlockFromDB(hash)
	if err != nil {
		return newGetBlockTxCountResp(-1, fmt.Sprintf("Error searching block: %s", req.BlockHash), 0), nil
	}
	msg, err := block.ToProtoMessage()
	if err != nil {
		return newGetBlockTxCountResp(-1, err.Error(), 0), nil
	}
	blockPb, ok := msg.(*corepb.Block)

	if !ok {
		return newGetBlockTxCountResp(-1, "can't convert proto message", 0), fmt.Errorf("Error converting proto message")
	}
	for i := 0; i < len(blockPb.Txs); i++ {
		num++
	}
	return newGetBlockTxCountResp(0, "ok", num), nil
}
func (s *ctlserver) GetBlockTransactionCountByHeight(ctx context.Context,
	req *rpcpb.GetBlockTransactionCountByHeightReq) (*rpcpb.GetBlockTxCountResp, error) {
	var num uint32
	hash, err := s.server.GetChainReader().GetBlockHash(req.Height)
	if err != nil {
		return newGetBlockTxCountResp(-1, err.Error(), 0), nil
	}
	block, _, err := s.server.GetChainReader().ReadBlockFromDB(hash)
	if err != nil {
		return newGetBlockTxCountResp(-1, "Invalid hash", 0), nil
	}
	msg, err := block.ToProtoMessage()
	if err != nil {
		return newGetBlockTxCountResp(-1, err.Error(), 0), nil
	}
	blockPb, ok := msg.(*corepb.Block)

	if !ok {
		return newGetBlockTxCountResp(-1, "can't convert proto message", 0), fmt.Errorf("Error converting proto message")
	}
	for i := 0; i < len(blockPb.Txs); i++ {
		num++
	}
	return newGetBlockTxCountResp(0, "ok", num), nil
}

func newGetTxResp(code int32, message string, tx *corepb.Transaction) *rpcpb.GetTxResp {
	return &rpcpb.GetTxResp{
		Code:    code,
		Message: message,
		Tx:      tx,
	}
}

func (s *ctlserver) GetTransactionByBlockHashAndIndex(ctx context.Context,
	req *rpcpb.GetTransactionByBlockHashAndIndexReq) (*rpcpb.GetTxResp, error) {
	hash := &crypto.HashType{}
	err := hash.SetString(req.BlockHash)
	if err != nil {
		return newGetTxResp(-1, fmt.Sprintf("Invalid hash: %s", req.BlockHash), nil), nil
	}
	block, _, err := s.server.GetChainReader().ReadBlockFromDB(hash)
	if err != nil {
		return newGetTxResp(-1, fmt.Sprintf("Error searching block: %s", req.BlockHash), nil), nil
	}
	msg, err := block.ToProtoMessage()
	if err != nil {
		return newGetTxResp(-1, err.Error(), nil), nil
	}
	blockPb, ok := msg.(*corepb.Block)

	if !ok {
		return newGetTxResp(-1, "can't convert proto message", nil), fmt.Errorf("Error converting proto message")
	}
	for k, v := range blockPb.Txs {
		if int32(k) == req.Index {
			return newGetTxResp(0, "ok", v), nil
		}
	}
	return newGetTxResp(-1, "can't find Tx", nil), fmt.Errorf("Can't find the index is %d Tx in hash %v block", req.Index, req.BlockHash)
}

func (s *ctlserver) GetTransactionByBlockHeightAndIndex(ctx context.Context,
	req *rpcpb.GetTransactionByBlockHeightAndIndexReq) (*rpcpb.GetTxResp, error) {
	hash, err := s.server.GetChainReader().GetBlockHash(req.Height)
	if err != nil {
		return newGetTxResp(-1, err.Error(), nil), nil
	}
	block, _, err := s.server.GetChainReader().ReadBlockFromDB(hash)
	if err != nil {
		return newGetTxResp(-1, "Invalid hash", nil), nil
	}
	msg, err := block.ToProtoMessage()
	if err != nil {
		return newGetTxResp(-1, err.Error(), nil), nil
	}
	blockPb, ok := msg.(*corepb.Block)

	if !ok {
		return newGetTxResp(-1, "can't convert proto message", nil), fmt.Errorf("Error converting proto message")
	}
	for k, v := range blockPb.Txs {
		if int32(k) == req.Index {
			return newGetTxResp(0, "ok", v), nil
		}
	}
	return newGetTxResp(-1, "can't find Tx", nil), fmt.Errorf("Can't find the index is %d Tx in height %d block",
		req.Index, req.Height)

}
