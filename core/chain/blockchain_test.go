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
	"github.com/BOXFoundation/boxd/util"
	"github.com/facebookgo/ensure"
	"github.com/jbenet/goprocess"
)

// test setup
var (
	_, publicKey, _ = crypto.NewKeyPair()
	minerAddr, _    = types.NewAddressFromPubKey(publicKey, 0x00 /* net ID */)
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
	return blockChain
}

// generate a child block
func nextBlock(parentBlock *types.Block) *types.Block {
	newBlock := types.NewBlock(parentBlock)

	coinbaseTx, _ := CreateCoinbaseTx(minerAddr, parentBlock.Height+1)
	newBlock.Txs = []*types.Transaction{coinbaseTx}
	newBlock.Header.TxsRoot = *util.CalcTxsHash(newBlock.Txs)
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
	verifyProcessBlock(t, b2A, false, false, nil, 2, b2)

	// reorg: side chain grows longer than main chain
	// b0 -> b1 -> b2
	//		   \-> b2A -> b3A
	b3A := nextBlock(b2A)
	verifyProcessBlock(t, b3A, true, false, nil, 3, b3A)
}
