// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package controller

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/BOXFoundation/boxd/core"
	ctlpb "github.com/BOXFoundation/boxd/core/controller/pb"
	corepb "github.com/BOXFoundation/boxd/core/pb"
	coreTypes "github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/crypto"
	conv "github.com/BOXFoundation/boxd/p2p/convert"
	"github.com/gogo/protobuf/proto"
)

var (
	_ conv.Convertible  = (*Evidence)(nil)
	_ conv.Serializable = (*Evidence)(nil)
	_ conv.Convertible  = (*BlacklistMsg)(nil)
	_ conv.Serializable = (*BlacklistMsg)(nil)
	_ conv.Convertible  = (*BlacklistConfirmMsg)(nil)
	_ conv.Serializable = (*BlacklistConfirmMsg)(nil)
	_ conv.Convertible  = (*BlacklistTxData)(nil)
	_ conv.Serializable = (*BlacklistTxData)(nil)

	errInvalidProtoMessage = errors.New("Invalid proto message")
)

// BlacklistMsg contains evidences for bad node
type BlacklistMsg struct {
	hash      []byte
	evidences []*Evidence
}

// BlacklistConfirmMsg contains evidences for bad node
type BlacklistConfirmMsg struct {
	pubkey    []byte
	hash      []byte
	signature []byte
	timestamp int64
}

// BlacklistTxData will put into tx on chain
type BlacklistTxData struct {
	pubkey     []byte
	hash       []byte
	signatures [][]byte
}

func newBlacklistMsg(evis ...*Evidence) *BlacklistMsg {
	if evis == nil {
		evis = make([]*Evidence, 0)
	}
	blm := &BlacklistMsg{evidences: evis}
	hash, _ := blm.calcHash()
	blm.hash = hash
	return blm
}

// calcHash calculates the identifier hash for the msg.
func (blm *BlacklistMsg) calcHash() ([]byte, error) {
	if blm.evidences == nil || len(blm.evidences) == 0 {
		return nil, core.ErrEmptyEvidence
	}

	msgBuf := make([][]byte, len(blm.evidences))

	for _, evidence := range blm.evidences {
		eviBuf, err := evidence.Marshal()
		if err != nil {
			return nil, err
		}
		msgBuf = append(msgBuf, eviBuf)
	}
	hash := crypto.Sha256Multi(msgBuf...)
	return hash, nil
}

func (blm *BlacklistMsg) validateHash() (bool, error) {
	eviHash, err := blm.calcHash()
	if err != nil {
		return false, err
	}
	return bytes.Equal(blm.hash, eviHash), nil
}

// Hash return hash
func (btd *BlacklistTxData) Hash() []byte {
	return btd.hash
}

// Signatures return signatures
func (btd *BlacklistTxData) Signatures() [][]byte {
	return btd.signatures
}

////////////////////////////////////////////////////////////////////

// ToProtoMessage converts Evidence to proto message.
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

	return &ctlpb.Evidence{
		PubKey: evi.PubKey,
		Tx:     tx,
		Block:  block,
		Type:   evi.Type,
		Err:    evi.Err,
		Ts:     evi.Ts,
	}, nil
}

