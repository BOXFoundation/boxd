// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package blacklist

import (
	"errors"
	"fmt"

	"github.com/BOXFoundation/boxd/core"
	blpb "github.com/BOXFoundation/boxd/core/blacklist/pb"
	corepb "github.com/BOXFoundation/boxd/core/pb"
	coreTypes "github.com/BOXFoundation/boxd/core/types"
	conv "github.com/BOXFoundation/boxd/p2p/convert"
	"github.com/gogo/protobuf/proto"
)

var (
	_ conv.Convertible  = (*Evidence)(nil)
	_ conv.Serializable = (*Evidence)(nil)
	_ conv.Convertible  = (*BlacklistMsg)(nil)
	_ conv.Serializable = (*BlacklistMsg)(nil)

	errInvalidProtoMessage = errors.New("Invalid proto message")
)

// BlacklistMsg contains evidences for bad node
type BlacklistMsg struct {
	evidences []*Evidence
}

func newBlacklistMsg(evis ...*Evidence) *BlacklistMsg {
	if evis == nil {
		evis = make([]*Evidence, 0)
	}
	return &BlacklistMsg{evidences: evis}
}

// ToProtoMessage converts EternalBlockMsg to proto message.
func (evi *Evidence) ToProtoMessage() (proto.Message, error) {

	var err error
	var tx *corepb.Transaction
	var block *corepb.Block

	switch evi.Type {
	case TxEvidence:
		tx, err = ConvTxToPbTx(evi.Tx)
		block = &corepb.Block{}

	case BlockEvidence:
		tx = &corepb.Transaction{}
		block, err = ConvBlockToPbBlock(evi.Block)

	}
	if err != nil {
		return nil, err
	}

	return &blpb.Evidence{
		PubKeyChecksum: evi.PubKeyChecksum,
		Tx:             tx,
		Block:          block,
		Type:           evi.Type,
		Err:            evi.Err,
		Ts:             evi.Ts,
	}, nil
}

// FromProtoMessage converts proto message to EternalBlockMsg.
func (evi *Evidence) FromProtoMessage(message proto.Message) error {
	if evi == nil {
		evi = &Evidence{}
	}
	var err error
	if message, ok := message.(*blpb.Evidence); ok {
		if message != nil {
			evi.PubKeyChecksum = message.PubKeyChecksum
			evi.Tx, err = ConvPbTxToTx(message.Tx)
			if err != nil {
				return err
			}
			evi.Block, err = ConvPbBlockToBlock(message.Block)
			if err != nil {
				return err
			}
			evi.Type = message.Type
			evi.Err = message.Err
			evi.Ts = message.Ts
			return nil
		}
		return core.ErrEmptyProtoMessage
	}

	return errInvalidProtoMessage
}

// Marshal method marshal Candidate object to binary
func (evi *Evidence) Marshal() (data []byte, err error) {
	return conv.MarshalConvertible(evi)
}

// Unmarshal method unmarshal binary data to Candidate object
func (evi *Evidence) Unmarshal(data []byte) error {
	msg := &blpb.Evidence{}
	if err := proto.Unmarshal(data, msg); err != nil {
		return err
	}
	return evi.FromProtoMessage(msg)
}

// ToProtoMessage converts EternalBlockMsg to proto message.
func (blm *BlacklistMsg) ToProtoMessage() (proto.Message, error) {
	if blm == nil {
		blm = newBlacklistMsg()
	}
	evis, err := ConvEvidencesToPbEvidences(blm.evidences)
	if err != nil {
		return nil, err
	}
	return &blpb.BlacklistMsg{
		Evidences: evis,
	}, nil
}

// FromProtoMessage converts proto message to EternalBlockMsg.
func (blm *BlacklistMsg) FromProtoMessage(message proto.Message) error {
	if blm == nil {
		blm = newBlacklistMsg()
	}
	if msg, ok := message.(*blpb.BlacklistMsg); ok {
		if msg != nil {
			var err error
			blm.evidences, err = ConvPbEvidencesToEvidences(msg.Evidences)
			if err != nil {
				logger.Error(err.Error())
				return errInvalidProtoMessage
			}
			return nil
		}
		return core.ErrEmptyProtoMessage
	}
	return errInvalidProtoMessage
}

// Marshal method marshal Candidate object to binary
func (blm *BlacklistMsg) Marshal() (data []byte, err error) {
	return conv.MarshalConvertible(blm)
}

// Unmarshal method unmarshal binary data to Candidate object
func (blm *BlacklistMsg) Unmarshal(data []byte) error {
	msg := &blpb.BlacklistMsg{}
	if err := proto.Unmarshal(data, msg); err != nil {
		return err
	}
	return blm.FromProtoMessage(msg)
}

// ConvBlockToPbBlock convert *coreTypes.Block to *corepb.Block
func ConvBlockToPbBlock(block *coreTypes.Block) (*corepb.Block, error) {
	msg, err := block.ToProtoMessage()
	if err != nil {
		return nil, err
	}
	if hmsg, ok := msg.(*corepb.Block); ok {
		return hmsg, nil
	}
	return nil, fmt.Errorf("asserted failed for proto.Message to *corepb.Block")
}

// ConvPbBlockToBlock convert *corepb.Block to *coreTypes.Block
func ConvPbBlockToBlock(pbBlock *corepb.Block) (*coreTypes.Block, error) {
	block := new(coreTypes.Block)
	if err := block.FromProtoMessage(pbBlock); err != nil {
		return nil, err
	}
	return block, nil
}

// ConvTxToPbTx convert *coreTypes.Transaction to *corepb.Transaction
func ConvTxToPbTx(v *coreTypes.Transaction) (*corepb.Transaction, error) {
	tx, err := v.ToProtoMessage()
	if err != nil {
		return nil, err
	}
	if tx, ok := tx.(*corepb.Transaction); ok {
		return tx, nil
	}
	return nil, fmt.Errorf("asserted failed for proto.Message to *corepb.Transaction")
}

// ConvPbTxToTx convert *corepb.Transaction to *coreTypes.Transaction
func ConvPbTxToTx(pbTx *corepb.Transaction) (*coreTypes.Transaction, error) {
	tx := new(coreTypes.Transaction)
	if err := tx.FromProtoMessage(pbTx); err != nil {
		return nil, err
	}
	return tx, nil
}

// ConvEvidencesToPbEvidences convert []*coreTypes.Block to []*corepb.Block
func ConvEvidencesToPbEvidences(evidences []*Evidence) ([]*blpb.Evidence, error) {
	pbEvis := make([]*blpb.Evidence, 0, len(evidences))
	for _, v := range evidences {
		msg, err := v.ToProtoMessage()
		if err != nil {
			return nil, err
		}
		evi, ok := msg.(*blpb.Evidence)
		if !ok {
			return nil, fmt.Errorf("asserted failed for proto.Message to *blpb.Evidence")
		}
		pbEvis = append(pbEvis, evi)
	}
	return pbEvis, nil
}

// ConvPbEvidencesToEvidences convert []*corepb.Block to []*coreTypes.Block
func ConvPbEvidencesToEvidences(pbEvis []*blpb.Evidence) ([]*Evidence, error) {
	evidences := make([]*Evidence, 0, len(pbEvis))
	for _, v := range pbEvis {
		evidence := new(Evidence)
		if err := evidence.FromProtoMessage(v); err != nil {
			return nil, err
		}
		evidences = append(evidences, evidence)
	}
	return evidences, nil
}
