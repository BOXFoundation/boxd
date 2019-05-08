// Copyright 2016 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package vm

import (
	"math/big"

	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/crypto"
	vmtypes "github.com/BOXFoundation/boxd/vm/common/types"
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

	AddLog(*vmtypes.Log)
	AddPreimage(crypto.HashType, []byte)

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
