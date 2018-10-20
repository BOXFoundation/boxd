// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package dpos

import (
	"github.com/BOXFoundation/boxd/consensus/dpos/pb"
	"github.com/BOXFoundation/boxd/core"
	"github.com/BOXFoundation/boxd/core/chain"
	"github.com/BOXFoundation/boxd/core/types"
	conv "github.com/BOXFoundation/boxd/p2p/convert"
	proto "github.com/gogo/protobuf/proto"
	peer "github.com/libp2p/go-libp2p-peer"
)

// ConsensusContext represent consensus context info.
type ConsensusContext struct {
	chain      *chain.BlockChain
	height     int32
	period     []types.AddressHash
	nextPeriod []types.AddressHash
	candidates []*Candidate
}

// NewContext new consensus context.
func NewContext(chain *chain.BlockChain) *ConsensusContext {
	tail := chain.TailBlock()
	return &ConsensusContext{
		chain:  chain,
		height: tail.Height,
	}
}

var _ conv.Convertible = (*ConsensusContext)(nil)
var _ conv.Serializable = (*ConsensusContext)(nil)

// ToProtoMessage converts ConsensusContext to proto message.
func (context *ConsensusContext) ToProtoMessage() (proto.Message, error) {

	var period [][]byte
	for _, v := range context.period {
		period = append(period, v[:])
	}

	var nextPeriod [][]byte
	for _, v := range context.nextPeriod {
		nextPeriod = append(nextPeriod, v[:])
	}

	return &dpospb.ConsensusContext{
		Period:     period,
		NextPeriod: nextPeriod,
	}, nil
}

// FromProtoMessage converts proto message to ConsensusContext.
func (context *ConsensusContext) FromProtoMessage(message proto.Message) error {
	if message, ok := message.(*dpospb.ConsensusContext); ok {
		if message != nil {
			var period []types.AddressHash
			for _, v := range message.Period {
				var temp types.AddressHash
				copy(temp[:], v)
				period = append(period, temp)
			}

			var nextPeriod []types.AddressHash
			for _, v := range message.Period {
				var temp types.AddressHash
				copy(temp[:], v)
				nextPeriod = append(nextPeriod, temp)
			}

			context.period = period
			context.nextPeriod = nextPeriod
			return nil
		}
		return core.ErrEmptyProtoMessage
	}

	return core.ErrInvalidBlockHeaderProtoMessage
}

// Marshal method marshal ConsensusContext object to binary
func (context *ConsensusContext) Marshal() (data []byte, err error) {
	return conv.MarshalConvertible(context)
}

// Unmarshal method unmarshal binary data to ConsensusContext object
func (context *ConsensusContext) Unmarshal(data []byte) error {
	msg := &dpospb.Candidate{}
	if err := proto.Unmarshal(data, msg); err != nil {
		return err
	}
	return context.FromProtoMessage(msg)
}

// FindMinerWithTimeStamp find miner in given timestamp
func (context *ConsensusContext) FindMinerWithTimeStamp(timestamp int64) (*types.AddressHash, error) {

	period := context.period
	offsetPeriod := timestamp % (NewBlockTimeInterval * PeriodSize)
	if (offsetPeriod % NewBlockTimeInterval) != 0 {
		return nil, ErrWrongTimeToMint
	}
	offset := offsetPeriod / NewBlockTimeInterval
	offset = offset % PeriodSize

	var miner *types.AddressHash
	if offset >= 0 && int(offset) < len(period) {
		miner = &period[offset]
	} else {
		return nil, ErrNotFoundMiner
	}
	return miner, nil
}

// Candidates represent possible to be the miner.
type Candidates struct {
	height     int64
	candidates []*Candidate
}

// Candidate represent possible to be the miner.
type Candidate struct {
	addr  types.AddressHash
	votes int32
	peer  peer.ID
}

var _ conv.Convertible = (*Candidate)(nil)
var _ conv.Serializable = (*Candidate)(nil)

// ToProtoMessage converts block header to proto message.
func (candidate *Candidate) ToProtoMessage() (proto.Message, error) {
	return &dpospb.Candidate{
		Addr:  candidate.addr[:],
		Votes: candidate.votes,
		Peer:  candidate.peer.Pretty(),
	}, nil
}

// FromProtoMessage converts proto message to candidate.
func (candidate *Candidate) FromProtoMessage(message proto.Message) error {
	if message, ok := message.(*dpospb.Candidate); ok {
		if message != nil {
			copy(candidate.addr[:], message.Addr)
			candidate.votes = message.Votes
			return nil
		}
		return core.ErrEmptyProtoMessage
	}

	return core.ErrInvalidBlockHeaderProtoMessage
}

// Marshal method marshal Candidate object to binary
func (candidate *Candidate) Marshal() (data []byte, err error) {
	return conv.MarshalConvertible(candidate)
}

// Unmarshal method unmarshal binary data to Candidate object
func (candidate *Candidate) Unmarshal(data []byte) error {
	msg := &dpospb.Candidate{}
	if err := proto.Unmarshal(data, msg); err != nil {
		return err
	}
	return candidate.FromProtoMessage(msg)
}
