// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package types

import (
	"math/big"
)

// Message represents a message sent to a contract.
type Message interface {
	From() *AddressHash
	To() *AddressHash

	GasPrice() *big.Int
	Gas() uint64
	Value() *big.Int

	Type() ContractType

	Nonce() uint64
	Data() []byte
	String() string
}
