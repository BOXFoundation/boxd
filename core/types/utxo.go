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

// UtxoType descript utxo type
type UtxoType string

//
const (
	PayToPubkeyUtxo   UtxoType = "pay_to_pubkey_utxo"
	TokenIssueUtxo    UtxoType = "token_issue_utxo"
	TokenTransferUtxo UtxoType = "token_transfer_utxo"
	SplitAddrUtxo     UtxoType = "split_addr_utxo"
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

// SetValue set utxoWrap value.
func (utxoWrap *UtxoWrap) SetValue(value uint64) {
	utxoWrap.value = value
}

// Script returns the pubkey script of the output.
func (utxoWrap *UtxoWrap) Script() []byte {
	return utxoWrap.script
}
