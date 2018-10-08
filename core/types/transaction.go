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
	Hash     *crypto.HashType
	Version  int32
	Vin      []*TxIn
	Vout     []*TxOut
	Magic    uint32
	LockTime int64
}

var _ conv.Convertible = (*Transaction)(nil)
var _ conv.Serializable = (*Transaction)(nil)

// TxOut defines a transaction output.
type TxOut struct {
	Value        int64
	ScriptPubKey []byte
}

var _ conv.Convertible = (*TxOut)(nil)
var _ conv.Serializable = (*TxOut)(nil)

// TxIn defines a transaction input.
type TxIn struct {
	PrevOutPoint OutPoint
	ScriptSig    []byte
	Sequence     uint32
}

var _ conv.Convertible = (*TxIn)(nil)
var _ conv.Serializable = (*TxIn)(nil)

// OutPoint defines a data type that is used to track previous transaction outputs.
type OutPoint struct {
	Hash  crypto.HashType
	Index uint32
}

var _ conv.Convertible = (*OutPoint)(nil)
var _ conv.Serializable = (*OutPoint)(nil)

////////////////////////////////////////////////////////////////////////////////

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

// Marshal method marshal TxOut object to binary
func (txout *TxOut) Marshal() (data []byte, err error) {
	return conv.MarshalConvertible(txout)
}

// Unmarshal method unmarshal binary data to TxOut object
func (txout *TxOut) Unmarshal(data []byte) error {
	msg := &corepb.TxOut{}
	if err := proto.Unmarshal(data, msg); err != nil {
		return err
	}
	return txout.FromProtoMessage(msg)
}

////////////////////////////////////////////////////////////////////////////////

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

// Marshal method marshal TxIn object to binary
func (txin *TxIn) Marshal() (data []byte, err error) {
	return conv.MarshalConvertible(txin)
}

// Unmarshal method unmarshal binary data to TxIn object
func (txin *TxIn) Unmarshal(data []byte) error {
	msg := &corepb.TxIn{}
	if err := proto.Unmarshal(data, msg); err != nil {
		return err
	}
	return txin.FromProtoMessage(msg)
}

////////////////////////////////////////////////////////////////////////////////

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

// Marshal method marshal OutPoint object to binary
func (op *OutPoint) Marshal() (data []byte, err error) {
	return conv.MarshalConvertible(op)
}

// Unmarshal method unmarshal binary data to OutPoint object
func (op *OutPoint) Unmarshal(data []byte) error {
	msg := &corepb.OutPoint{}
	if err := proto.Unmarshal(data, msg); err != nil {
		return err
	}
	return op.FromProtoMessage(msg)
}

////////////////////////////////////////////////////////////////////////////////

// TxHash returns tx hash
func (tx *Transaction) TxHash() (*crypto.HashType, error) {
	if tx.Hash != nil {
		return tx.Hash, nil
	}

	data, err := tx.Marshal()
	if err != nil {
		return nil, err
	}
	hash, err := calcDoubleHash(data)
	if err != nil {
		return nil, err
	}
	// cache it
	tx.Hash = hash
	return hash, nil
}

// ToProtoMessage converts transaction to proto message.
func (tx *Transaction) ToProtoMessage() (proto.Message, error) {
	var vins []*corepb.TxIn
	var vouts []*corepb.TxOut
	for _, v := range tx.Vin {
		vin, err := v.ToProtoMessage()
		if err != nil {
			return nil, err
		}
		if vin, ok := vin.(*corepb.TxIn); ok {
			vins = append(vins, vin)
		}
	}
	for _, v := range tx.Vout {
		vout, _ := v.ToProtoMessage()
		if vout, ok := vout.(*corepb.TxOut); ok {
			vouts = append(vouts, vout)
		}
	}
	return &corepb.Transaction{
		Version:  tx.Version,
		Vin:      vins,
		Vout:     vouts,
		Magic:    tx.Magic,
		LockTime: tx.LockTime,
	}, nil
}

// FromProtoMessage converts proto message to transaction.
func (tx *Transaction) FromProtoMessage(message proto.Message) error {
	if message, ok := message.(*corepb.Transaction); ok {
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

			// fill in hash
			tx.Hash, _ = calcProtoMsgDoubleHash(message)
			tx.Version = message.Version
			tx.Vin = vins
			tx.Vout = vouts
			tx.Magic = message.Magic
			tx.LockTime = message.LockTime
			return nil
		}
		return ErrEmptyProtoMessage
	}
	return ErrInvalidTxInProtoMessage
}

// Marshal method marshal tx object to binary
func (tx *Transaction) Marshal() (data []byte, err error) {
	return conv.MarshalConvertible(tx)
}

// Unmarshal method unmarshal binary data to tx object
func (tx *Transaction) Unmarshal(data []byte) error {
	msg := &corepb.Transaction{}
	if err := proto.Unmarshal(data, msg); err != nil {
		return err
	}
	return tx.FromProtoMessage(msg)
}

// SerializeSize return tx size.
func (tx *Transaction) SerializeSize() (int, error) {
	serializedTx, err := tx.Marshal()
	if err != nil {
		return 0, err
	}
	return len(serializedTx), nil
}

// calcProtoMsgDoubleHash calculates double hash of proto msg
func calcProtoMsgDoubleHash(pb proto.Message) (*crypto.HashType, error) {
	data, err := proto.Marshal(pb)
	if err != nil {
		return nil, err
	}
	return calcDoubleHash(data)
}

// calcDoubleHash calculates double hash of bytes
func calcDoubleHash(data []byte) (*crypto.HashType, error) {
	hash := crypto.DoubleHashH(data)
	return &hash, nil
}
