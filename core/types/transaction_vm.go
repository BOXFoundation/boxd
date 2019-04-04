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
	Version     int32
	SenderNonce uint64
	HashWith    *crypto.HashType
	Sender      *AddressHash
	Receiver    *AddressHash
	Value       *big.Int
	GasPrice    *big.Int
	Gas         uint64
	Code        []byte
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
		Version:     0,
		SenderNonce: ^(uint64(0)),
		Receiver:    receiver,
		Value:       value,
		GasPrice:    gasPrice,
		Gas:         gas,
		Code:        code,
	}
}

// WithSenderNonce sets SenderNonce
func (tx *BoxTransaction) WithSenderNonce(n uint64) *BoxTransaction {
	tx.SenderNonce = n
	return tx
}

// WithSender sets SenderNonce
func (tx *BoxTransaction) WithSender(sender *AddressHash) *BoxTransaction {
	tx.Sender = sender
	return tx
}

// WithHashWith sets SenderNonce
func (tx *BoxTransaction) WithHashWith(hash *crypto.HashType) *BoxTransaction {
	tx.HashWith = hash
	return tx
}

// WithVoutNum sets VoutNum
func (tx *BoxTransaction) WithVoutNum(n uint32) *BoxTransaction {
	tx.VoutNum = n
	return tx
}