// FromProtoMessage converts proto message to Evidence.
func (evi *Evidence) FromProtoMessage(message proto.Message) error {
	if evi == nil {
		evi = &Evidence{}
	}
	var err error
	if message, ok := message.(*ctlpb.Evidence); ok {
		if message != nil {
			evi.PubKey = make([]byte, len(message.PubKey))
			copy(evi.PubKey[:], message.PubKey[:])
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

// Marshal method marshal Evidence object to binary
func (evi *Evidence) Marshal() (data []byte, err error) {
	return conv.MarshalConvertible(evi)
}

// Unmarshal method unmarshal binary data to Evidence object
func (evi *Evidence) Unmarshal(data []byte) error {
	msg := &ctlpb.Evidence{}
	if err := proto.Unmarshal(data, msg); err != nil {
		return err
	}
	return evi.FromProtoMessage(msg)
}

// ToProtoMessage converts BlacklistMsg to proto message.
func (blm *BlacklistMsg) ToProtoMessage() (proto.Message, error) {
	if blm == nil {
		blm = newBlacklistMsg()
	}
	evis, err := ConvEvidencesToPbEvidences(blm.evidences)
	if err != nil {
		return nil, err
	}

	hash := make([]byte, len(blm.hash))
	copy(hash[:], blm.hash[:])

	return &ctlpb.BlacklistMsg{
		Evidences: evis,
		Hash:      hash,
	}, nil
}

// FromProtoMessage converts proto message to BlacklistMsg.
func (blm *BlacklistMsg) FromProtoMessage(message proto.Message) error {
	if blm == nil {
		blm = newBlacklistMsg()
	}
	if msg, ok := message.(*ctlpb.BlacklistMsg); ok {
		if msg != nil {
			var err error
			blm.evidences, err = ConvPbEvidencesToEvidences(msg.Evidences)
			if err != nil {
				logger.Error(err.Error())
				return errInvalidProtoMessage
			}
			blm.hash = make([]byte, len(msg.Hash))
			copy(blm.hash[:], msg.Hash[:])
			return nil
		}
		return core.ErrEmptyProtoMessage
	}
	return errInvalidProtoMessage
}

// Marshal method marshal BlacklistMsg object to binary
func (blm *BlacklistMsg) Marshal() (data []byte, err error) {
	return conv.MarshalConvertible(blm)
}

// Unmarshal method unmarshal binary data to BlacklistMsg object
func (blm *BlacklistMsg) Unmarshal(data []byte) error {
	msg := &ctlpb.BlacklistMsg{}
	if err := proto.Unmarshal(data, msg); err != nil {
		return err
	}
	return blm.FromProtoMessage(msg)
}

// ToProtoMessage converts BlacklistConfirmMsg to proto message.
func (bcm *BlacklistConfirmMsg) ToProtoMessage() (proto.Message, error) {
	if bcm == nil {
		return nil, core.ErrEmptyProtoSource
	}

	hash := make([]byte, len(bcm.hash))
	copy(hash[:], bcm.hash[:])

	signature := make([]byte, len(bcm.signature))
	copy(signature[:], bcm.signature[:])

	pubkey := make([]byte, len(bcm.pubkey))
	copy(pubkey[:], bcm.pubkey[:])

	return &ctlpb.BlacklistConfirmMsg{
		Pubkey:    pubkey,
		Hash:      hash,
		Signature: signature,
		Timestamp: bcm.timestamp,
	}, nil
}

// FromProtoMessage converts proto message to BlacklistConfirmMsg.
func (bcm *BlacklistConfirmMsg) FromProtoMessage(message proto.Message) error {
	if bcm == nil {
		return core.ErrEmptyProtoMessage
	}
	if msg, ok := message.(*ctlpb.BlacklistConfirmMsg); ok {
		if msg != nil {
			bcm.pubkey = make([]byte, len(msg.Pubkey))
			copy(bcm.pubkey[:], msg.Pubkey[:])
			bcm.hash = make([]byte, len(msg.Hash))
			copy(bcm.hash[:], msg.Hash[:])
			bcm.signature = make([]byte, len(msg.Signature))
			copy(bcm.signature[:], msg.Signature[:])
			bcm.timestamp = msg.Timestamp
			return nil
		}
		return core.ErrEmptyProtoMessage
	}
	return errInvalidProtoMessage
}

// Marshal method marshal BlacklistConfirmMsg object to binary
func (bcm *BlacklistConfirmMsg) Marshal() (data []byte, err error) {
	return conv.MarshalConvertible(bcm)
}

// Unmarshal method unmarshal binary data to BlacklistConfirmMsg object
func (bcm *BlacklistConfirmMsg) Unmarshal(data []byte) error {
	msg := &ctlpb.BlacklistConfirmMsg{}
	if err := proto.Unmarshal(data, msg); err != nil {
		return err
	}
	return bcm.FromProtoMessage(msg)
}

// ToProtoMessage converts BlacklistConfirmMsg to proto message.
func (btd *BlacklistTxData) ToProtoMessage() (proto.Message, error) {
	if btd == nil {
		return nil, core.ErrEmptyProtoSource
	}

	hash := make([]byte, len(btd.hash))
	copy(hash[:], btd.hash[:])
	pubkey := make([]byte, len(btd.pubkey))
	copy(pubkey[:], btd.pubkey[:])

	signs := [][]byte{}
	for _, v := range btd.signatures {
		sign := make([]byte, len(v))
		copy(sign[:], v[:])
		signs = append(signs, sign)
	}

	return &ctlpb.BlacklistTxData{
		Pubkey:     pubkey,
		Hash:       hash,
		Signatures: signs,
	}, nil
}

// FromProtoMessage converts proto message to BlacklistConfirmMsg.
func (btd *BlacklistTxData) FromProtoMessage(message proto.Message) error {
	if btd == nil {
		btd = &BlacklistTxData{}
	}
	if msg, ok := message.(*ctlpb.BlacklistTxData); ok {
		if msg != nil {
			btd.pubkey = make([]byte, len(msg.Pubkey))
			copy(btd.pubkey[:], msg.Pubkey[:])
			btd.hash = make([]byte, len(msg.Hash))
			copy(btd.hash[:], msg.Hash[:])

			btd.signatures = make([][]byte, len(btd.signatures))
			for _, v := range msg.Signatures {
				sign := make([]byte, len(v))
				copy(sign[:], v[:])
				btd.signatures = append(btd.signatures, sign)
			}
			return nil
		}
		return core.ErrEmptyProtoMessage
	}
	return errInvalidProtoMessage
}

// Marshal method marshal BlacklistConfirmMsg object to binary
func (btd *BlacklistTxData) Marshal() (data []byte, err error) {
	return conv.MarshalConvertible(btd)
}

// Unmarshal method unmarshal binary data to BlacklistConfirmMsg object
func (btd *BlacklistTxData) Unmarshal(data []byte) error {
	msg := &ctlpb.BlacklistTxData{}
	if err := proto.Unmarshal(data, msg); err != nil {
		return err
	}
	return btd.FromProtoMessage(msg)
}

///////////////////////////////////////////////////////////////////////

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
func ConvEvidencesToPbEvidences(evidences []*Evidence) ([]*ctlpb.Evidence, error) {
	pbEvis := make([]*ctlpb.Evidence, 0, len(evidences))
	for _, v := range evidences {
		msg, err := v.ToProtoMessage()
		if err != nil {
			return nil, err
		}
		evi, ok := msg.(*ctlpb.Evidence)
		if !ok {
			return nil, fmt.Errorf("asserted failed for proto.Message to *ctlpb.Evidence")
		}
		pbEvis = append(pbEvis, evi)
	}
	return pbEvis, nil
}

// ConvPbEvidencesToEvidences convert []*corepb.Block to []*coreTypes.Block
func ConvPbEvidencesToEvidences(pbEvis []*ctlpb.Evidence) ([]*Evidence, error) {
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
