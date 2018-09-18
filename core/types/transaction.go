// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package types

import (
	"github.com/BOXFoundation/Quicksilver/crypto"
)

// Transaction defines a transaction.
type Transaction struct {
	version  int32
	vin      []*TxIn
	vout     []*TxOut
	lockTime uint32
}

// TxOut defines a transaction output.
type TxOut struct {
	value        int64
	scriptPubKey []byte
}

// TxIn defines a transaction input.
type TxIn struct {
	prevOutPoint OutPoint
	scriptSig    []byte
	sequence     uint32
}

// OutPoint defines a data type that is used to track previous transaction outputs.
type OutPoint struct {
	hash  crypto.HashType
	index uint32
}
