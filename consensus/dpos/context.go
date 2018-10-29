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
	ErrInvalidConsensusContextProtoMessage = errors.New("Invalid consensus context proto message")
	ErrInvalidCandidateContextProtoMessage = errors.New("Invalid condidate context proto message")
	ErrInvalidPeriodContextProtoMessage    = errors.New("Invalid period contex proto message")
	ErrInvalidPeriodProtoMessage           = errors.New("Invalid period proto message")
	ErrInvalidEternalBlockMsgProtoMessage  = errors.New("Invalid eternalBlockMsg proto message")
)

// ConsensusContext represent consensus context info.
type ConsensusContext struct {
	timestamp        int64
	periodContext    *PeriodContext
	candidateContext *CandidateContext
}

// PeriodContext represent period context info.
type PeriodContext struct {
	period      []*Period
	nextPeriod  []*Period
	periodAddrs []types.AddressHash
	periodPeers []string
}

// InitPeriodContext init period context.
func InitPeriodContext() (*PeriodContext, error) {

	periods := make([]*Period, len(chain.GenesisPeriod))
	periodAddrs := make([]types.AddressHash, len(chain.GenesisPeriod))
	periodPeers := make([]string, len(chain.GenesisPeriod))
	for k, v := range chain.GenesisPeriod {
		period := new(Period)
		addr, err := types.ParseAddress(v["addr"])
		if err != nil {
			return nil, err
		}
		period.addr = *addr.Hash160()
		period.peerID = v["peerID"]
		periods[k] = period
		periodAddrs[k] = period.addr
		periodPeers[k] = period.peerID
	}
	return &PeriodContext{
		period:      periods,
		periodAddrs: periodAddrs,
		periodPeers: periodPeers,
	}, nil
}

var _ conv.Convertible = (*PeriodContext)(nil)
var _ conv.Serializable = (*PeriodContext)(nil)

// ToProtoMessage converts PeriodContext to proto message.
func (pc *PeriodContext) ToProtoMessage() (proto.Message, error) {

	periods := make([]*dpospb.Period, len(pc.period))
	for k, v := range pc.period {
		period, err := v.ToProtoMessage()
		if err != nil {
			return nil, err
		}
		if period, ok := period.(*dpospb.Period); ok {
			periods[k] = period
		}
	}

	nextPeriods := make([]*dpospb.Period, len(pc.nextPeriod))
	for k, v := range pc.nextPeriod {
		period, err := v.ToProtoMessage()
		if err != nil {
			return nil, err
		}
		if period, ok := period.(*dpospb.Period); ok {
			nextPeriods[k] = period
		}
	}

	return &dpospb.PeriodContext{
		Period:     periods,
		NextPeriod: nextPeriods,
	}, nil
}

// FromProtoMessage converts proto message to PeriodContext.
func (pc *PeriodContext) FromProtoMessage(message proto.Message) error {
	if message, ok := message.(*dpospb.PeriodContext); ok {
		if message != nil {
			periods := make([]*Period, len(message.Period))
			periodAddrs := make([]types.AddressHash, len(message.Period))
			for k, v := range message.Period {
				period := new(Period)
				if err := period.FromProtoMessage(v); err != nil {
					return err
				}
				periods[k] = period
				periodAddrs[k] = period.addr
			}

			nextPeriods := make([]*Period, len(message.NextPeriod))
			for k, v := range message.NextPeriod {
				period := new(Period)
				if err := period.FromProtoMessage(v); err != nil {
					return err
				}
				nextPeriods[k] = period
			}

			pc.period = periods
			pc.nextPeriod = nextPeriods
			pc.periodAddrs = periodAddrs
			return nil
		}
		return core.ErrEmptyProtoMessage
	}

	return ErrInvalidPeriodContextProtoMessage
}

// Marshal method marshal ConsensusContext object to binary
func (pc *PeriodContext) Marshal() (data []byte, err error) {
	return conv.MarshalConvertible(pc)
}

// Unmarshal method unmarshal binary data to ConsensusContext object
func (pc *PeriodContext) Unmarshal(data []byte) error {
	msg := &dpospb.Candidate{}
	if err := proto.Unmarshal(data, msg); err != nil {
		return err
	}
	return pc.FromProtoMessage(msg)
}

