// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package types

import (
	"time"

	"github.com/BOXFoundation/Quicksilver/crypto"
)

// Block defines a block containing msgBlock and height.
type Block struct {
	Hash     *crypto.HashType
	MsgBlock *MsgBlock
	Height   int
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
			Txs: make([]*MsgTx, 0),
		},
	}
}

// BlockHash returns the block identifier hash for the Block.
func (block *Block) BlockHash() (*crypto.HashType, error) {
	if block.Hash != nil {
		return block.Hash, nil
	}

	// Cache the block hash and return it.
	hash, err := block.MsgBlock.BlockHash()
	if err != nil {
		return nil, err
	}
	block.Hash = hash
	return hash, nil
}

// PackTxs packed txs and add them to block.
func (block *Block) PackTxs() {

}
