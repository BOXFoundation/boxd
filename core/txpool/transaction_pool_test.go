// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package txpool

import (
	"os"
	"testing"

	"github.com/BOXFoundation/boxd/core"
	"github.com/BOXFoundation/boxd/core/chain"
	"github.com/BOXFoundation/boxd/core/pb"
	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/crypto"
	"github.com/BOXFoundation/boxd/p2p"
	"github.com/facebookgo/ensure"
	"github.com/jbenet/goprocess"
)

// test setup
var (
	proc        = goprocess.WithSignals(os.Interrupt)
	txpool      = NewTransactionPool(proc, p2p.NewDummyPeer(), nil)
	chainHeight = int32(0)
	utxoSet     = chain.NewUtxoSet()

	txOutIdx = uint32(0)
	value    = int64(1)
)

// create a child tx spending parent tx's output
func createChildTx(parentTxHash *crypto.HashType) *types.Transaction {
	outPoint := types.OutPoint{
		Hash:  *parentTxHash,
		Index: txOutIdx,
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
		ScriptPubKey: []byte{},
	}
	vOut := []*corepb.TxOut{txOut}
	return &types.Transaction{
		Version:  1,
		Vin:      vIn,
		Vout:     vOut,
		Magic:    1,
		LockTime: 0,
	}
}

func getTxHash(tx *types.Transaction) *crypto.HashType {
	txHash, _ := tx.TxHash()
	return txHash
}

// return new child tx
func verifyDoProcessTx(t *testing.T, parentTxHash *crypto.HashType, expectedErr error,
	isTransactionInPool, isOrphanInPool bool) *types.Transaction {

	tx := createChildTx(parentTxHash)
	err := txpool.doProcessTx(tx, chainHeight, utxoSet, false /* do not broadcast */)
	ensure.DeepEqual(t, err, expectedErr)
	verifyTxInPool(t, tx, isTransactionInPool, isOrphanInPool)

	return tx
}

func verifyTxInPool(t *testing.T, tx *types.Transaction, isTransactionInPool, isOrphanInPool bool) {
	txHash := getTxHash(tx)
	ensure.DeepEqual(t, isTransactionInPool, txpool.isTransactionInPool(txHash))
	ensure.DeepEqual(t, isOrphanInPool, txpool.isOrphanInPool(txHash))
}

func TestDoProcessTx(t *testing.T) {
	// Notations
	// txi <- txj: txj spends output of txi, i.e., is a child tx of txi
	// txi(m): txi is in main pool
	// txi(o): txi is in orphan pool
	// txi(): txi is in neither pool. This can happen, e.g., if a tx is a coinbase and rejected

	// tx0(o)
	// tx0 is not admitted into main pool since its referenced outpoint does not exist
	tx0 := verifyDoProcessTx(t, &crypto.HashType{0}, core.ErrOrphanTransaction, false, true)

	// mark tx0's output as unspent
	utxoSet.AddUtxo(tx0, 0, chainHeight)

	// tx0(o) <- tx1(m)
	// tx1 is admitted into main pool since it spends from a valid UTXO, i.e., tx0
	verifyDoProcessTx(t, getTxHash(tx0), nil, true, false)
}
