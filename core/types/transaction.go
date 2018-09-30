// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package types

import (
	"errors"

	corepb "github.com/BOXFoundation/Quicksilver/core/pb"
	"github.com/BOXFoundation/Quicksilver/crypto"
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

// TxOut defines a transaction output.
type TxOut struct {
	Value        int64
	ScriptPubKey []byte
}

// TxIn defines a transaction input.
type TxIn struct {
	PrevOutPoint *OutPoint
	ScriptSig    []byte
	Sequence     uint32
}

// OutPoint defines a data type that is used to track previous transaction outputs.
type OutPoint struct {
	Hash  crypto.HashType
	Index uint32
}

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

// Serialize transaction to proto message.
func (msgTx *MsgTx) Serialize() (proto.Message, error) {

	var vins []*corepb.TxIn
	var vouts []*corepb.TxOut
	for _, v := range msgTx.Vin {
		vin, err := v.Serialize()
		if err != nil {
			return nil, err
		}
		if vin, ok := vin.(*corepb.TxIn); ok {
			vins = append(vins, vin)
		}
	}
	for _, v := range msgTx.Vout {
		vout, _ := v.Serialize()
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

// Deserialize convert proto message to transaction.
func (msgTx *MsgTx) Deserialize(message proto.Message) error {

	if message, ok := message.(*corepb.MsgTx); ok {
		if message != nil {
			var vins []*TxIn
			for _, v := range message.Vin {
				txin := new(TxIn)
				if err := txin.Deserialize(v); err != nil {
					return err
				}
				vins = append(vins, txin)
			}

			var vouts []*TxOut
			for _, v := range message.Vout {
				txout := new(TxOut)
				if err := txout.Deserialize(v); err != nil {
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

// Serialize txout to proto message.
func (txout *TxOut) Serialize() (proto.Message, error) {

	return &corepb.TxOut{
		Value:        txout.Value,
		ScriptPubKey: txout.ScriptPubKey,
	}, nil
}

// Deserialize convert proto message to txout.
func (txout *TxOut) Deserialize(message proto.Message) error {

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

// Serialize txin to proto message.
func (txin *TxIn) Serialize() (proto.Message, error) {
	if txin.PrevOutPoint == nil {
		return nil, ErrSerializeOutPoint
	}
	prevOutPoint, _ := txin.PrevOutPoint.Serialize()
	if prevOutPoint, ok := prevOutPoint.(*corepb.OutPoint); ok {
		return &corepb.TxIn{
			PrevOutPoint: prevOutPoint,
			ScriptSig:    txin.ScriptSig,
			Sequence:     txin.Sequence,
		}, nil
	}
	return nil, ErrSerializeOutPoint
}

// Deserialize convert proto message to txin.
func (txin *TxIn) Deserialize(message proto.Message) error {

	if message, ok := message.(*corepb.TxIn); ok {
		if message != nil {
			outPoint := new(OutPoint)
			if err := outPoint.Deserialize(message.PrevOutPoint); err != nil {
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

// Serialize out point to proto message.
func (op *OutPoint) Serialize() (proto.Message, error) {

	return &corepb.OutPoint{
		Hash:  op.Hash[:],
		Index: op.Index,
	}, nil
}

// Deserialize convert proto message to out point.
func (op *OutPoint) Deserialize(message proto.Message) error {

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
	pbtx, err := msgTx.Serialize()
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
	pbtx, err := msgTx.Serialize()
	if err != nil {
		return 0, err
	}
	rawMsgTx, err := proto.Marshal(pbtx)
	if err != nil {
		return 0, err
	}
	return len(rawMsgTx), nil
}
