// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package dpos

import (
	"errors"

	"github.com/BOXFoundation/boxd/consensus/dpos/pb"
	"github.com/BOXFoundation/boxd/core"
	"github.com/BOXFoundation/boxd/core/chain"
	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/crypto"
	conv "github.com/BOXFoundation/boxd/p2p/convert"
	proto "github.com/gogo/protobuf/proto"
	peer "github.com/libp2p/go-libp2p-peer"
)

// Define error
var (
	ErrInvalidCandidateProtoMessage        = errors.New("Invalid candidate proto message")
	ErrInvalidConsensusContextProtoMessage = errors.New("Invalid consensusContext proto message")
	ErrInvalidCandidateContextProtoMessage = errors.New("Invalid condidate Context proto message")
)

// ConsensusContext represent consensus context info.
type ConsensusContext struct {
	timestamp        int64
	periodContext    *PeriodContext
	candidateContext *CandidateContext
}

// PeriodContext represent period context info.
type PeriodContext struct {
	period     []types.AddressHash
	nextPeriod []types.AddressHash
}

// InitPeriodContext init period context.
func InitPeriodContext() (*PeriodContext, error) {

	period := make([]types.AddressHash, PeriodSize)
	for k, v := range chain.GenesisPeriod {
		addr, err := types.ParseAddress(v)
		if err != nil {
			return nil, err
		}
		period[k] = *addr.Hash160()
	}
	return &PeriodContext{
		period: period,
	}, nil
}

var _ conv.Convertible = (*PeriodContext)(nil)
var _ conv.Serializable = (*PeriodContext)(nil)

// ToProtoMessage converts PeriodContext to proto message.
func (context *PeriodContext) ToProtoMessage() (proto.Message, error) {

	var period [][]byte
	for _, v := range context.period {
		period = append(period, v[:])
	}

	var nextPeriod [][]byte
	for _, v := range context.nextPeriod {
		nextPeriod = append(nextPeriod, v[:])
	}

	return &dpospb.PeriodContext{
		Period:     period,
		NextPeriod: nextPeriod,
	}, nil
}

// FromProtoMessage converts proto message to PeriodContext.
func (context *PeriodContext) FromProtoMessage(message proto.Message) error {
	if message, ok := message.(*dpospb.PeriodContext); ok {
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

	return ErrInvalidConsensusContextProtoMessage
}

// Marshal method marshal ConsensusContext object to binary
func (context *PeriodContext) Marshal() (data []byte, err error) {
	return conv.MarshalConvertible(context)
}

// Unmarshal method unmarshal binary data to ConsensusContext object
func (context *PeriodContext) Unmarshal(data []byte) error {
	msg := &dpospb.Candidate{}
	if err := proto.Unmarshal(data, msg); err != nil {
		return err
	}
	return context.FromProtoMessage(msg)
}

// FindMinerWithTimeStamp find miner in given timestamp
func (context *PeriodContext) FindMinerWithTimeStamp(timestamp int64) (*types.AddressHash, error) {

	period := context.period
	offsetPeriod := (timestamp * SecondInMs) % (NewBlockTimeInterval * PeriodSize)
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

///////////////////////////////////////////////////////////////////////////////////
//////////////////////////////////////////////////////////////////////////////////

// CandidateContext represent possible to be the miner.
type CandidateContext struct {
	height     int32
	candidates []*Candidate
	addrs      []types.AddressHash
}

// InitCandidateContext init candidate context
func InitCandidateContext() *CandidateContext {
	return &CandidateContext{
		height:     0,
		candidates: []*Candidate{},
	}
}

var _ conv.Convertible = (*CandidateContext)(nil)
var _ conv.Serializable = (*CandidateContext)(nil)

// ToProtoMessage converts block header to proto message.
func (candidateContext *CandidateContext) ToProtoMessage() (proto.Message, error) {

	candidates := make([]*dpospb.Candidate, len(candidateContext.candidates))
	for k, v := range candidateContext.candidates {
		candidate, err := v.ToProtoMessage()
		if err != nil {
			return nil, err
		}
		if candidate, ok := candidate.(*dpospb.Candidate); ok {
			candidates[k] = candidate
		}
	}

	return &dpospb.CandidateContext{
		Height:     candidateContext.height,
		Candidates: candidates,
	}, nil
}

// FromProtoMessage converts proto message to candidate.
func (candidateContext *CandidateContext) FromProtoMessage(message proto.Message) error {

	if message, ok := message.(*dpospb.CandidateContext); ok {
		if message != nil {
			candidates := make([]*Candidate, len(message.Candidates))
			addrs := make([]types.AddressHash, len(message.Candidates))
			for k, v := range message.Candidates {
				candidate := new(Candidate)
				if err := candidate.FromProtoMessage(v); err != nil {
					return err
				}
				candidates[k] = candidate
				addrs[k] = candidate.addr
			}
			candidateContext.height = message.Height
			candidateContext.candidates = candidates
			candidateContext.addrs = addrs
			return nil
		}
		return core.ErrEmptyProtoMessage
	}

	return ErrInvalidCandidateContextProtoMessage
}

// Marshal method marshal CandidateContext object to binary
func (candidateContext *CandidateContext) Marshal() (data []byte, err error) {
	return conv.MarshalConvertible(candidateContext)
}

// Unmarshal method unmarshal binary data to CandidateContext object
func (candidateContext *CandidateContext) Unmarshal(data []byte) error {
	msg := &dpospb.CandidateContext{}
	if err := proto.Unmarshal(data, msg); err != nil {
		return err
	}
	return candidateContext.FromProtoMessage(msg)
}

// CandidateContextHash calc candidate context hash.
func (candidateContext *CandidateContext) CandidateContextHash() (*crypto.HashType, error) {
	bytes, err := candidateContext.Marshal()
	if err != nil {
		return nil, err
	}
	hash := crypto.DoubleHashH(bytes) // dhash of header
	return &hash, nil
}

///////////////////////////////////////////////////////////////////////////////////
//////////////////////////////////////////////////////////////////////////////////

// Candidate represent possible to be the miner.
type Candidate struct {
	addr  types.AddressHash
	votes int64
	peer  peer.ID
}

var _ conv.Convertible = (*Candidate)(nil)
var _ conv.Serializable = (*Candidate)(nil)

// ToProtoMessage converts candidate to proto message.
func (candidate *Candidate) ToProtoMessage() (proto.Message, error) {
	return &dpospb.Candidate{
		Addr:  candidate.addr[:],
		Votes: candidate.votes,
		// Peer:  candidate.peer.Pretty(),
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

	return ErrInvalidCandidateProtoMessage
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