// FindMinerWithTimeStamp find miner in given timestamp
func (pc *PeriodContext) FindMinerWithTimeStamp(timestamp int64) (*types.AddressHash, error) {

	period := pc.period
	offsetPeriod := (timestamp * SecondInMs) % (NewBlockTimeInterval * PeriodSize)
	if (offsetPeriod % NewBlockTimeInterval) != 0 {
		return nil, ErrWrongTimeToMint
	}
	offset := offsetPeriod / NewBlockTimeInterval
	offset = offset % PeriodSize

	var miner *types.AddressHash
	if offset >= 0 && int(offset) < len(period) {
		miner = &period[offset].addr
	} else {
		return nil, ErrNotFoundMiner
	}
	return miner, nil
}

// Period represent period info.
type Period struct {
	addr   types.AddressHash
	peerID string
}

var _ conv.Convertible = (*Period)(nil)
var _ conv.Serializable = (*Period)(nil)

// ToProtoMessage converts candidate to proto message.
func (period *Period) ToProtoMessage() (proto.Message, error) {
	return &dpospb.Period{
		Addr:   period.addr[:],
		PeerId: period.peerID,
	}, nil
}

// FromProtoMessage converts proto message to candidate.
func (period *Period) FromProtoMessage(message proto.Message) error {
	if message, ok := message.(*dpospb.Period); ok {
		if message != nil {
			copy(period.addr[:], message.Addr)
			period.peerID = message.PeerId
			return nil
		}
		return core.ErrEmptyProtoMessage
	}

	return ErrInvalidPeriodProtoMessage
}

// Marshal method marshal Period object to binary
func (period *Period) Marshal() (data []byte, err error) {
	return conv.MarshalConvertible(period)
}

// Unmarshal method unmarshal binary data to Period object
func (period *Period) Unmarshal(data []byte) error {
	msg := &dpospb.CandidateContext{}
	if err := proto.Unmarshal(data, msg); err != nil {
		return err
	}
	return period.FromProtoMessage(msg)
}

///////////////////////////////////////////////////////////////////////////////////
//////////////////////////////////////////////////////////////////////////////////

// CandidateContext represent possible to be the miner.
type CandidateContext struct {
	height     uint32
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

///////////////////////////////////////////////////////////////////////////////////
//////////////////////////////////////////////////////////////////////////////////

// EternalBlockMsg represent eternal block msg.
type EternalBlockMsg struct {
	hash      crypto.HashType
	signature []byte
	timestamp int64
}

var _ conv.Convertible = (*EternalBlockMsg)(nil)
var _ conv.Serializable = (*EternalBlockMsg)(nil)

// ToProtoMessage converts EternalBlockMsg to proto message.
func (ebm *EternalBlockMsg) ToProtoMessage() (proto.Message, error) {
	return &dpospb.EternalBlockMsg{
		Hash:      ebm.hash[:],
		Timestamp: ebm.timestamp,
		Signature: ebm.signature,
	}, nil
}

// FromProtoMessage converts proto message to EternalBlockMsg.
func (ebm *EternalBlockMsg) FromProtoMessage(message proto.Message) error {
	if message, ok := message.(*dpospb.EternalBlockMsg); ok {
		if message != nil {
			copy(ebm.hash[:], message.Hash)
			ebm.timestamp = message.Timestamp
			ebm.signature = message.Signature
			return nil
		}
		return core.ErrEmptyProtoMessage
	}

	return ErrInvalidEternalBlockMsgProtoMessage
}

// Marshal method marshal Candidate object to binary
func (ebm *EternalBlockMsg) Marshal() (data []byte, err error) {
	return conv.MarshalConvertible(ebm)
}

// Unmarshal method unmarshal binary data to Candidate object
func (ebm *EternalBlockMsg) Unmarshal(data []byte) error {
	msg := &dpospb.EternalBlockMsg{}
	if err := proto.Unmarshal(data, msg); err != nil {
		return err
	}
	return ebm.FromProtoMessage(msg)
}
