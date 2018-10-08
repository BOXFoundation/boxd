// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package types

import (
	"errors"

	corepb "github.com/BOXFoundation/Quicksilver/core/pb"
	"github.com/BOXFoundation/Quicksilver/crypto"
	conv "github.com/BOXFoundation/Quicksilver/p2p/convert"
	proto "github.com/gogo/protobuf/proto"
)

// Define error message
var (
	ErrSerializeOutPoint           = errors.New("serialize outPoint error")
	ErrInvalidOutPointProtoMessage = errors.New("Invalid OutPoint proto message")
	ErrInvalidTxInProtoMessage     = errors.New("Invalid TxIn proto message")
	ErrInvalidTxOutProtoMessage    = errors.New("Invalid TxOut proto message")
	ErrInvalidTxProtoMessage       = errors.New("Invalid tx proto message")
)

// Transaction defines a transaction.
type Transaction struct {
	Hash    *crypto.HashType
	MsgTx   *MsgTx
	TxIndex int // Position within a block or TxIndexUnknown
}

// MsgTx is used to deliver transaction information.
type MsgTx struct {
	Version  int32
	Vin      []*TxIn
	Vout     []*TxOut
	Magic    uint32
	LockTime int64
}

var _ conv.Convertible = (*MsgTx)(nil)

// TxOut defines a transaction output.
type TxOut struct {
	Value        int64
	ScriptPubKey []byte
}

var _ conv.Convertible = (*TxOut)(nil)

// TxIn defines a transaction input.
type TxIn struct {
	PrevOutPoint OutPoint
	ScriptSig    []byte
	Sequence     uint32
}

var _ conv.Convertible = (*TxIn)(nil)

// OutPoint defines a data type that is used to track previous transaction outputs.
type OutPoint struct {
	Hash  crypto.HashType
	Index uint32
}

var _ conv.Convertible = (*OutPoint)(nil)

// NewTx a Transaction wrapped by msgTx.
func NewTx(msgTx *MsgTx) (*Transaction, error) {
	hash, err := msgTx.MsgTxHash()
	if err != nil {
		return nil, err
	}
	return &Transaction{
		Hash:  hash,
		MsgTx: msgTx,
	}, nil
}

// ToProtoMessage converts transaction to proto message.
func (msgTx *MsgTx) ToProtoMessage() (proto.Message, error) {
	var vins []*corepb.TxIn
	var vouts []*corepb.TxOut
	for _, v := range msgTx.Vin {
		vin, err := v.ToProtoMessage()
		if err != nil {
			return nil, err
		}
		if vin, ok := vin.(*corepb.TxIn); ok {
			vins = append(vins, vin)
		}
	}
	for _, v := range msgTx.Vout {
		vout, _ := v.ToProtoMessage()
		if vout, ok := vout.(*corepb.TxOut); ok {
			vouts = append(vouts, vout)
		}
	}
	return &corepb.MsgTx{
		Version:  msgTx.Version,
		Vin:      vins,
		Vout:     vouts,
		Magic:    msgTx.Magic,
		LockTime: msgTx.LockTime,
	}, nil
}

// FromProtoMessage converts proto message to transaction.
func (msgTx *MsgTx) FromProtoMessage(message proto.Message) error {
	if message, ok := message.(*corepb.MsgTx); ok {
		if message != nil {
			var vins []*TxIn
			for _, v := range message.Vin {
				txin := new(TxIn)
				if err := txin.FromProtoMessage(v); err != nil {
					return err
				}
				vins = append(vins, txin)
			}

			var vouts []*TxOut
			for _, v := range message.Vout {
				txout := new(TxOut)
				if err := txout.FromProtoMessage(v); err != nil {
					return err
				}
				vouts = append(vouts, txout)
			}

			msgTx.Version = message.Version
			msgTx.Vin = vins
			msgTx.Vout = vouts
			msgTx.Magic = message.Magic
			msgTx.LockTime = message.LockTime
			return nil
		}
		return ErrEmptyProtoMessage
	}
	return ErrInvalidTxInProtoMessage
}

// ToProtoMessage converts txout to proto message.
func (txout *TxOut) ToProtoMessage() (proto.Message, error) {
	return &corepb.TxOut{
		Value:        txout.Value,
		ScriptPubKey: txout.ScriptPubKey,
	}, nil
}

// FromProtoMessage converts proto message to txout.
func (txout *TxOut) FromProtoMessage(message proto.Message) error {
	if message, ok := message.(*corepb.TxOut); ok {
		if message != nil {
			txout.ScriptPubKey = message.ScriptPubKey
			txout.Value = message.Value
			return nil
		}
		return ErrEmptyProtoMessage
	}

	return ErrInvalidTxOutProtoMessage
}

// ToProtoMessage converts txin to proto message.
func (txin *TxIn) ToProtoMessage() (proto.Message, error) {
	prevOutPoint, _ := txin.PrevOutPoint.ToProtoMessage()
	if prevOutPoint, ok := prevOutPoint.(*corepb.OutPoint); ok {
		return &corepb.TxIn{
			PrevOutPoint: prevOutPoint,
			ScriptSig:    txin.ScriptSig,
			Sequence:     txin.Sequence,
		}, nil
	}
	return nil, ErrSerializeOutPoint
}

// FromProtoMessage converts proto message to txin.
func (txin *TxIn) FromProtoMessage(message proto.Message) error {
	if message, ok := message.(*corepb.TxIn); ok {
		if message != nil {
			outPoint := new(OutPoint)
			if err := outPoint.FromProtoMessage(message.PrevOutPoint); err != nil {
				return err
			}
			txin.PrevOutPoint = *outPoint
			txin.ScriptSig = message.ScriptSig
			txin.Sequence = message.Sequence
			return nil
		}
		return ErrEmptyProtoMessage
	}
	return ErrInvalidTxInProtoMessage
}

// ToProtoMessage converts out point to proto message.
func (op *OutPoint) ToProtoMessage() (proto.Message, error) {
	return &corepb.OutPoint{
		Hash:  op.Hash[:],
		Index: op.Index,
	}, nil
}

// FromProtoMessage converts proto message to out point.
func (op *OutPoint) FromProtoMessage(message proto.Message) error {
	if message, ok := message.(*corepb.OutPoint); ok {
		if message != nil {
			copy(op.Hash[:], message.Hash[:])
			op.Index = message.Index
			return nil
		}
		return ErrEmptyProtoMessage
	}

	return ErrInvalidOutPointProtoMessage
}

// TxHash returns tx hash
func (tx *Transaction) TxHash() (*crypto.HashType, error) {
	if tx.Hash != nil {
		return tx.Hash, nil
	}
	hash, err := tx.MsgTx.MsgTxHash()
	if err != nil {
		return nil, err
	}
	// cache it
	tx.Hash = hash
	return hash, nil
}

// MsgTxHash return msg tx hash
func (msgTx *MsgTx) MsgTxHash() (*crypto.HashType, error) {
	pbtx, err := msgTx.ToProtoMessage()
	if err != nil {
		return nil, err
	}
	rawMsgTx, err := proto.Marshal(pbtx)
	if err != nil {
		return nil, err
	}
	hash := crypto.DoubleHashH(rawMsgTx)
	return &hash, nil
}

// SerializeSize return msgTx size.
func (msgTx *MsgTx) SerializeSize() (int, error) {
	pbtx, err := msgTx.ToProtoMessage()
	if err != nil {
		return 0, err
	}
	rawMsgTx, err := proto.Marshal(pbtx)
	if err != nil {
		return 0, err
	}
	return len(rawMsgTx), nil
}
