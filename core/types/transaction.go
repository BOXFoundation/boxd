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
	ErrSerializeOutPoint           = errors.New("serialize outpoint error")
	ErrInvalidOutPointProtoMessage = errors.New("Invalid OutPoint proto message")
	ErrInvalidTxInProtoMessage     = errors.New("Invalid TxIn proto message")
	ErrInvalidTxOutProtoMessage    = errors.New("Invalid TxOut proto message")
	ErrInvalidTxProtoMessage       = errors.New("Invalid tx proto message")
)

// Transaction defines a transaction.
type Transaction struct {
	version  int32
	vin      []*TxIn
	vout     []*TxOut
	magic    uint32
	lockTime int64
}

// TxOut defines a transaction output.
type TxOut struct {
	value        int64
	scriptPubKey []byte
}

// TxIn defines a transaction input.
type TxIn struct {
	prevOutPoint *OutPoint
	scriptSig    []byte
	sequence     uint32
}

// OutPoint defines a data type that is used to track previous transaction outputs.
type OutPoint struct {
	hash  crypto.HashType
	index uint32
}

// Serialize transaction to proto message.
func (tx *Transaction) Serialize() (proto.Message, error) {

	var vins []*corepb.TxIn
	var vouts []*corepb.TxOut
	for _, v := range tx.vin {
		vin, err := v.Serialize()
		if err != nil {
			return nil, err
		}
		if vin, ok := vin.(*corepb.TxIn); ok {
			vins = append(vins, vin)
		}
	}
	for _, v := range tx.vout {
		vout, _ := v.Serialize()
		if vout, ok := vout.(*corepb.TxOut); ok {
			vouts = append(vouts, vout)
		}
	}
	return &corepb.Transaction{
		Version:  tx.version,
		Vin:      vins,
		Vout:     vouts,
		Magic:    tx.magic,
		LockTime: tx.lockTime,
	}, nil
}

// Deserialize convert proto message to transaction.
func (tx *Transaction) Deserialize(message proto.Message) error {

	if message, ok := message.(*corepb.Transaction); ok {
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
			for _, v := range message.Vin {
				txout := new(TxOut)
				if err := txout.Deserialize(v); err != nil {
					return err
				}
				vouts = append(vouts, txout)
			}

			tx.version = message.Version
			tx.vin = vins
			tx.vout = vouts
			tx.magic = message.Magic
			tx.lockTime = message.LockTime
			return nil
		}
		return ErrEmptyProtoMessage
	}
	return ErrInvalidTxInProtoMessage
}

// Serialize txout to proto message.
func (txout *TxOut) Serialize() (proto.Message, error) {

	return &corepb.TxOut{
		Value:        txout.value,
		ScriptPubKey: txout.scriptPubKey,
	}, nil
}

// Deserialize convert proto message to txout.
func (txout *TxOut) Deserialize(message proto.Message) error {

	if message, ok := message.(*corepb.TxOut); ok {
		if message != nil {
			txout.scriptPubKey = message.ScriptPubKey
			txout.value = message.Value
			return nil
		}
		return ErrEmptyProtoMessage
	}

	return ErrInvalidTxOutProtoMessage
}

// Serialize txin to proto message.
func (txin *TxIn) Serialize() (proto.Message, error) {

	prevOutPoint, _ := txin.prevOutPoint.Serialize()
	if prevOutPoint, ok := prevOutPoint.(*corepb.OutPoint); ok {
		return &corepb.TxIn{
			PrevOutPoint: prevOutPoint,
			ScriptSig:    txin.scriptSig,
			Sequence:     txin.sequence,
		}, nil
	}
	return nil, ErrSerializeOutPoint
}

// Deserialize convert proto message to txin.
func (txin *TxIn) Deserialize(message proto.Message) error {

	if message, ok := message.(*corepb.TxIn); ok {
		if message != nil {
			outpoint := new(OutPoint)
			if err := outpoint.Deserialize(message.PrevOutPoint); err != nil {
				return err
			}
			txin.prevOutPoint = outpoint
			txin.scriptSig = message.ScriptSig
			txin.sequence = message.Sequence
			return nil
		}
		return ErrEmptyProtoMessage
	}
	return ErrInvalidTxInProtoMessage
}

// Serialize out point to proto message.
func (op *OutPoint) Serialize() (proto.Message, error) {

	return &corepb.OutPoint{
		Hash:  op.hash[:],
		Index: op.index,
	}, nil
}

// Deserialize convert proto message to out point.
func (op *OutPoint) Deserialize(message proto.Message) error {

	if message, ok := message.(*corepb.OutPoint); ok {
		if message != nil {
			copy(op.hash[:], message.Hash[:])
			op.index = message.Index
			return nil
		}
		return ErrEmptyProtoMessage
	}

	return ErrInvalidOutPointProtoMessage
}
