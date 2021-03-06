// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package types

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"

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

// TxType defines tx's type
type TxType int

// pay to pubkey tx set 1 at most right bit
// contract tx set 1 at second right bit
const (
	UnknownTx       TxType = 0x0
	PayToPubkTx     TxType = 0x01
	ContractTx      TxType = 0x02
	TokenIssueTx    TxType = 0x04
	TokenTransferTx TxType = 0x05
	NormalTx        TxType = (PayToPubkTx | ContractTx | TokenIssueTx | TokenTransferTx)
	SplitTx         TxType = 0x08
	InternalTx      TxType = 0x10
	ErrorTx         TxType = math.MaxUint32
)

// TokenID defines token id
type TokenID OutPoint

// TxOut defines Transaction output as corepb.TxOut
type TxOut corepb.TxOut

// MarshalJSON implements TxOut json marshaler interface
func (txout *TxOut) MarshalJSON() ([]byte, error) {
	out := struct {
		Value        uint64
		ScriptPubKey string
	}{
		txout.Value,
		hex.EncodeToString(txout.ScriptPubKey),
	}
	return json.Marshal(out)
}

// Data defines transaction payload as corepb.Data
type Data corepb.Data

// DataType is used to indicate Transaction type filled in Data.Type
type DataType int32

//
const (
	GeneralDataType DataType = iota
	ContractDataType
)

// NewData news a Data instance
func NewData(typ DataType, data []byte) *Data {
	return &Data{Type: int32(typ), Content: data}
}

// MarshalJSON implements Data json marshaler interface
func (data *Data) MarshalJSON() ([]byte, error) {
	if data == nil {
		return nil, nil
	}
	content, suffix := data.Content, ""
	if len(content) > 256 {
		content = append(content[:100])
		suffix = "..."
	}
	td := struct {
		Type    int32
		Content string
	}{
		data.Type,
		hex.EncodeToString(content) + suffix,
	}
	return json.Marshal(td)
}

// Transaction defines a transaction.
type Transaction struct {
	hash     *crypto.HashType
	Type     TxType
	Version  int32
	Vin      []*TxIn
	Vout     []*TxOut
	Data     *Data
	Magic    uint32
	LockTime int64
}

// TxWrap wrap transaction
type TxWrap struct {
	Tx             *Transaction
	AddedTimestamp int64
	Height         uint32
	IsScriptValid  bool
}

// GetTx return tx in TxWrap
func (w *TxWrap) GetTx() *Transaction {
	if w == nil {
		return nil
	}
	return w.Tx
}

// String prints TxWrap
func (w *TxWrap) String() string {
	txHash, _ := w.Tx.TxHash()
	return fmt.Sprintf("{Tx: %s, AddedTimestamp: %d, Height: %d, IsScriptValid: %t}",
		txHash, w.AddedTimestamp, w.Height, w.IsScriptValid)
}

var _ conv.Convertible = (*Transaction)(nil)
var _ conv.Serializable = (*Transaction)(nil)

// NewTx generates a new Transaction
func NewTx(ver int32, magic uint32, lockTime int64) *Transaction {
	return &Transaction{
		Version:  ver,
		Magic:    magic,
		LockTime: lockTime,
	}
}

// NewTxOut generates a new TxOut
func NewTxOut(value uint64, scriptPubKey []byte) *TxOut {
	return &TxOut{
		Value:        value,
		ScriptPubKey: scriptPubKey,
	}
}

// NewTxIn generates a new TxIn
func NewTxIn(prevOutPoint *OutPoint, scriptSig []byte, seq uint32) *TxIn {
	return &TxIn{
		PrevOutPoint: *prevOutPoint,
		ScriptSig:    scriptSig,
		Sequence:     seq,
	}
}

// AppendVin appends a tx in to tx
func (tx *Transaction) AppendVin(in ...*TxIn) *Transaction {
	tx.Vin = append(tx.Vin, in...)
	return tx
}

// AppendVout appends a tx out to tx
func (tx *Transaction) AppendVout(out ...*TxOut) *Transaction {
	tx.Vout = append(tx.Vout, out...)
	return tx
}

// WithData sets Data to tx
func (tx *Transaction) WithData(typ DataType, code []byte) *Transaction {
	tx.Data = NewData(typ, code)
	return tx
}

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
	if hash == nil {
		return &OutPoint{Index: index}
	}
	return &OutPoint{Hash: *hash, Index: index}
}

// NewNullOutPoint returns a null outpoint with zero hash and max uint32 index
func NewNullOutPoint() *OutPoint {
	return &OutPoint{Index: math.MaxUint32}
}

func (op OutPoint) String() string {
	return fmt.Sprintf("{Hash: %s, Index: %d}", op.Hash, op.Index)
}

// IsContractType tells if the OutPoint is an OutPoint pointing to contract
func (op OutPoint) IsContractType() bool {
	if op.Index != 0 {
		return false
	}
	if !bytes.Equal(op.Hash[:12], ZeroAddressHash[:12]) ||
		bytes.Equal(op.Hash[12:], ZeroAddressHash[12:]) {
		return false
	}
	return true
}

var _ conv.Convertible = (*OutPoint)(nil)
var _ conv.Serializable = (*OutPoint)(nil)

////////////////////////////////////////////////////////////////////////////////

// AddressHash returns the owner of this txin
// TODO: the method have not been tested and verified
func (txin *TxIn) AddressHash() *AddressHash {
	if txin.PrevOutPoint.Hash == crypto.ZeroHash {
		return nil
	}
	if len(txin.ScriptSig) <= crypto.PublicKeySize {
		return nil
	}
	addrBytes := crypto.Hash160(txin.ScriptSig[len(txin.ScriptSig)-crypto.PublicKeySize:])
	addrHash := new(AddressHash)
	addrHash.SetBytes(addrBytes)
	return addrHash
}

