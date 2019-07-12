// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package worldstate

import (
	"math"
	"math/big"

	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/crypto"
	"golang.org/x/crypto/ripemd160"
)

// NewMockdb new Mockdb instead of StateDb.
func NewMockdb() *Mockdb {
	return &Mockdb{
		contracts: make(map[types.AddressHash][]uint8, 100),
		nonces:    make(map[types.AddressHash]uint64, 100),
		states:    make(map[crypto.HashType]crypto.HashType, 100),
		accounts:  make(map[types.AddressHash]uint64, 100),
	}
}

// Mockdb is used instead of StateDb for test.
type Mockdb struct {
	contracts map[types.AddressHash][]uint8
	nonces    map[types.AddressHash]uint64
	states    map[crypto.HashType]crypto.HashType
	accounts  map[types.AddressHash]uint64
}

// CreateAccount create account.
func (mock *Mockdb) CreateAccount(address types.AddressHash) {
	mock.accounts[address] = 0
}

// SubBalance sub balance.
func (mock *Mockdb) SubBalance(address types.AddressHash, amount *big.Int) {
	mock.accounts[address] -= amount.Uint64()
}

// AddBalance add balance.
func (mock *Mockdb) AddBalance(address types.AddressHash, amount *big.Int) {
	mock.accounts[address] += amount.Uint64()
}

// GetBalance get balance.
func (mock *Mockdb) GetBalance(address types.AddressHash) *big.Int {
	return big.NewInt(math.MaxUint32)
}

// GetNonce get nonce.
func (mock *Mockdb) GetNonce(address types.AddressHash) uint64 {
	return mock.nonces[address]
}

// SetNonce set nonce.
func (mock *Mockdb) SetNonce(address types.AddressHash, nonce uint64) {
	mock.nonces[address] = nonce
}

// GetCodeHash get code hash.
func (mock *Mockdb) GetCodeHash(address types.AddressHash) crypto.HashType {
	code := mock.contracts[address]
	if code == nil {
		return crypto.HashType{}
	}
	return crypto.BytesToHash(code)
}

// GetCode get code.
func (mock *Mockdb) GetCode(address types.AddressHash) []byte {
	code := mock.contracts[address]
	if code != nil {
		return code
	}
	return []byte{}
}

// SetCode set code.
func (mock *Mockdb) SetCode(address types.AddressHash, code []byte) {
	mock.contracts[address] = code
}

// GetCodeSize get code size.
func (mock *Mockdb) GetCodeSize(address types.AddressHash) int {
	code := mock.contracts[address]
	if code == nil {
		return 0
	}
	return len(code)
}

// AddRefund add refund.
func (mock *Mockdb) AddRefund(refund uint64) {
}

// SubRefund sub refund.
func (mock *Mockdb) SubRefund(refund uint64) {
}

// GetRefund get refund.
func (mock *Mockdb) GetRefund() uint64 {
	return 0
}

// GetCommittedState get committed state.
func (mock *Mockdb) GetCommittedState(address types.AddressHash, key crypto.HashType) crypto.HashType {
	return [crypto.HashSize]byte{}
}

// GetState get state.
func (mock *Mockdb) GetState(address types.AddressHash, key crypto.HashType) crypto.HashType {
	newKey := []byte{}
	newKey = append(newKey, address.Bytes()...)
	newKey = append(newKey, key.Bytes()[:crypto.HashSize-ripemd160.Size]...)
	return mock.states[crypto.BytesToHash(newKey)]
}

// SetState set state.
func (mock *Mockdb) SetState(address types.AddressHash, key, value crypto.HashType) {
	newKey := []byte{}
	newKey = append(newKey, address.Bytes()...)
	newKey = append(newKey, key.Bytes()[:crypto.HashSize-ripemd160.Size]...)
	mock.states[crypto.BytesToHash(newKey)] = value
}

// Suicide suicide.
func (mock *Mockdb) Suicide(address types.AddressHash) bool {
	return false
}

// HasSuicided judge suicided.
func (mock *Mockdb) HasSuicided(address types.AddressHash) bool {
	return false
}

// Exist judge exist.
func (mock *Mockdb) Exist(address types.AddressHash) bool {
	if mock.contracts[address] != nil {
		return true
	}
	return false
}

// Empty judge empty.
func (mock *Mockdb) Empty(address types.AddressHash) bool {
	return false
}

// RevertToSnapshot revert to snap.
func (mock *Mockdb) RevertToSnapshot(i int) {
}

// Snapshot record snap.
func (mock *Mockdb) Snapshot() int {
	return 0
}

// AddLog add log.
func (mock *Mockdb) AddLog(log *types.Log) {
}

// AddPreimage add preimage.
func (mock *Mockdb) AddPreimage(hash crypto.HashType, data []byte) {
}

// ForEachStorage traverse storage.
func (mock *Mockdb) ForEachStorage(types.AddressHash, func(crypto.HashType, crypto.HashType) bool) {
}
