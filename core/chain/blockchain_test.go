// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package chain

import (
	"testing"

	"github.com/BOXFoundation/boxd/core"
	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/crypto"
	_ "github.com/BOXFoundation/boxd/storage/memdb"
	"github.com/facebookgo/ensure"
)

// test setup
var (
	_, publicKey, _ = crypto.NewKeyPair()
	minerAddr, _    = types.NewAddressFromPubKey(publicKey)
	blockChain      = NewTestBlockChain()
)

// Test if appending a slice while looping over it using index works.
// Just to make sure compiler is not optimizing len() condition away.
func TestAppendInLoop(t *testing.T) {
	const n = 100
	samples := make([]int, n)
	num := 0
	// loop with index, not range
	for i := 0; i < len(samples); i++ {
		num++
		if i < n {
			// double samples
			samples = append(samples, 0)
		}
	}
	if num != 2*n {
		t.Errorf("Expect looping %d times, but got %d times instead", n, num)
	}
}

// generate a child block
func nextBlock(parentBlock *types.Block) *types.Block {
	newBlock := types.NewBlock(parentBlock)

	coinbaseTx, _ := CreateCoinbaseTx(minerAddr.Hash(), parentBlock.Height+1)
	newBlock.Txs = []*types.Transaction{coinbaseTx}
	newBlock.Header.TxsRoot = *CalcTxsHash(newBlock.Txs)
	return newBlock
}

func getTailBlock() *types.Block {
	tailBlock, _ := blockChain.loadTailBlock()
	return tailBlock
}

func verifyProcessBlock(t *testing.T, newBlock *types.Block, expectedErr error, expectedChainHeight uint32, expectedChainTail *types.Block) {

	err := blockChain.ProcessBlock(newBlock, false /* not broadcast */, false, "")

	// ensure.DeepEqual(t, isMainChain, expectedIsMainChain)
	// ensure.DeepEqual(t, isOrphan, expectedIsOrphan)
	ensure.DeepEqual(t, err, expectedErr)
	ensure.DeepEqual(t, blockChain.LongestChainHeight, expectedChainHeight)
	ensure.DeepEqual(t, getTailBlock(), expectedChainTail)
}

// Test blockchain block processing
func TestBlockProcessing(t *testing.T) {
	ensure.NotNil(t, blockChain)
	ensure.True(t, blockChain.LongestChainHeight == 0)

	b0 := getTailBlock()
	ensure.DeepEqual(t, b0, &GenesisBlock)

	// try to append an existing block: genesis block
	verifyProcessBlock(t, b0, core.ErrBlockExists, 0, b0)

	// extend main chain
	// b0 -> b1
	b1 := nextBlock(b0)
	verifyProcessBlock(t, b1, nil, 1, b1)

	// extend main chain
	// b0 -> b1 -> b2
	b2 := nextBlock(b1)
	verifyProcessBlock(t, b2, nil, 2, b2)

	// extend side chain: fork from b1
	// b0 -> b1 -> b2
	//		   \-> b2A
	b2A := nextBlock(b1)
	verifyProcessBlock(t, b2A, core.ErrBlockExists, 2, b2)

	// reorg: side chain grows longer than main chain
	// b0 -> b1 -> b2
	//		   \-> b2A -> b3A
	b3A := nextBlock(b2A)
	verifyProcessBlock(t, b3A, nil, 3, b3A)

	// Extend b2 fork twice to make first chain longer and force reorg
	// b0 -> b1 -> b2  -> b3  -> b4
	//		   \-> b2A -> b3A
	b3 := nextBlock(b2)
	verifyProcessBlock(t, b3, core.ErrBlockExists, 3, b3A)
	b4 := nextBlock(b3)
	verifyProcessBlock(t, b4, nil, 4, b4)

	// Third fork
	// b0 -> b1 -> b2  -> b3  ->  b4
	//						  \-> b4B -> b5B
	//		   \-> b2A -> b3A
	b4B := nextBlock(b3)
	verifyProcessBlock(t, b4B, core.ErrBlockExists, 4, b4)
	b5B := nextBlock(b4B)
	verifyProcessBlock(t, b5B, nil, 5, b5B)

	// add b7 -> b8 -> b9 -> b10: added them to orphan pool
	// b0 -> b1 -> b2  -> b3  ->  b4
	//						  \-> b4B -> b5B
	//		   \-> b2A -> b3A
	// b7 -> b8 -> b9 -> b10
	// withhold b6 for now and add it later
	b6 := nextBlock(b5B)
	b7 := nextBlock(b6)
	verifyProcessBlock(t, b7, nil, 5, b5B)
	b8 := nextBlock(b7)
	verifyProcessBlock(t, b8, nil, 5, b5B)
	b9 := nextBlock(b8)
	verifyProcessBlock(t, b9, nil, 5, b5B)
	b10 := nextBlock(b9)
	verifyProcessBlock(t, b10, nil, 5, b5B)

	// add b7: already exists
	verifyProcessBlock(t, b7, core.ErrBlockExists, 5, b5B)

	// add b6:
	// b0 -> b1 -> b2  -> b3  ->  b4
	//						  \-> b4B -> b5B -> b6 -> b7 -> b8 -> b9 -> b10
	//		   \-> b2A -> b3A
	verifyProcessBlock(t, b6, nil, 10, b10)

	// add b11:
	// b0 -> b1 -> b2  -> b3  ->  b4
	//						  \-> b4B -> b5B -> b6 -> b7 -> b8 -> b9 -> b10 -> b11
	//		   \-> b2A -> b3A
	b11 := nextBlock(b10)
	verifyProcessBlock(t, b11, nil, 11, b11)

	// TODOs
	// Double spend

	// Create a fork that ends with block that generates too much coinbase

	// Create block that spends a transaction from a block that failed to connect (due to containing a double spend)

	// Create block with an otherwise valid transaction in place of where the coinbase must be

	// Create block with no transactions

	// Create block with two coinbase transactions

	// Create block with duplicate transactions

	// Create a block that spends a transaction that does not exist

}

func TestBlockChain_WriteDelTxIndex(t *testing.T) {
	ensure.NotNil(t, blockChain)

	b0 := getTailBlock()

	b1 := nextBlock(b0)
	ensure.Nil(t, blockChain.StoreBlockToDb(b1))

	txhash, _ := b1.Txs[0].TxHash()

	ensure.Nil(t, blockChain.WriteTxIndex(b1))
	tx, err := blockChain.LoadTxByHash(*txhash)
	ensure.Nil(t, err)
	ensure.DeepEqual(t, b1.Txs[0], tx)

	ensure.Nil(t, blockChain.DelTxIndex(b1))
	_, err = blockChain.LoadTxByHash(*txhash)
	ensure.NotNil(t, err)
}
