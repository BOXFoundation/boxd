// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package types

import (
	"fmt"

	"github.com/BOXFoundation/boxd/core"
	corepb "github.com/BOXFoundation/boxd/core/pb"
	"github.com/BOXFoundation/boxd/crypto"
	conv "github.com/BOXFoundation/boxd/p2p/convert"
	proto "github.com/gogo/protobuf/proto"
)

// Define const
const (
	GeneralTx = iota
	RegisterCandidateTx
	VoteTx
)

// Transaction defines a transaction.
type Transaction struct {
	hash     *crypto.HashType
	Version  int32
	Vin      []*TxIn
	Vout     []*corepb.TxOut
	Data     *corepb.Data
	Magic    uint32
	LockTime int64
}

var _ conv.Convertible = (*Transaction)(nil)
var _ conv.Serializable = (*Transaction)(nil)

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

// NewOutPoint constructs a OutPoint
func NewOutPoint(hash *crypto.HashType, index uint32) *OutPoint {
	return &OutPoint{
		Hash:  *hash,
		Index: index,
	}
}

func (op OutPoint) String() string {
	return fmt.Sprintf("{Hash: %s, Index: %d}", op.Hash, op.Index)
}

var _ conv.Convertible = (*OutPoint)(nil)
var _ conv.Serializable = (*OutPoint)(nil)

////////////////////////////////////////////////////////////////////////////////

func (txin *TxIn) String() string {
	return fmt.Sprintf("{PrevOutPoint: %s, ScriptSig: %s, Sequence: %d}",
		txin.PrevOutPoint, string(txin.ScriptSig), txin.Sequence)
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
	return nil, core.ErrSerializeOutPoint
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
		return core.ErrEmptyProtoMessage
	}
	return core.ErrInvalidTxInProtoMessage
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
		return core.ErrEmptyProtoMessage
	}

	return core.ErrInvalidOutPointProtoMessage
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

// TxHash returns tx hash; return cached hash if it exists
func (tx *Transaction) TxHash() (*crypto.HashType, error) {
	if tx.hash != nil {
		return tx.hash, nil
	}

	hash, err := tx.CalcTxHash()
	if err != nil {
		return nil, err
	}

	// cache it
	tx.hash = hash
	return hash, nil
}

// CalcTxHash calculates tx hash
func (tx *Transaction) CalcTxHash() (*crypto.HashType, error) {
	data, err := tx.Marshal()
	if err != nil {
		return nil, err
	}
	return calcDoubleHash(data)
}

// ToProtoMessage converts transaction to proto message.
func (tx *Transaction) ToProtoMessage() (proto.Message, error) {
	var vins []*corepb.TxIn
	for _, v := range tx.Vin {
		vin, err := v.ToProtoMessage()
		if err != nil {
			return nil, err
		}
		if vin, ok := vin.(*corepb.TxIn); ok {
			vins = append(vins, vin)
		}
	}

	return &corepb.Transaction{
		Version:  tx.Version,
		Vin:      vins,
		Vout:     tx.Vout,
		Data:     tx.Data,
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

			// fill in hash
			tx.hash, _ = calcProtoMsgDoubleHash(message)
			tx.Version = message.Version
			tx.Vin = vins
			tx.Vout = message.Vout
			tx.Data = message.Data
			tx.Magic = message.Magic
			tx.LockTime = message.LockTime
			return nil
		}
		return core.ErrEmptyProtoMessage
	}
	return core.ErrInvalidTxProtoMessage
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

// Copy returns a deep copy, mostly for parallel script verification
// Do not copy hash since it will be updated anyway in script verification
func (tx *Transaction) Copy() *Transaction {
	vin := make([]*TxIn, 0)
	for _, txIn := range tx.Vin {
		txInCopy := &TxIn{
			PrevOutPoint: txIn.PrevOutPoint,
			ScriptSig:    txIn.ScriptSig,
			Sequence:     txIn.Sequence,
		}
		vin = append(vin, txInCopy)
	}

	vout := make([]*corepb.TxOut, 0)
	for _, txOut := range tx.Vout {
		txOutCopy := &corepb.TxOut{
			Value:        txOut.Value,
			ScriptPubKey: txOut.ScriptPubKey,
		}
		vout = append(vout, txOutCopy)
	}

	data := &corepb.Data{}
	if tx.Data != nil {
		data.Type = tx.Data.Type
		copy(data.Content, tx.Data.Content)
	} else {
		data = nil
	}

	return &Transaction{
		Version:  tx.Version,
		Vin:      vin,
		Vout:     vout,
		Data:     data,
		Magic:    tx.Magic,
		LockTime: tx.LockTime,
	}
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

// OutputAmount returns total amount from tx's outputs
func (tx *Transaction) OutputAmount() uint64 {
	totalOutputAmount := uint64(0)
	for _, txOut := range tx.Vout {
		totalOutputAmount += txOut.Value
	}
	return totalOutputAmount
}
