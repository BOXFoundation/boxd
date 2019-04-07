// Copyright 2014 The go-ethereum Authors
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

// Package state provides a caching layer atop the Ethereum state trie.
package state

import (
	"math/big"

	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/crypto"
	"github.com/BOXFoundation/boxd/storage"
)

// StateDBs within the ethereum protocol are used to store anything
// within the merkle trie. StateDBs take care of caching and storing
// nested states. It's the general query interface to retrieve:
// * Contracts
// * Accounts
type StateDB struct {
	db storage.Table
}

func New(root *crypto.HashType, db storage.Table) (*StateDB, error) {
	return &StateDB{db: db}, nil
}

func (s *StateDB) DB() storage.Table               { return s.db }
func (s *StateDB) CreateAccount(types.AddressHash) {}

func (s *StateDB) SubBalance(types.AddressHash, *big.Int) {}
func (s *StateDB) AddBalance(types.AddressHash, *big.Int) {}
func (s *StateDB) GetBalance(types.AddressHash) *big.Int  { return nil }

func (s *StateDB) GetNonce(types.AddressHash) uint64  { return 0 }
func (s *StateDB) SetNonce(types.AddressHash, uint64) {}

func (s *StateDB) GetCodeHash(types.AddressHash) crypto.HashType { return crypto.HashType{} }
func (s *StateDB) GetCode(types.AddressHash) []byte              { return []byte{} }
func (s *StateDB) SetCode(types.AddressHash, []byte)             {}
func (s *StateDB) GetCodeSize(types.AddressHash) int             { return 0 }

func (s *StateDB) AddRefund(uint64)  {}
func (s *StateDB) SubRefund(uint64)  {}
func (s *StateDB) GetRefund() uint64 { return 0 }

func (s *StateDB) GetCommittedState(types.AddressHash, crypto.HashType) crypto.HashType {
	return crypto.HashType{}
}
func (s *StateDB) GetState(types.AddressHash, crypto.HashType) crypto.HashType {
	return crypto.HashType{}
}
func (s *StateDB) SetState(types.AddressHash, crypto.HashType, crypto.HashType) {}

func (s *StateDB) Suicide(types.AddressHash) bool     { return true }
func (s *StateDB) HasSuicided(types.AddressHash) bool { return true }

// Exist reports whether the given account exists in state.
// Notably this should also return true for suicided accounts.
func (s *StateDB) Exist(types.AddressHash) bool { return true }

// Empty returns whether the given account is empty. Empty
// is defined according to EIP161 (balance = nonce = code = 0).
func (s *StateDB) Empty(types.AddressHash) bool { return true }
func (s *StateDB) RevertToSnapshot(int)         {}
func (s *StateDB) Snapshot() int                { return 0 }

// AddLog(*types.Log)
func (s *StateDB) AddPreimage(crypto.HashType, []byte) {}

func (s *StateDB) ForEachStorage(types.AddressHash, func(crypto.HashType, crypto.HashType) bool) {}
