// // Copyright (c) 2018 ContentBox Authors.
// // Use of this source code is governed by a MIT-style
// // license that can be found in the LICENSE file.

package bpos

import (
	"math"
	"math/big"

	bpospb "github.com/BOXFoundation/boxd/consensus/bpos/pb"
	"github.com/BOXFoundation/boxd/core"
	"github.com/BOXFoundation/boxd/core/chain"
	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/crypto"
	conv "github.com/BOXFoundation/boxd/p2p/convert"
	proto "github.com/gogo/protobuf/proto"
)

// ConsensusContext represents consensus context info.
type ConsensusContext struct {
	timestamp                    int64
	dynasty                      *Dynasty
	verifyDynasty                *Dynasty
	candidates                   []Delegate
	delegates                    []Delegate
	dynastySwitchThreshold       *big.Int
	calcScoreThreshold           *big.Int
	verifyDynastySwitchThreshold *big.Int
	bookKeeperReward             *big.Int
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

var _ conv.Convertible = (*Delegate)(nil)
var _ conv.Serializable = (*Delegate)(nil)

// ToProtoMessage converts Delegate to proto message.
func (delegate *Delegate) ToProtoMessage() (proto.Message, error) {
	return &bpospb.Delegate{
		Addr:         delegate.Addr[:],
		PeerID:       delegate.PeerID,
		Votes:        delegate.Votes.Int64(),
		PledgeAmount: delegate.PledgeAmount.Int64(),
		Score:        delegate.Score.Int64(),
		IsExist:      delegate.IsExist,
	}, nil
}

// FromProtoMessage converts proto message to Delegate.
func (delegate *Delegate) FromProtoMessage(message proto.Message) error {

	if message, ok := message.(*bpospb.Delegate); ok {
		if message != nil {
			copy(delegate.Addr[:], message.Addr)
			delegate.PeerID = message.PeerID
			delegate.Votes = big.NewInt(message.Votes)
			delegate.PledgeAmount = big.NewInt(message.PledgeAmount)
			delegate.Score = big.NewInt(message.Score)
			delegate.IsExist = message.IsExist
			return nil
		}
		return core.ErrEmptyProtoMessage
	}

	return ErrInvalidDelegateProtoMessage
}

// Marshal method marshal Delegate object to binary
func (delegate *Delegate) Marshal() (data []byte, err error) {
	return conv.MarshalConvertible(delegate)
}

// Unmarshal method unmarshal binary data to Delegate object
func (delegate *Delegate) Unmarshal(data []byte) error {
	msg := &bpospb.EternalBlockMsg{}
	if err := proto.Unmarshal(data, msg); err != nil {
		return err
	}
	return delegate.FromProtoMessage(msg)
}

// ///////////////////////////////////////////////////////////////////////////////////
// //////////////////////////////////////////////////////////////////////////////////

// Dynasty is a collection of current bookkeeper nodes.
type Dynasty struct {
	delegates []Delegate
	addrs     []types.AddressHash
	peers     []string
}

var _ conv.Convertible = (*Dynasty)(nil)
var _ conv.Serializable = (*Dynasty)(nil)

// ToProtoMessage converts Dynasty to proto message.
func (dynasty *Dynasty) ToProtoMessage() (proto.Message, error) {
	delegates := make([]*bpospb.Delegate, len(dynasty.delegates))
	for k, v := range dynasty.delegates {
		delegate, err := v.ToProtoMessage()
		if err != nil {
			return nil, err
		}
		if delegate, ok := delegate.(*bpospb.Delegate); ok {
			delegates[k] = delegate
		}
	}
	return &bpospb.Dynasty{
		Delegates: delegates,
	}, nil
}

// FromProtoMessage converts proto message to Dynasty.
func (dynasty *Dynasty) FromProtoMessage(message proto.Message) error {

	if message, ok := message.(*bpospb.Dynasty); ok {
		if message != nil {
			delegates := make([]Delegate, len(message.Delegates))
			for k, v := range message.Delegates {
				delegate := new(Delegate)
				if err := delegate.FromProtoMessage(v); err != nil {
					return err
				}
				delegates[k] = *delegate
			}
			dynasty.delegates = delegates
			return nil
		}
		return core.ErrEmptyProtoMessage
	}

	return ErrInvalidDynastyProtoMessage
}

// Marshal method marshal Dynasty object to binary
func (dynasty *Dynasty) Marshal() (data []byte, err error) {
	return conv.MarshalConvertible(dynasty)
}

// Unmarshal method unmarshal binary data to Dynasty object
func (dynasty *Dynasty) Unmarshal(data []byte) error {
	msg := &bpospb.EternalBlockMsg{}
	if err := proto.Unmarshal(data, msg); err != nil {
		return err
	}
	return dynasty.FromProtoMessage(msg)
}

// FindProposerWithTimeStamp find proposer in given timestamp
func (bpos *Bpos) FindProposerWithTimeStamp(timestamp int64, delegates []Delegate) (*types.AddressHash, error) {

	dynastySize := int64(len(delegates))
	offsetPeriod := (timestamp * SecondInMs) % (BookkeeperRefreshInterval * dynastySize)
	offset := (offsetPeriod / BookkeeperRefreshInterval) % dynastySize

	var bookkeeper *types.AddressHash
	if offset >= 0 && offset < dynastySize {
		bookkeeper = &delegates[offset].Addr
	} else {
		return nil, ErrNotFoundBookkeeper
	}
	return bookkeeper, nil
}

func (bpos *Bpos) fetchDelegatesByHeight(height uint32) ([]Delegate, error) {
	output, err := bpos.chain.Call(height, "getDelegates")
	if err != nil {
		return nil, err
	}
	var delegates []Delegate
	if err := chain.ContractAbi.Unpack(&delegates, "getDelegates", output); err != nil {
		logger.Errorf("Failed to unpack the result of call getCandidates. Err: %v", err)
		return nil, err
	}
	return delegates, nil
}

func (bpos *Bpos) fetchDynastyByHeight(height uint32) (*Dynasty, error) {

	output, err := bpos.chain.Call(height, "getDynasty")
	if err != nil {
		return nil, err
	}
	var dynasty []Delegate
	if err := chain.ContractAbi.Unpack(&dynasty, "getDynasty", output); err != nil {
		logger.Errorf("Failed to unpack the result of call getDynasty. Err: %v", err)
		return nil, err
	}
	PeriodSize := len(dynasty)
	addrs := make([]types.AddressHash, PeriodSize)
	peers := make([]string, PeriodSize)
	for i := 0; i < PeriodSize; i++ {
		addrs[i] = dynasty[i].Addr
		peers[i] = dynasty[i].PeerID
	}
	return &Dynasty{
		delegates: dynasty,
		addrs:     addrs,
		peers:     peers,
	}, nil
}

func (bpos *Bpos) calcScores() ([]*big.Int, error) {
	var totalVote int64
	for _, v := range bpos.context.delegates {
		totalVote += v.Votes.Int64()
	}
	var scores []*big.Int
	for _, v := range bpos.context.delegates {
		score, err := bpos.calcScore(totalVote, v)
		if err != nil {
			return nil, err
		}
		scores = append(scores, score)
	}
	return scores, nil
}

func (bpos *Bpos) calcScore(totalVote int64, delegate Delegate) (*big.Int, error) {
	currentDynasty := (int64(bpos.chain.LongestChainHeight) / bpos.context.dynastySwitchThreshold.Int64()) + 1
	pledgeScore := float64((int64(len(bpos.context.dynasty.delegates)) * delegate.PledgeAmount.Int64() / int64(len(bpos.context.delegates)))) / math.Pow(float64(currentDynasty), 1.5)
	voteScore := delegate.Votes.Int64() * delegate.Votes.Int64() / totalVote
	score := math.Trunc(math.Exp(-0.1*float64(delegate.ContinualPeriod.Int64())) * (pledgeScore + float64(voteScore)))
	return big.NewInt(int64(score)), nil
}

// ///////////////////////////////////////////////////////////////////////////////////
// //////////////////////////////////////////////////////////////////////////////////

// EternalBlockMsg represents eternal block msg.
type EternalBlockMsg struct {
	Hash      crypto.HashType
	Signature []byte
	Timestamp int64
}

var _ conv.Convertible = (*EternalBlockMsg)(nil)
var _ conv.Serializable = (*EternalBlockMsg)(nil)

// ToProtoMessage converts EternalBlockMsg to proto message.
func (ebm *EternalBlockMsg) ToProtoMessage() (proto.Message, error) {
	return &bpospb.EternalBlockMsg{
		Hash:      ebm.Hash[:],
		Timestamp: ebm.Timestamp,
		Signature: ebm.Signature,
	}, nil
}

// FromProtoMessage converts proto message to EternalBlockMsg.
func (ebm *EternalBlockMsg) FromProtoMessage(message proto.Message) error {
	if message, ok := message.(*bpospb.EternalBlockMsg); ok {
		if message != nil {
			copy(ebm.Hash[:], message.Hash)
			ebm.Timestamp = message.Timestamp
			ebm.Signature = message.Signature
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
	msg := &bpospb.EternalBlockMsg{}
	if err := proto.Unmarshal(data, msg); err != nil {
		return err
	}
	return ebm.FromProtoMessage(msg)
}
