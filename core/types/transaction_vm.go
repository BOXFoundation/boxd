// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package types

import (
	"math/big"

	"github.com/BOXFoundation/boxd/crypto"
)

// BoxTransaction defines the transaction used to interact with vm
type BoxTransaction struct {
	version     int32
	senderNonce uint64
	hashWith    *crypto.HashType
	sender      *AddressHash
	receiver    *AddressHash
	value       *big.Int
	gasPrice    *big.Int
	gas         uint64
	code        []byte
	VoutNum     uint32
}

// BoxTxParams defines BoxTx params parsed from script pubkey
type BoxTxParams struct {
	GasPrice uint64
	GasLimit uint64
	Version  int32
	Code     []byte
	Receiver *AddressHash
}

// NewBoxTransaction new a BoxTransaction instance with given parameters
func NewBoxTransaction(
	value, gasPrice *big.Int, gas uint64, receiver *AddressHash, code []byte,
) *BoxTransaction {
	return &BoxTransaction{
		version:     0,
		senderNonce: ^(uint64(0)),
		receiver:    receiver,
		value:       value,
		gasPrice:    gasPrice,
		gas:         gas,
		code:        code,
	}
}

// WithSenderNonce sets SenderNonce
func (tx *BoxTransaction) WithSenderNonce(n uint64) *BoxTransaction {
	tx.senderNonce = n
	return tx
}

// WithSender sets SenderNonce
func (tx *BoxTransaction) WithSender(sender *AddressHash) *BoxTransaction {
	tx.sender = sender
	return tx
}

// WithHashWith sets SenderNonce
func (tx *BoxTransaction) WithHashWith(hash *crypto.HashType) *BoxTransaction {
	tx.hashWith = hash
	return tx
}

// WithVoutNum sets VoutNum
func (tx *BoxTransaction) WithVoutNum(n uint32) *BoxTransaction {
	tx.VoutNum = n
	return tx
}

// From returns the tx from addressHash.
func (tx *BoxTransaction) From() AddressHash {
	return *tx.sender
}

// To returns the tx to addressHash.
func (tx *BoxTransaction) To() *AddressHash {
	return tx.receiver
}

// GasPrice returns the gasprice of the tx.
func (tx *BoxTransaction) GasPrice() *big.Int {
	return tx.gasPrice
}

// Gas returns the gaslimit of the tx.
func (tx *BoxTransaction) Gas() uint64 {
	return tx.gas
}

// Value returns the transfer value of the tx.
func (tx *BoxTransaction) Value() *big.Int {
	return tx.value
}

// Nonce returns the nonce of the tx.
func (tx *BoxTransaction) Nonce() uint64 {
	return tx.senderNonce
}

// CheckNonce returns if check nonce with the tx.
func (tx *BoxTransaction) CheckNonce() bool {
	return true
}

// Data returns the code of the tx.
func (tx *BoxTransaction) Data() []byte {
	return tx.code
}
