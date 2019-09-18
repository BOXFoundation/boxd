// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package rpc

import (
	"context"
	"fmt"
	"math/big"

	"github.com/BOXFoundation/boxd/boxd/eventbus"
	"github.com/BOXFoundation/boxd/core/chain"
	corepb "github.com/BOXFoundation/boxd/core/pb"
	"github.com/BOXFoundation/boxd/core/types"
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

func (s *ctlserver) GetCurrentBlockHeight(ctx context.Context, req *rpcpb.GetCurrentBlockHeightRequest) (*rpcpb.GetCurrentBlockHeightResponse, error) {
	height := s.server.GetChainReader().TailBlock().Header.Height
	return &rpcpb.GetCurrentBlockHeightResponse{Code: 0, Message: "ok", Height: height}, nil
}

func (s *ctlserver) GetCurrentBlockHash(ctx context.Context, req *rpcpb.GetCurrentBlockHashRequest) (*rpcpb.GetCurrentBlockHashResponse, error) {
	hash := s.server.GetChainReader().TailBlock().Hash
	return &rpcpb.GetCurrentBlockHashResponse{Code: 0, Message: "ok", Hash: hash.String()}, nil
}

func (s *ctlserver) GetBlockHash(ctx context.Context, req *rpcpb.GetBlockHashRequest) (*rpcpb.GetBlockHashResponse, error) {
	hash, err := s.server.GetChainReader().GetBlockHash(req.Height)
	if err != nil {
		return &rpcpb.GetBlockHashResponse{Code: -1, Message: err.Error()}, nil
	}
	return &rpcpb.GetBlockHashResponse{Code: 0, Message: "ok", Hash: hash.String()}, nil
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
	return newGetBlockHeaderResponse(-1, "Internal Error", nil), nil
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
	return newGetBlockResponse(-1, "Internal Error", nil), nil
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
	return newGetBlockResponse(-1, "Internal Error", nil), nil

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
		return newGetBlockTxCountResp(-1, "can't convert proto message", 0), nil
	}
	count := len(blockPb.Txs)
	return newGetBlockTxCountResp(0, "ok", uint32(count)), nil
}
func (s *ctlserver) GetBlockTransactionCountByHeight(ctx context.Context,
	req *rpcpb.GetBlockTransactionCountByHeightReq) (*rpcpb.GetBlockTxCountResp, error) {
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
		return newGetBlockTxCountResp(-1, "can't convert proto message", 0), nil
	}
	count := len(blockPb.Txs)
	return newGetBlockTxCountResp(0, "ok", uint32(count)), nil
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
		return newGetTxResp(-1, "can't convert proto message", nil), nil
	}
	if req.Index > uint32(len(blockPb.Txs)-1) {
		return newGetTxResp(-1, "can't find Tx", nil), nil
	}
	tx := blockPb.Txs[req.Index]
	return newGetTxResp(0, "ok", tx), nil
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
		return newGetTxResp(-1, "can't convert proto message", nil), nil
	}
	if req.Index > uint32(len(blockPb.Txs)-1) {
		return newGetTxResp(-1, "can't find Tx", nil), nil
	}
	tx := blockPb.Txs[req.Index]
	return newGetTxResp(0, "ok", tx), nil
}

func newCurrentBookkeepersResp(
	code int32, msg string, keepers []*rpcpb.Bookkeeper,
) *rpcpb.CurrentBookkeepersResp {
	return &rpcpb.CurrentBookkeepersResp{
		Code:        code,
		Message:     msg,
		Bookkeepers: keepers,
	}
}

// Delegate is a bookkeeper node.
type Delegate struct {
	Addr            types.AddressHash
	PeerID          string
	Votes           *big.Int
	PledgeAmount    *big.Int
	Score           *big.Int
	ContinualPeriod *big.Int
	IsExist         bool
}

func (s *ctlserver) CurrentBookkeepers(
	ctx context.Context, req *rpcpb.CurrentBookkeepersReq,
) (*rpcpb.CurrentBookkeepersResp, error) {

	output, err := s.server.GetChainReader().CallGenesisContract(0, "getDynasty")
	if err != nil {
		return newCurrentBookkeepersResp(-1, err.Error(), nil), nil
	}
	var dynasties []Delegate
	if err := chain.ContractAbi.Unpack(&dynasties, "getDynasty", output); err != nil {
		return newCurrentBookkeepersResp(-1, err.Error(), nil), nil
	}
	bookkeepers := make([]*rpcpb.Bookkeeper, 0, len(dynasties))
	for _, d := range dynasties {
		addr, err := types.NewAddressPubKeyHash(d.Addr[:])
		if err != nil {
			return nil, fmt.Errorf("get bokkeeper %x error: %s", d.Addr[:], err)
		}
		bk := &rpcpb.Bookkeeper{
			Addr:             addr.String(),
			Votes:            d.Votes.Uint64(),
			PledgeAmount:     d.PledgeAmount.Uint64(),
			Score:            d.Score.Uint64(),
			ContinualPeriods: uint32(d.ContinualPeriod.Uint64()),
		}
		bookkeepers = append(bookkeepers, bk)
	}

	return newCurrentBookkeepersResp(0, "ok", bookkeepers), nil
}
