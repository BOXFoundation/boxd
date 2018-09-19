// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package types

import (
	"time"

	"github.com/BOXFoundation/Quicksilver/crypto"
)

// BlockHeader defines information about a block and is used in the
// block (MsgBlock) and headers (MsgHeaders) messages.
type BlockHeader struct {
	// Version of the block.  This is not the same as the protocol version.
	version int32

	// Hash of the previous block header in the block chain.
	prevBlock crypto.HashType

	// Merkle tree reference to hash of all transactions for the block.
	merkleRoot crypto.HashType

	dposContextRoot DposContext

	// Time the block was created.  This is, unfortunately, encoded as a
	// uint32 on the wire and therefore is limited to 2106.
	timestamp time.Time

	// Difficulty target for the block.
	bits uint32

	// Nonce used to generate the block.
	nonce uint32
}

// DposContext define struct
type DposContext struct {
	// TODO: fill in here or some other package
}

// Block defines a block containing header and transactions within it.
type Block struct {
	header *BlockHeader
	txs    []Transaction
}
