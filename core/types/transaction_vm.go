// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package types

import (
	"math/big"

	"github.com/BOXFoundation/boxd/crypto"
)

// ContractType defines script contract type
type ContractType string

//
const (
	VMVersion = 0

	ContractUnkownType   ContractType = "contract_unkown"
	ContractCreationType ContractType = "contract_creation"
	ContractCallType     ContractType = "contract_call"
)

// VMTransaction defines the transaction used to interact with vm
type VMTransaction struct {
	version  int32
	sender   *AddressHash
	originTx *crypto.HashType
	receiver *AddressHash
	value    *big.Int
	gasPrice *big.Int
	gas      uint64
	code     []byte
	typ      ContractType
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
	value, gasPrice *big.Int, gas uint64, hash *crypto.HashType, typ ContractType,
	code []byte,
) *VMTransaction {
	return &VMTransaction{
		version:  VMVersion,
		typ:      typ,
		value:    value,
		originTx: hash,
		gasPrice: gasPrice,
		gas:      gas,
		code:     code,
	}
}

// WithSender sets sender
func (tx *VMTransaction) WithSender(sender *AddressHash) *VMTransaction {
	tx.sender = sender
	return tx
}

// WithReceiver sets receiver
func (tx *VMTransaction) WithReceiver(receiver *AddressHash) *VMTransaction {
	tx.receiver = receiver
	return tx
}

// Version returns the version of the tx.
func (tx *VMTransaction) Version() int32 {
	return tx.version
}

// From returns the tx from addressHash.
func (tx *VMTransaction) From() *AddressHash {
	return tx.sender
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

//// Nonce returns the nonce of the tx.
//func (tx *VMTransaction) Nonce() uint64 {
//	return 0
//}
//
//// CheckNonce returns if check nonce with the tx.
//func (tx *VMTransaction) CheckNonce() bool {
//	return true
//}

// Data returns the code of the tx.
func (tx *VMTransaction) Data() []byte {
	return tx.code
}

// Type returns the type of the contract tx.
func (tx *VMTransaction) Type() ContractType {
	return tx.typ
}

// OriginTxHash returns the origin tx hash of the contract tx.
func (tx *VMTransaction) OriginTxHash() *crypto.HashType {
	return tx.originTx
}
