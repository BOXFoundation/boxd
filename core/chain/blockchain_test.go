// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package chain

import (
	"os"
	"testing"

	"github.com/BOXFoundation/boxd/p2p"
	"github.com/BOXFoundation/boxd/storage"
	_ "github.com/BOXFoundation/boxd/storage/memdb" // init memdb
	"github.com/facebookgo/ensure"
	"github.com/jbenet/goprocess"
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

// Test blockchain block processing
func TestBlockProcessing(t *testing.T) {
	blockChain := genNewChain()
	ensure.NotNil(t, blockChain)
	ensure.True(t, blockChain.LongestChainHeight == 0)
	tailBlock, _ := blockChain.LoadTailBlock()
	ensure.DeepEqual(t, tailBlock, &genesisBlock)

	// try to append an existing block: genesis block
	isMainChain, isOrphan, err := blockChain.ProcessBlock(&genesisBlock, false)
	ensure.False(t, isMainChain)
	ensure.False(t, isOrphan)
	ensure.DeepEqual(t, err, ErrBlockExists)
}
