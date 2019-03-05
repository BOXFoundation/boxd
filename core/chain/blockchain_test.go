// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package chain

import (
	"testing"
	"time"

	"github.com/BOXFoundation/boxd/core"
	corepb "github.com/BOXFoundation/boxd/core/pb"
	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/crypto"
	"github.com/BOXFoundation/boxd/script"
	_ "github.com/BOXFoundation/boxd/storage/memdb"
	"github.com/facebookgo/ensure"
)

// test setup
var (
	privKey, pubKey, _ = crypto.NewKeyPair()
	minerAddr, _       = types.NewAddressFromPubKey(pubKey)
	scriptPubKey       = script.PayToPubKeyHashScript(minerAddr.Hash())
	blockChain         = NewTestBlockChain()
	timestamp          = time.Now().Unix()
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

// create a child tx spending parent tx's output
func createChildTx(parentTx *types.Transaction) *types.Transaction {
	outPoint := types.OutPoint{
		Hash:  *getTxHash(parentTx),
		Index: 0,
	}
	txIn := &types.TxIn{
		PrevOutPoint: outPoint,
		ScriptSig:    []byte{},
		Sequence:     0,
	}
	vIn := []*types.TxIn{
		txIn,
	}
	txOut := &corepb.TxOut{
		Value:        value,
		ScriptPubKey: *scriptPubKey,
	}
	vOut := []*corepb.TxOut{txOut}
	tx := &types.Transaction{
		Version:  1,
		Vin:      vIn,
		Vout:     vOut,
		Magic:    1,
		LockTime: 0,
	}

	// sign it
	for txInIdx, txIn := range tx.Vin {
		sigHash, err := script.CalcTxHashForSig(*scriptPubKey, tx, txInIdx)
		if err != nil {
			return nil
		}
		sig, err := crypto.Sign(privKey, sigHash)
		if err != nil {
			return nil
		}
		scriptSig := script.SignatureScript(sig, pubKey.Serialize())
		txIn.ScriptSig = *scriptSig

		// test to ensure
		if err = script.Validate(scriptSig, scriptPubKey, tx, txInIdx); err != nil {
			return nil
		}
	}
	return tx
}

func getTxHash(tx *types.Transaction) *crypto.HashType {
	txHash, _ := tx.TxHash()
	return txHash
}

// generate a child block
func nextBlock(parentBlock *types.Block) *types.Block {
	timestamp++
	newBlock := types.NewBlock(parentBlock)

	coinbaseTx, _ := CreateCoinbaseTx(minerAddr.Hash(), parentBlock.Height+1)
	// use time to ensure we create a different/unique block each time
	coinbaseTx.Vin[0].Sequence = uint32(time.Now().UnixNano())
	newBlock.Txs = []*types.Transaction{coinbaseTx}
	newBlock.Header.TxsRoot = *CalcTxsHash(newBlock.Txs)
	newBlock.Header.TimeStamp = timestamp
	return newBlock
}

func getTailBlock() *types.Block {
	tailBlock, _ := blockChain.loadTailBlock()
	return tailBlock
}

func verifyProcessBlock(t *testing.T, newBlock *types.Block, expectedErr error, expectedChainHeight uint32, expectedChainTail *types.Block) {

	err := blockChain.ProcessBlock(newBlock, core.DefaultMode /* not broadcast */, false, "")

	ensure.DeepEqual(t, err, expectedErr)
	ensure.DeepEqual(t, blockChain.LongestChainHeight, expectedChainHeight)
	ensure.DeepEqual(t, getTailBlock(), expectedChainTail)
}

// Test blockchain block processing
func TestBlockProcessing(t *testing.T) {
	ensure.NotNil(t, blockChain)
	ensure.True(t, blockChain.LongestChainHeight == 0)

	b0 := getTailBlock()

	// try to append an existing block: genesis block
	verifyProcessBlock(t, b0, core.ErrBlockExists, 0, b0)

	// extend main chain
	// b0 -> b1
	b1 := nextBlock(b0)
	verifyProcessBlock(t, b1, nil, 1, b1)

	b1DoubleMint := nextBlock(b1)
	b1DoubleMint.Header.TimeStamp = b1.Header.TimeStamp
	verifyProcessBlock(t, b1DoubleMint, core.ErrRepeatedMintAtSameTime, 1, b1)

	// extend main chain
	// b0 -> b1 -> b2
	b2 := nextBlock(b1)
	// add a tx spending from previous block's coinbase
	b2.Txs = append(b2.Txs, createChildTx(b1.Txs[0]))
	b2.Header.TxsRoot = *CalcTxsHash(b2.Txs)
	verifyProcessBlock(t, b2, nil, 2, b2)
	// extend side chain: fork from b1
	// b0 -> b1 -> b2
	//		   \-> b2A
	b2A := nextBlock(b1)
	verifyProcessBlock(t, b2A, core.ErrBlockInSideChain, 2, b2)

	// reorg: side chain grows longer than main chain
	// b0 -> b1 -> b2
	//		   \-> b2A -> b3A
	b3A := nextBlock(b2A)
	verifyProcessBlock(t, b3A, nil, 3, b3A)

	// Extend b2 fork twice to make first chain longer and force reorg
	// b0 -> b1 -> b2  -> b3  -> b4
	//		   \-> b2A -> b3A
	b3 := nextBlock(b2)
	verifyProcessBlock(t, b3, core.ErrBlockInSideChain, 3, b3A)
	b4 := nextBlock(b3)
	verifyProcessBlock(t, b4, nil, 4, b4)

	// Third fork
	// b0 -> b1 -> b2  -> b3  ->  b4
	//						  \-> b4B -> b5B
	//		   \-> b2A -> b3A
	b4B := nextBlock(b3)
	verifyProcessBlock(t, b4B, core.ErrBlockInSideChain, 4, b4)
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
}

func TestBlockChain_WriteDelTxIndex(t *testing.T) {
	ensure.NotNil(t, blockChain)

	b0 := getTailBlock()

	b1 := nextBlock(b0)
	batch := blockChain.db.NewBatch()
	ensure.Nil(t, blockChain.StoreBlockInBatch(b1, batch))

	txhash, _ := b1.Txs[0].TxHash()

	ensure.Nil(t, blockChain.WriteTxIndex(b1, []*types.Transaction{}, batch))
	batch.Write()

	_, tx, err := blockChain.LoadBlockInfoByTxHash(*txhash)
	ensure.Nil(t, err)
	ensure.DeepEqual(t, b1.Txs[0], tx)

	ensure.Nil(t, blockChain.DelTxIndex(b1, []*types.Transaction{}, batch))
	batch.Write()
	_, _, err = blockChain.LoadBlockInfoByTxHash(*txhash)
	ensure.NotNil(t, err)
}
