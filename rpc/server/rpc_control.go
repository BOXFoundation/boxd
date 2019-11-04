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

type adminControl struct {
	server    GRPCServer
	whiteList []string
	TableReader
}

type ctlserver struct {
	server GRPCServer
}

func (ac *adminControl) GetNodeInfo(
	ctx context.Context, req *rpcpb.GetNodeInfoRequest,
) (*rpcpb.GetNodeInfoResponse, error) {
	if !isInIPs(ctx, ac.whiteList) {
		return &rpcpb.GetNodeInfoResponse{Code: -1, Message: "allowed only users in white list!"}, nil
	}
	bus := ac.server.GetEventBus()
	ch := make(chan []pstore.NodeInfo)
	bus.Send(eventbus.TopicGetAddressBook, ch)
	defer close(ch)
	nodes := <-ch
	resp := &rpcpb.GetNodeInfoResponse{
		Code:    0,
		Message: "ok",
	}
	for _, n := range nodes {
		resp.Nodes = append(resp.Nodes, &rpcpb.Node{
			Id:    n.PeerID.Pretty(),
			Addrs: n.Addr,
			Ttl:   n.TTL.String(),
		})
	}
	return resp, nil
}

func newBaseResp(code int, msg string) *rpcpb.BaseResponse {
	return &rpcpb.BaseResponse{Code: int32(code), Message: msg}
}

func (ac *adminControl) AddNode(
	ctx context.Context, req *rpcpb.AddNodeRequest,
) (*rpcpb.BaseResponse, error) {
	if !isInIPs(ctx, ac.whiteList) {
		return newBaseResp(-1, ErrIPNotAllowed.Error()), nil
	}
	out := make(chan error)
	ac.server.GetEventBus().Send(eventbus.TopicP2PAddPeer, req.Node, out)
	if err := <-out; err != nil {
		return newBaseResp(-1, err.Error()), nil
	}
	return newBaseResp(0, "ok"), nil
}

// SetDebugLevel implements SetDebugLevel
func (ac *adminControl) SetDebugLevel(
	ctx context.Context, in *rpcpb.DebugLevelRequest,
) (*rpcpb.BaseResponse, error) {
	if !isInIPs(ctx, ac.whiteList) {
		return newBaseResp(-1, ErrIPNotAllowed.Error()), nil
	}
	bus := ac.server.GetEventBus()
	ch := make(chan bool)
	bus.Send(eventbus.TopicSetDebugLevel, in.Level, ch)
	if <-ch {
		return newBaseResp(0, "ok"), nil
	}
	return newBaseResp(-1, "wrong debug level"), nil
}

// GetNetworkID returns
func (ac *adminControl) GetNetworkID(
	ctx context.Context, req *rpcpb.GetNetworkIDRequest,
) (*rpcpb.GetNetworkIDResponse, error) {
	if !isInIPs(ctx, ac.whiteList) {
		return &rpcpb.GetNetworkIDResponse{Code: -1, Message: ErrIPNotAllowed.Error()}, nil
	}
	ch := make(chan uint32)
	ac.server.GetEventBus().Send(eventbus.TopicGetNetworkID, ch)
	current := <-ch
	var literal string
	if current == p2p.Mainnet {
		literal = "Mainnet"
	} else if current == p2p.Testnet {
		literal = "Testnet"
	} else {
		literal = "Unknown"
	}
	return &rpcpb.GetNetworkIDResponse{Code: 0, Message: "ok", Id: current, Literal: literal}, nil
}

// UpdateNetworkID implements UpdateNetworkID
// NOTE: should be remove in product env
func (ac *adminControl) UpdateNetworkID(
	ctx context.Context, in *rpcpb.UpdateNetworkIDRequest,
) (*rpcpb.BaseResponse, error) {
	if !isInIPs(ctx, ac.whiteList) {
		return newBaseResp(-1, ErrIPNotAllowed.Error()), nil
	}
	bus := ac.server.GetEventBus()
	ch := make(chan bool)
	bus.Send(eventbus.TopicUpdateNetworkID, in.Id, ch)
	if <-ch {
		return newBaseResp(0, "ok"), nil
	}
	return newBaseResp(-1, fmt.Sprintf("Wrong NetworkID: %d", in.Id)), nil
}

func (ac *adminControl) PeerID(
	ctx context.Context, req *rpcpb.PeerIDReq,
) (*rpcpb.PeerIDResp, error) {
	if !isInIPs(ctx, ac.whiteList) {
		return &rpcpb.PeerIDResp{Code: -1, Message: ErrIPNotAllowed.Error()}, nil
	}
	return &rpcpb.PeerIDResp{
		Code:    0,
		Message: "",
		Peerid:  ac.TableReader.PeerID(),
	}, nil
}

