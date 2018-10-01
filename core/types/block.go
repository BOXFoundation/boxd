// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package types

import (
	"time"

	"github.com/BOXFoundation/Quicksilver/crypto"
	"github.com/BOXFoundation/Quicksilver/log"
)

var logger = log.NewLogger("core:types") // logger

// Block defines a block containing msgBlock and height that provides easier and more efficient
// manipulation of raw blocks.  It also memoizes hashes for the block and its
// transactions on their first access so subsequent accesses don't have to
// repeat the relatively expensive hashing operations.
type Block struct {
	Hash     *crypto.HashType
	MsgBlock *MsgBlock
}

// NewBlock new a block from parent.
func NewBlock(parent *Block) *Block {
	return &Block{
		MsgBlock: &MsgBlock{
			Header: &BlockHeader{
				Magic:         parent.MsgBlock.Header.Magic,
				PrevBlockHash: *parent.Hash,
				TimeStamp:     time.Now().Unix(),
			},
			Txs:    make([]*MsgTx, 0),
			Height: parent.MsgBlock.Height + 1,
		},
	}
}

// BlockHash returns the block identifier hash for the Block.
func (block *Block) BlockHash() *crypto.HashType {
	if block.Hash != nil {
		return block.Hash
	}

	// Cache the block hash and return it.
	hash, err := block.MsgBlock.BlockHash()
	if err != nil {
		logger.Errorf("Failed to get block hash, err = %s", err.Error())
		return nil
	}
	block.Hash = hash
	return hash
}
