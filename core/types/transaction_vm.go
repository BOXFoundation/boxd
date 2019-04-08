// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package types

import (
	"math/big"

	"github.com/BOXFoundation/boxd/crypto"
)

// VMTransaction defines the transaction used to interact with vm
type VMTransaction struct {
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

// VMTxParams defines BoxTx params parsed from script pubkey
type VMTxParams struct {
	GasPrice uint64
	GasLimit uint64
	Version  int32
	Code     []byte
	Receiver *AddressHash
}

// NewVMTransaction new a VMTransaction instance with given parameters
func NewVMTransaction(
	value, gasPrice *big.Int, gas uint64, receiver *AddressHash, code []byte,
) *VMTransaction {
	return &VMTransaction{
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
func (tx *VMTransaction) WithSenderNonce(n uint64) *VMTransaction {
	tx.senderNonce = n
	return tx
}

// WithSender sets SenderNonce
func (tx *VMTransaction) WithSender(sender *AddressHash) *VMTransaction {
	tx.sender = sender
	return tx
}

// WithHashWith sets SenderNonce
func (tx *VMTransaction) WithHashWith(hash *crypto.HashType) *VMTransaction {
	tx.hashWith = hash
	return tx
}

// WithVoutNum sets VoutNum
func (tx *VMTransaction) WithVoutNum(n uint32) *VMTransaction {
	tx.VoutNum = n
	return tx
}

// From returns the tx from addressHash.
func (tx *VMTransaction) From() AddressHash {
	return *tx.sender
}

// To returns the tx to addressHash.
func (tx *VMTransaction) To() *AddressHash {
	return tx.receiver
}

// GasPrice returns the gasprice of the tx.
func (tx *VMTransaction) GasPrice() *big.Int {
	return tx.gasPrice
}

// Gas returns the gaslimit of the tx.
func (tx *VMTransaction) Gas() uint64 {
	return tx.gas
}

// Value returns the transfer value of the tx.
func (tx *VMTransaction) Value() *big.Int {
	return tx.value
}

// Nonce returns the nonce of the tx.
func (tx *VMTransaction) Nonce() uint64 {
	return tx.senderNonce
}

// CheckNonce returns if check nonce with the tx.
func (tx *VMTransaction) CheckNonce() bool {
	return true
}

// Data returns the code of the tx.
func (tx *VMTransaction) Data() []byte {
	return tx.code
}
