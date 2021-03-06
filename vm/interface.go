// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package vm

import (
	"math/big"

	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/crypto"
)

// StateDB is an EVM database for full state querying.
type StateDB interface {
	CreateAccount(types.AddressHash)

	SubBalance(types.AddressHash, *big.Int)
	AddBalance(types.AddressHash, *big.Int)
	GetBalance(types.AddressHash) *big.Int

	GetNonce(types.AddressHash) uint64
	SetNonce(types.AddressHash, uint64)

	GetCodeHash(types.AddressHash) crypto.HashType
	GetCode(types.AddressHash) []byte
	SetCode(types.AddressHash, []byte)
	GetCodeSize(types.AddressHash) int
	IsContractAddr(addr types.AddressHash) bool

	SetError(error)
	Error() error

	AddRefund(uint64)
	SubRefund(uint64)
	GetRefund() uint64

	UpdateUtxo(addr types.AddressHash, utxoBytes []byte) error

	GetCommittedState(types.AddressHash, crypto.HashType) crypto.HashType
	GetState(types.AddressHash, crypto.HashType) crypto.HashType
	SetState(types.AddressHash, crypto.HashType, crypto.HashType)

	Suicide(types.AddressHash) bool
	HasSuicided(types.AddressHash) bool

	// Exist reports whether the given account exists in state.
	// Notably this should also return true for suicided accounts.
	Exist(types.AddressHash) bool
	// Empty returns whether the given account is empty. Empty
	// is defined according to EIP161 (balance = nonce = code = 0).
	Empty(types.AddressHash) bool

	RevertToSnapshot(int)
	Snapshot() int

	AddLog(*types.Log)
	AddPreimage(crypto.HashType, []byte)
	GetLogs(hash crypto.HashType) []*types.Log

	THash() crypto.HashType

	ForEachStorage(types.AddressHash, func(crypto.HashType, crypto.HashType) bool)
}

// CallContext provides a basic interface for the EVM calling conventions. The EVM
// depends on this context being implemented for doing subcalls and initialising new EVM contracts.
type CallContext interface {
	// Call another contract
	Call(env *EVM, me ContractRef, addr types.AddressHash, data []byte, gas, value *big.Int) ([]byte, error)
	// Take another's contract code and execute within our own context
	CallCode(env *EVM, me ContractRef, addr types.AddressHash, data []byte, gas, value *big.Int) ([]byte, error)
	// Same as CallCode except sender and value is propagated from parent to child scope
	DelegateCall(env *EVM, me ContractRef, addr types.AddressHash, data []byte, gas *big.Int) ([]byte, error)
	// Create a new contract
	Create(env *EVM, me ContractRef, data []byte, gas, value *big.Int) ([]byte, types.AddressHash, error)
}
