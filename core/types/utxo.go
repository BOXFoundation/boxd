// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package types

type utxoFlags uint8

const (
	coinBase utxoFlags = 1 << iota

	spent

	modified
)

// UtxoWrap contains info about utxo
type UtxoWrap struct {
	value  uint64
	script []byte // The public key script for the output.
	height uint32 // Height of block containing tx.
	flags  utxoFlags
}

// UtxoMap defines a map type with OutPoint to UtxoWrap
type UtxoMap map[OutPoint]*UtxoWrap

// NewUtxoWrap returns new utxoWrap.
func NewUtxoWrap(value uint64, script []byte, height uint32) *UtxoWrap {
	return &UtxoWrap{
		value:  value,
		script: script,
		height: height,
		flags:  modified,
	}
}

// IsModified returns whether or not the output ismodified.
func (utxoWrap *UtxoWrap) IsModified() bool {
	return utxoWrap.flags&modified == modified
}

// Modified marks the output as modified.
func (utxoWrap *UtxoWrap) Modified() {
	utxoWrap.flags |= modified
}

// IsCoinBase returns whether or not the output was contained in a coinbase
// transaction.
func (utxoWrap *UtxoWrap) IsCoinBase() bool {
	return utxoWrap.flags&coinBase == coinBase
}

// SetCoinBase marks the output as coinBase.
func (utxoWrap *UtxoWrap) SetCoinBase() {
	utxoWrap.flags |= coinBase
}

// IsSpent returns whether or not the output has been spent.
func (utxoWrap *UtxoWrap) IsSpent() bool {
	return utxoWrap.flags&spent == spent
}

// Height returns the height of the block containing the output.
func (utxoWrap *UtxoWrap) Height() uint32 {
	return utxoWrap.height
}

// Spend marks the output as spent.
func (utxoWrap *UtxoWrap) Spend() {

	if utxoWrap.IsSpent() {
		return
	}
	utxoWrap.flags |= spent | modified
}

// UnSpend marks the output as unspent.
func (utxoWrap *UtxoWrap) UnSpend() {
	utxoWrap.flags = modified
	if utxoWrap.IsCoinBase() {
		utxoWrap.SetCoinBase()
	}
}

// Value returns the value of the output.
func (utxoWrap *UtxoWrap) Value() uint64 {
	return utxoWrap.value
}

// Script returns the pubkey script of the output.
func (utxoWrap *UtxoWrap) Script() []byte {
	return utxoWrap.script
}

// ToProtoMessage converts utxo wrap to proto message.
// func (utxoWrap *UtxoWrap) ToProtoMessage() (proto.Message, error) {
// 	return &corepb.UtxoWrap{
// 		Output:      utxoWrap.Output,
// 		BlockHeight: utxoWrap.BlockHeight,
// 		IsCoinbase:  utxoWrap.IsCoinBase,
// 		IsSpent:     utxoWrap.IsSpent,
// 	}, nil
// }

// // FromProtoMessage converts proto message to utxo wrap.
// func (utxoWrap *UtxoWrap) FromProtoMessage(message proto.Message) error {
// 	if message, ok := message.(*corepb.UtxoWrap); ok {
// 		utxoWrap.Output = message.Output
// 		utxoWrap.BlockHeight = message.BlockHeight
// 		utxoWrap.IsCoinBase = message.IsCoinbase
// 		utxoWrap.IsSpent = message.IsSpent
// 		utxoWrap.IsModified = false
// 		return nil
// 	}
// 	return core.ErrInvalidUtxoWrapProtoMessage
// }

// Marshal method marshal UtxoWrap object to binary
// func (utxoWrap *UtxoWrap) Marshal() (data []byte, err error) {
// 	return conv.MarshalConvertible(utxoWrap)
// }

// // Unmarshal method unmarshal binary data to UtxoWrap object
// func (utxoWrap *UtxoWrap) Unmarshal(data []byte) error {
// 	msg := &corepb.UtxoWrap{}
// 	if err := proto.Unmarshal(data, msg); err != nil {
// 		return err
// 	}
// 	return utxoWrap.FromProtoMessage(msg)
// }

// Value returns utxo amount
// func (utxoWrap *UtxoWrap) Value() uint64 {
// 	return utxoWrap.Output.Value
// }
