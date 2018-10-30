// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package types

import (
	"github.com/BOXFoundation/boxd/core"
	corepb "github.com/BOXFoundation/boxd/core/pb"
	conv "github.com/BOXFoundation/boxd/p2p/convert"
	proto "github.com/gogo/protobuf/proto"
)

// UtxoWrap contains info about utxo
type UtxoWrap struct {
	Output      *corepb.TxOut
	BlockHeight uint32
	IsCoinBase  bool
	IsSpent     bool
	IsModified  bool
}

// ToProtoMessage converts utxo wrap to proto message.
func (utxoWrap *UtxoWrap) ToProtoMessage() (proto.Message, error) {
	return &corepb.UtxoWrap{
		Output:      utxoWrap.Output,
		BlockHeight: utxoWrap.BlockHeight,
		IsCoinbase:  utxoWrap.IsCoinBase,
		IsSpent:     utxoWrap.IsSpent,
	}, nil
}

// FromProtoMessage converts proto message to utxo wrap.
func (utxoWrap *UtxoWrap) FromProtoMessage(message proto.Message) error {
	if message, ok := message.(*corepb.UtxoWrap); ok {
		utxoWrap.Output = message.Output
		utxoWrap.BlockHeight = message.BlockHeight
		utxoWrap.IsCoinBase = message.IsCoinbase
		utxoWrap.IsSpent = message.IsSpent
		utxoWrap.IsModified = false
		return nil
	}
	return core.ErrInvalidUtxoWrapProtoMessage
}

// Marshal method marshal UtxoWrap object to binary
func (utxoWrap *UtxoWrap) Marshal() (data []byte, err error) {
	return conv.MarshalConvertible(utxoWrap)
}

// Unmarshal method unmarshal binary data to UtxoWrap object
func (utxoWrap *UtxoWrap) Unmarshal(data []byte) error {
	msg := &corepb.UtxoWrap{}
	if err := proto.Unmarshal(data, msg); err != nil {
		return err
	}
	return utxoWrap.FromProtoMessage(msg)
}

// Value returns utxo amount
func (utxoWrap *UtxoWrap) Value() uint64 {
	return utxoWrap.Output.Value
}
