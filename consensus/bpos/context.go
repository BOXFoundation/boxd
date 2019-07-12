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

// Delegate is a bookkeeper node.
type Delegate struct {
	Addr         types.AddressHash
	PeerID       string
	Votes        *big.Int
	PledgeAmount *big.Int
	Score        *big.Int
	IsExist      bool
}

// Dynasty is a collection of current bookkeeper nodes.
type Dynasty struct {
	delegates []Delegate
	addrs     []types.AddressHash
	peers     []string
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

func (bpos *Bpos) fetchDynastyByHeight(height uint32) (*Dynasty, error) {

	abiObj, err := chain.ReadAbi(bpos.chain.Cfg().ContractABIPath)
	if err != nil {
		return nil, err
	}
	data, err := abiObj.Pack("getDynasty")
	if err != nil {
		return nil, err
	}
	msg := types.NewVMTransaction(new(big.Int), big.NewInt(1), math.MaxUint64/2,
		0, nil, types.ContractCallType, data).WithFrom(bpos.bookkeeper.Address.Hash160()).WithTo(&chain.ContractAddr)
	evm, vmErr, err := bpos.chain.NewEvmContextForLocalCallByHeight(msg, height)

	output, _, _, _, _, err := chain.ApplyMessage(evm, msg)
	if err := vmErr(); err != nil {
		return nil, err
	}
	var dynasty []Delegate
	if err := abiObj.Unpack(&dynasty, "getDynasty", output); err != nil {
		logger.Errorf("Failed to unpack the result of call getDynasty")
		return nil, err
	}
	logger.Infof("get dynasty from contract: %v", dynasty)
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