func (ac *adminControl) Miners(
	ctx context.Context, req *rpcpb.MinersReq,
) (*rpcpb.MinersResp, error) {

	if !isInIPs(ctx, ac.whiteList) {
		return &rpcpb.MinersResp{Code: -1, Message: ErrIPNotAllowed.Error()}, nil
	}

	infos := ac.TableReader.Miners()
	miners := make([]*rpcpb.MinerDetail, len(infos))

	for i, info := range infos {
		miners[i] = &rpcpb.MinerDetail{
			Id:      info.ID,
			Address: info.Addr,
			Iplist:  info.Iplist,
		}
	}

	return &rpcpb.MinersResp{
		Code:    0,
		Message: "",
		Miners:  miners,
	}, nil
}

func (s *ctlserver) GetCurrentBlockHeight(
	ctx context.Context, req *rpcpb.GetCurrentBlockHeightRequest,
) (*rpcpb.GetCurrentBlockHeightResponse, error) {
	height := s.server.GetChainReader().TailBlock().Header.Height
	return &rpcpb.GetCurrentBlockHeightResponse{Code: 0, Message: "ok", Height: height}, nil
}

func (s *ctlserver) LatestConfirmedHeight(
	ctx context.Context, req *rpcpb.LatestConfirmedHeightReq,
) (*rpcpb.LatestConfirmedHeightResp, error) {
	height := s.server.GetChainReader().EternalBlock().Header.Height
	return &rpcpb.LatestConfirmedHeightResp{Code: 0, Message: "ok", Height: height}, nil
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

func newGetTxCountResp(code int32, message string, count uint32) *rpcpb.GetTxCountResp {
	return &rpcpb.GetTxCountResp{
		Code:    code,
		Message: message,
		Count:   count,
	}
}

func (s *ctlserver) GetTxCount(ctx context.Context,
	req *rpcpb.GetTxCountReq) (*rpcpb.GetTxCountResp, error) {
	hash := new(crypto.HashType)
	var err error
	if len(req.BlockHash) == 0 {
		hash, err = s.server.GetChainReader().GetBlockHash(req.BlockHeight)
		if err != nil {
			return newGetTxCountResp(-1, err.Error(), 0), nil
		}
	} else {
		if err := hash.SetString(req.BlockHash); err != nil {
			return newGetTxCountResp(-1, fmt.Sprintf("Invalid hash: %s", req.BlockHash), 0), nil
		}
	}
	block, _, err := s.server.GetChainReader().ReadBlockFromDB(hash)
	if err != nil {
		return newGetTxCountResp(-1, fmt.Sprintf("Error searching block: %s", req.BlockHash), 0), nil
	}
	msg, err := block.ToProtoMessage()
	if err != nil {
		return newGetTxCountResp(-1, err.Error(), 0), nil
	}
	blockPb, ok := msg.(*corepb.Block)

	if !ok {
		return newGetTxCountResp(-1, "can't convert proto message", 0), nil
	}
	count := len(blockPb.Txs)
	return newGetTxCountResp(0, "ok", uint32(count)), nil
}

// Delegate is an account to run for bookkeepers.
type Delegate struct {
	Addr            types.AddressHash
	PeerID          string
	Votes           *big.Int
	PledgeAmount    *big.Int
	Score           *big.Int
	ContinualPeriod *big.Int
	IsExist         bool
}

func (s *ctlserver) getDelegates(methodName string) ([]*rpcpb.Delegate, error) {
	output, err := s.server.GetChainReader().CallGenesisContract(0, methodName)
	if err != nil {
		return nil, err
	}
	var delegates []Delegate
	if err := chain.ContractAbi.Unpack(&delegates, methodName, output); err != nil {
		return nil, err
	}
	delegatesR := make([]*rpcpb.Delegate, 0, len(delegates))
	for _, d := range delegates {
		addr, err := types.NewAddressPubKeyHash(d.Addr[:])
		if err != nil {
			return nil, err
		}
		dl := &rpcpb.Delegate{
			Addr:             addr.String(),
			Votes:            d.Votes.Uint64(),
			PledgeAmount:     d.PledgeAmount.Uint64(),
			Score:            d.Score.Uint64(),
			ContinualPeriods: uint32(d.ContinualPeriod.Uint64()),
		}
		delegatesR = append(delegatesR, dl)
	}
	return delegatesR, nil
}

func newDelegateResp(
	code int32, msg string, delegates []*rpcpb.Delegate,
) *rpcpb.DelegatesResp {
	return &rpcpb.DelegatesResp{
		Code:      code,
		Message:   msg,
		Delegates: delegates,
	}
}

func (s *ctlserver) Delegates(
	ctx context.Context, req *rpcpb.DelegatesReq,
) (*rpcpb.DelegatesResp, error) {

	method := "getDynasty"
	switch req.Type {
	case rpcpb.DelegatesReq_BOOKKEEPERS:
		method = "getDynasty"
	case rpcpb.DelegatesReq_DELEGATES:
		method = "getCurrentDelegates"
	case rpcpb.DelegatesReq_CANDIDATES:
		method = "getNextDelegates"
	default:
		return newDelegateResp(-1, "invalid type", nil), nil
	}
	delegates, err := s.getDelegates(method)
	if err != nil {
		return newDelegateResp(-1, err.Error(), nil), nil
	}
	return newDelegateResp(0, "ok", delegates), nil
}
