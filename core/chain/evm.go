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

package chain

import (
	"math/big"

	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/crypto"
	"github.com/BOXFoundation/boxd/vm"
)

// Transfers used to record the transfer information
var (
	// Transfers          map[types.AddressHash][]*TransferInfo
	// TransferToContract *types.Transaction
	Transfers = make(map[types.AddressHash][]*TransferInfo)
)

// TransferInfo used to record the transfer information occurred during the execution of the contract
type TransferInfo struct {
	from  types.AddressHash
	to    types.AddressHash
	value *big.Int
}

// NewTransferInfo creates a new transferInfo.
func NewTransferInfo(from, to types.AddressHash, value *big.Int) *TransferInfo {
	return &TransferInfo{
		from:  from,
		to:    to,
		value: value,
	}
}

// NewEVMContext creates a new context for use in the EVM.
func NewEVMContext(msg Message, header *types.BlockHeader, bc *BlockChain) vm.Context {
	// If we don't have an explicit author (i.e. not mining), extract from the header

	return vm.Context{
		CanTransfer: CanTransfer,
		Transfer:    Transfer,
		GetHash:     GetHashFn(bc),
		Origin:      *msg.From(),
		BlockNumber: new(big.Int).Set(big.NewInt(int64(header.Height))),
		Time:        new(big.Int).Set(big.NewInt(header.TimeStamp)),
		// GasLimit:    header.GasLimit,
		GasPrice: msg.GasPrice(),
	}
}

// GetHashFn returns a GetHashFunc which retrieves header hashes by number
func GetHashFn(bc *BlockChain) func(n uint64) *crypto.HashType {
	return func(n uint64) *crypto.HashType {
		hash, _ := bc.GetBlockHash(uint32(n))
		return hash
	}
}

// CanTransfer checks whether there are enough funds in the address' account to make a transfer.
// This does not take the necessary gas in to account to make the transfer valid.
func CanTransfer(db vm.StateDB, addr types.AddressHash, amount *big.Int) bool {
	return db.GetBalance(addr).Cmp(amount) >= 0
}

// Transfer subtracts amount from sender and adds amount to recipient using the given Db
func Transfer(db vm.StateDB, sender, recipient types.AddressHash, amount *big.Int, interpreterInvoke bool) {
	db.SubBalance(sender, amount)
	db.AddBalance(recipient, amount)
	if interpreterInvoke && amount.Uint64() > 0 {
		transferInfo := NewTransferInfo(sender, recipient, amount)
		if v, ok := Transfers[sender]; ok {
			v = append(v, transferInfo)
		} else {
			Transfers[sender] = []*TransferInfo{transferInfo}
		}
	}
}
