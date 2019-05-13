// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package test

import (
	"math/big"

	"github.com/BOXFoundation/boxd/core/types"
)

// Message is used for evm test.
type Message struct {
	from     *types.AddressHash
	gasPrice *big.Int
}

// NewMessage new msg.
func NewMessage(from *types.AddressHash, gasPrice *big.Int) Message {
	return Message{
		from:     from,
		gasPrice: gasPrice,
	}
}

// From get from.
func (msg Message) From() *types.AddressHash { return msg.from }

// To get to.
func (msg Message) To() *types.AddressHash { return &types.AddressHash{} }

// GasPrice get gas price.
func (msg Message) GasPrice() *big.Int { return msg.gasPrice }

// Gas get gas.
func (msg Message) Gas() uint64 { return 0 }

// Value get value.
func (msg Message) Value() *big.Int { return big.NewInt(0) }

// Nonce get nonce.
func (msg Message) Nonce() uint64 { return 0 }

// CheckNonce check nonce.
func (msg Message) CheckNonce() bool { return true }

// Data get data.
func (msg Message) Data() []byte { return []byte{} }

// Type get type.
func (msg Message) Type() types.ContractType { return types.ContractCreationType }
