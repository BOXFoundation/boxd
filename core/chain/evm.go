// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package chain

import (
	"math/big"

	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/crypto"
	"github.com/BOXFoundation/boxd/vm"
)

// NewEVMContext creates a new context for use in the EVM.
func NewEVMContext(msg types.Message, header *types.BlockHeader, bc *BlockChain) vm.Context {
	// If we don't have an explicit author (i.e. not mining), extract from the header
	return vm.Context{
		CanTransfer: CanTransfer,
		Transfer:    Transfer,
		GetHash:     GetHashFn(bc),
		Coinbase:    header.BookKeeper,
		Origin:      *msg.From(),
		BlockNumber: new(big.Int).Set(big.NewInt(int64(header.Height))),
		Time:        new(big.Int).Set(big.NewInt(header.TimeStamp)),
		GasPrice:    msg.GasPrice(),
		Nonce:       msg.Nonce(),
		Transfers:   make(map[types.AddressHash][]*vm.TransferInfo),
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
func Transfer(
	ctx *vm.Context, db vm.StateDB, sender, recipient types.AddressHash,
	amount *big.Int, interpreterInvoke bool,
) {
	// NOTE: amount is a re-used pointer varaible
	db.SubBalance(sender, amount)
	db.AddBalance(recipient, amount)
	if interpreterInvoke && amount.Uint64() > 0 {
		transferInfo := vm.NewTransferInfo(sender, recipient, amount.Uint64())
		logger.Debugf("new transfer info: sender: %x, recipient: %x, amount: %d",
			sender[:], recipient[:], amount)
		if v, ok := ctx.Transfers[sender]; ok {
			// if sender and recipient already exists in Transfers, update it instead
			// of append to it
			for _, w := range v {
				if w.To == recipient {
					// NOTE: cannot miss 'w.value = '
					w.Value += amount.Uint64()
					return
				}
			}
		}
		ctx.Transfers[sender] = append(ctx.Transfers[sender], transferInfo)
	}
}
