// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package chain

import (
	"os"
	"testing"

	"github.com/BOXFoundation/boxd/core"
	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/crypto"
	"github.com/BOXFoundation/boxd/p2p"
	"github.com/BOXFoundation/boxd/storage"
	_ "github.com/BOXFoundation/boxd/storage/memdb" // init memdb
	"github.com/facebookgo/ensure"
	"github.com/jbenet/goprocess"
)

// test setup
var (
	_, publicKey, _ = crypto.NewKeyPair()
	minerAddr, _    = types.NewAddressFromPubKey(publicKey)
	blockChain      = genNewChain()
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

// utility function to generate a chain
func genNewChain() *BlockChain {
	dbCfg := &storage.Config{
		Name: "memdb",
		Path: "~/tmp",
	}

	proc := goprocess.WithSignals(os.Interrupt)
	db, _ := storage.NewDatabase(proc, dbCfg)
	blockChain, _ := NewBlockChain(proc, p2p.NewDummyPeer(), db)
	dummyDpos := &DummyDpos{}
	blockChain.Setup(dummyDpos, nil)
	return blockChain
}

// generate a child block
func nextBlock(parentBlock *types.Block) *types.Block {
	newBlock := types.NewBlock(parentBlock)

	coinbaseTx, _ := CreateCoinbaseTx(minerAddr.ScriptAddress(), parentBlock.Height+1)
	newBlock.Txs = []*types.Transaction{coinbaseTx}
	newBlock.Header.TxsRoot = *CalcTxsHash(newBlock.Txs)
	return newBlock
}

func getTailBlock() *types.Block {
	tailBlock, _ := blockChain.LoadTailBlock()
	return tailBlock
}

func verifyProcessBlock(t *testing.T, newBlock *types.Block, expectedIsMainChain bool,
	expectedIsOrphan bool, expectedErr error, expectedChainHeight int32, expectedChainTail *types.Block) {

	isMainChain, isOrphan, err := blockChain.ProcessBlock(newBlock, false /* not broadcast */)

	ensure.DeepEqual(t, isMainChain, expectedIsMainChain)
	ensure.DeepEqual(t, isOrphan, expectedIsOrphan)
	ensure.DeepEqual(t, err, expectedErr)
	ensure.DeepEqual(t, blockChain.LongestChainHeight, expectedChainHeight)
	ensure.DeepEqual(t, getTailBlock(), expectedChainTail)
}

// Test blockchain block processing
func TestBlockProcessing(t *testing.T) {
	ensure.NotNil(t, blockChain)
	ensure.True(t, blockChain.LongestChainHeight == 0)

	b0 := getTailBlock()
	ensure.DeepEqual(t, b0, &genesisBlock)

	// try to append an existing block: genesis block
	verifyProcessBlock(t, b0, false, false, core.ErrBlockExists, 0, b0)

	// extend main chain
	// b0 -> b1
	b1 := nextBlock(b0)
	verifyProcessBlock(t, b1, true, false, nil, 1, b1)

	// extend main chain
	// b0 -> b1 -> b2
	b2 := nextBlock(b1)
	verifyProcessBlock(t, b2, true, false, nil, 2, b2)

	// extend side chain: fork from b1
	// b0 -> b1 -> b2
	//		   \-> b2A
	b2A := nextBlock(b1)
	verifyProcessBlock(t, b2A, false, false, core.ErrBlockExists, 2, b2)

	// reorg: side chain grows longer than main chain
	// b0 -> b1 -> b2
	//		   \-> b2A -> b3A
	b3A := nextBlock(b2A)
	verifyProcessBlock(t, b3A, true, false, nil, 3, b3A)

	// Extend b2 fork twice to make first chain longer and force reorg
	// b0 -> b1 -> b2  -> b3  -> b4
	//		   \-> b2A -> b3A
	b3 := nextBlock(b2)
	verifyProcessBlock(t, b3, false, false, core.ErrBlockExists, 3, b3A)
	b4 := nextBlock(b3)
	verifyProcessBlock(t, b4, true, false, nil, 4, b4)

	// Third fork
	// b0 -> b1 -> b2  -> b3  ->  b4
	//						  \-> b4B -> b5B
	//		   \-> b2A -> b3A
	b4B := nextBlock(b3)
	verifyProcessBlock(t, b4B, false, false, core.ErrBlockExists, 4, b4)
	b5B := nextBlock(b4B)
	verifyProcessBlock(t, b5B, true, false, nil, 5, b5B)

	// add b7 -> b8 -> b9 -> b10: added them to orphan pool
	// b0 -> b1 -> b2  -> b3  ->  b4
	//						  \-> b4B -> b5B
	//		   \-> b2A -> b3A
	// b7 -> b8 -> b9 -> b10
	// withhold b6 for now and add it later
	b6 := nextBlock(b5B)
	b7 := nextBlock(b6)
	verifyProcessBlock(t, b7, false, true, nil, 5, b5B)
	b8 := nextBlock(b7)
	verifyProcessBlock(t, b8, false, true, nil, 5, b5B)
	b9 := nextBlock(b8)
	verifyProcessBlock(t, b9, false, true, nil, 5, b5B)
	b10 := nextBlock(b9)
	verifyProcessBlock(t, b10, false, true, nil, 5, b5B)

	// add b7: already exists
	verifyProcessBlock(t, b7, false, false, core.ErrOrphanBlockExists, 5, b5B)

	// add b6:
	// b0 -> b1 -> b2  -> b3  ->  b4
	//						  \-> b4B -> b5B -> b6 -> b7 -> b8 -> b9 -> b10
	//		   \-> b2A -> b3A
	verifyProcessBlock(t, b6, true, false, nil, 10, b10)

	// add b11:
	// b0 -> b1 -> b2  -> b3  ->  b4
	//						  \-> b4B -> b5B -> b6 -> b7 -> b8 -> b9 -> b10 -> b11
	//		   \-> b2A -> b3A
	b11 := nextBlock(b10)
	verifyProcessBlock(t, b11, true, false, nil, 11, b11)

	// Double spend

	// Create a fork that ends with block that generates too much coinbase

	// Create block that spends a transaction from a block that failed to connect (due to containing a double spend)

	// Create block with an otherwise valid transaction in place of where the coinbase must be

	// Create block with no transactions

	// Create block with two coinbase transactions

	// Create block with duplicate transactions

	// Create a block that spends a transaction that does not exist

}

type DummyDpos struct {
}

func (dpos *DummyDpos) Run() error {
	return nil
}

func (dpos *DummyDpos) Stop() {

}

func (dpos *DummyDpos) StoreCandidateContext(crypto.HashType) error {
	return nil
}

func (dpos *DummyDpos) VerifySign(*types.Block) (bool, error) {
	return true, nil
}

func (dpos *DummyDpos) RecoverMint() {}

func (dpos *DummyDpos) StopMint() {}
