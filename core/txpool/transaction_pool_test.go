// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package txpool

import (
	"os"
	"testing"

	"github.com/BOXFoundation/boxd/core"
	"github.com/BOXFoundation/boxd/core/chain"
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

	_, publicKey, _ = crypto.NewKeyPair()
	minerAddr, _    = types.NewAddressFromPubKey(publicKey)
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
	txOut := &types.TxOut{
		Value:        value,
		ScriptPubKey: []byte{},
	}
	vOut := []*types.TxOut{txOut}
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
	tx.Hash, _ = tx.TxHash()
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

	// a duplicate tx0, already exists in tx pool
	verifyDoProcessTx(t, &crypto.HashType{0}, core.ErrDuplicateTxInPool, false, true)

	// mark tx0's output as unspent
	utxoSet.AddUtxo(tx0, 0, chainHeight)

	// tx0(o) <- tx1(m)
	// tx1 is admitted into main pool since it spends from a valid UTXO, i.e., tx0
	tx1 := verifyDoProcessTx(t, getTxHash(tx0), nil, true, false)

	utxoSet.AddUtxo(tx1, 0, chainHeight)
	// tx1(m) <- tx2(m)
	tx2 := verifyDoProcessTx(t, getTxHash(tx1), nil, true, false)

	// tx2 is already in the main pool. Ignore.
	verifyDoProcessTx(t, getTxHash(tx1), core.ErrDuplicateTxInPool, true, false)

	// mark t2's output as unspent
	utxoSet.AddUtxo(tx2, 0, chainHeight+1)

	// tx2(m) <- tx3(m)
	// tx3(m) is admitted into main pool since it spends from a valid UTXO, i.e., tx2
	verifyDoProcessTx(t, getTxHash(tx2), nil, true, false)

	// tx4 is a coinbase transaction.
	tx4, _ := chain.CreateCoinbaseTx(minerAddr, chainHeight)
	errTx4 := txpool.doProcessTx(tx4, chainHeight, utxoSet, false /* do not broadcast */)
	ensure.DeepEqual(t, errTx4, core.ErrCoinbaseTx)
	verifyTxInPool(t, tx4, false, false)

	// mark tx4's output as unspent
	utxoSet.AddUtxo(tx4, 0, chainHeight)

	//tx4(coinbase) <- tx5(m)
	//tx5(m) is admitted into main pool since it spends from a valid UTXO, i.e., tx4(coinbase)
	tx5 := verifyDoProcessTx(t, getTxHash(tx4), nil, true, false)

	// mark tx5's output as unspent
	utxoSet.AddUtxo(tx5, 0, chainHeight)

	//tx5(m) <- tx6(m)
	//tx6(m) is admitted into main pool since it spends from a valid UTXO, i.e., tx5
	tx6 := verifyDoProcessTx(t, getTxHash(tx5), nil, true, false)

	// mark tx6's output as unspent
	utxoSet.AddUtxo(tx6, 0, chainHeight)

	//tx6(m) <- tx7(m)
	//tx7(m )is admitted into main pool since it spends from a valid UTXO, i.e., tx6
	verifyDoProcessTx(t, getTxHash(tx6), nil, true, false)

	// get all transactions in the tx pool
	txs := txpool.GetAllTxs()

	// tx6(m) removal
	txpool.removeTx(tx6)

	// get all transactions in tx pool after tx6 removal
	txs1 := txpool.GetAllTxs()

	ensure.DeepEqual(t, len(txs)-1, len(txs1))

	//TODO: Test more than length. Test txs/txs1 contain exactly these transactions we put in.

}