func (txin *TxIn) String() string {
	return fmt.Sprintf("{PrevOutPoint: %s, ScriptSig: %x, Sequence: %d}",
		txin.PrevOutPoint, txin.ScriptSig, txin.Sequence)
}

// MarshalJSON implements TxIn json marshaler interface
func (txin *TxIn) MarshalJSON() ([]byte, error) {
	in := struct {
		PrevOutPoint *OutPoint
		Sequence     uint32
	}{
		&txin.PrevOutPoint,
		txin.Sequence,
	}
	return json.Marshal(in)
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
	return calcDoubleHash(data), nil
}

// ResetTxHash reset txhash
func (tx *Transaction) ResetTxHash() {
	tx.hash = nil
}

// ToProtoMessage converts transaction to proto message.
func (tx *Transaction) ToProtoMessage() (proto.Message, error) {
	vins := make([]*corepb.TxIn, 0, len(tx.Vin))
	for _, v := range tx.Vin {
		vin, err := v.ToProtoMessage()
		if err != nil {
			return nil, err
		}
		if vin, ok := vin.(*corepb.TxIn); ok {
			vins = append(vins, vin)
		}
	}
	vouts := make([]*corepb.TxOut, 0, len(tx.Vout))
	for _, v := range tx.Vout {
		vouts = append(vouts, (*corepb.TxOut)(v))
	}

	return &corepb.Transaction{
		Version:  tx.Version,
		Vin:      vins,
		Vout:     vouts,
		Data:     (*corepb.Data)(tx.Data),
		Magic:    tx.Magic,
		LockTime: tx.LockTime,
	}, nil
}

// ConvToPbTx convert a types tx to corepb tx
func (tx *Transaction) ConvToPbTx() (*corepb.Transaction, error) {
	data, err := tx.ToProtoMessage()
	if err != nil {
		return nil, err
	}
	return data.(*corepb.Transaction), nil
}

// FromProtoMessage converts proto message to transaction.
func (tx *Transaction) FromProtoMessage(message proto.Message) error {
	if message, ok := message.(*corepb.Transaction); ok {
		if message != nil {
			vins := make([]*TxIn, 0, len(message.Vin))
			for _, v := range message.Vin {
				txin := new(TxIn)
				if err := txin.FromProtoMessage(v); err != nil {
					return err
				}
				vins = append(vins, txin)
			}
			vouts := make([]*TxOut, 0, len(message.Vout))
			for _, v := range message.Vout {
				vouts = append(vouts, (*TxOut)(v))
			}

			// fill in hash
			tx.hash, _ = calcProtoMsgDoubleHash(message)
			tx.Version = message.Version
			tx.Vin = vins
			tx.Vout = vouts
			tx.Data = (*Data)(message.Data)
			tx.Magic = message.Magic
			tx.LockTime = message.LockTime
			return nil
		}
		return core.ErrEmptyProtoMessage
	}
	return core.ErrInvalidTxProtoMessage
}

// ConvPbTx  convert a pb tx to types tx
func ConvPbTx(orig *corepb.Transaction) *Transaction {
	tx := new(Transaction)
	if err := tx.FromProtoMessage(orig); err != nil {
		return nil
	}
	return tx
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
	vin := make([]*TxIn, 0, len(tx.Vin))
	for _, txIn := range tx.Vin {
		txInCopy := &TxIn{
			PrevOutPoint: txIn.PrevOutPoint,
			ScriptSig:    txIn.ScriptSig,
			Sequence:     txIn.Sequence,
		}
		vin = append(vin, txInCopy)
	}

	vout := make([]*TxOut, 0, len(tx.Vout))
	for _, txOut := range tx.Vout {
		txOutCopy := &TxOut{
			Value:        txOut.Value,
			ScriptPubKey: txOut.ScriptPubKey,
		}
		vout = append(vout, txOutCopy)
	}

	return &Transaction{
		Version:  tx.Version,
		Type:     tx.Type,
		Vin:      vin,
		Vout:     vout,
		Data:     tx.Data,
		Magic:    tx.Magic,
		LockTime: tx.LockTime,
	}
}

// ExtraFee returns extra fee
func (tx *Transaction) ExtraFee() uint64 {
	return uint64((len(tx.Vin)+len(tx.Vout))/core.InOutNumPerExtraFee) * core.TransferFee
}

// RoughSize return size of transaction in memory
// NOTE: it only denote a rough transaction size to limit block size on packing block
func (tx *Transaction) RoughSize() int {
	n := 16
	for _, in := range tx.Vin {
		n += crypto.HashSize + 4 + 4 + len(in.ScriptSig)
	}
	for _, out := range tx.Vout {
		n += 8 + len(out.ScriptPubKey)
	}
	if tx.Data != nil {
		n += 4 + len(tx.Data.Content)
	}
	return n
}

// calcProtoMsgDoubleHash calculates double hash of proto msg
func calcProtoMsgDoubleHash(pb proto.Message) (*crypto.HashType, error) {
	data, err := proto.Marshal(pb)
	if err != nil {
		return nil, err
	}
	return calcDoubleHash(data), nil
}

// calcDoubleHash calculates double hash of bytes
func calcDoubleHash(data []byte) *crypto.HashType {
	hash := crypto.DoubleHashH(data)
	return &hash
}

// OutputAmount returns total amount from tx's outputs
func (tx *Transaction) OutputAmount() uint64 {
	totalOutputAmount := uint64(0)
	for _, txOut := range tx.Vout {
		totalOutputAmount += txOut.Value
	}
	return totalOutputAmount
}
