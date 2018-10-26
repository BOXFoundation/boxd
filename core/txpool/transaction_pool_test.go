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
	"github.com/BOXFoundation/boxd/script"
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

	privKey, pubKey, _ = crypto.NewKeyPair()
	addr, _            = types.NewAddressFromPubKey(pubKey)
	scriptAddr         = addr.ScriptAddress()
	scriptPubKey       = script.PayToPubKeyHashScript(scriptAddr)
	coinbaseTx, _      = chain.CreateCoinbaseTx(addr.ScriptAddress(), chainHeight)
)

// create a child tx spending parent tx's output
func createChildTx(parentTx *types.Transaction) *types.Transaction {
	outPoint := types.OutPoint{
		Hash:  *getTxHash(parentTx),
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

		// concatenate unlocking & locking scripts
		catScript := script.NewScript().AddScript(scriptSig).AddOpCode(script.OPCODESEPARATOR).AddScript(scriptPubKey)
		// test to ensure
		if err = catScript.Evaluate(tx, txInIdx); err != nil {
			return nil
		}
	}
	return tx
}

func getTxHash(tx *types.Transaction) *crypto.HashType {
	txHash, _ := tx.TxHash()
	return txHash
}

// return new child tx
func verifyDoProcessTx(t *testing.T, tx *types.Transaction, expectedErr error,
	isTransactionInPool, isOrphanInPool bool) *types.Transaction {

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
	tx0 := createChildTx(coinbaseTx)
	ensure.NotNil(t, tx0)
	verifyDoProcessTx(t, tx0, core.ErrOrphanTransaction, false, true)

	// a duplicate tx0, already exists in tx pool
	verifyDoProcessTx(t, tx0, core.ErrDuplicateTxInPool, false, true)

	// mark tx0's output as unspent
	utxoSet.AddUtxo(tx0, 0, chainHeight)

	// tx0(o) <- tx1(m)
	// tx1 is admitted into main pool since it spends from a valid UTXO, i.e., tx0
	tx1 := createChildTx(tx0)
	ensure.NotNil(t, tx1)
	verifyDoProcessTx(t, tx1, nil, true, false)

	utxoSet.AddUtxo(tx1, 0, chainHeight)

	// tx1A(m) <- tx2(m)
	tx2 := createChildTx(tx1)
	ensure.NotNil(t, tx2)
	verifyDoProcessTx(t, tx2, nil, true, false)

	// tx2 is already in the main pool. Ignore.
	verifyDoProcessTx(t, tx2, core.ErrDuplicateTxInPool, true, false)

	// mark t2's output as unspent
	utxoSet.AddUtxo(tx2, 0, chainHeight+1)

	// tx2(m) <- tx3(m)
	// tx3(m) is admitted into main pool since it spends from a valid UTXO, i.e., tx2
	tx3 := createChildTx(tx2)
	ensure.NotNil(t, tx3)
	verifyDoProcessTx(t, tx3, nil, true, false)

	// tx4 is a coinbase transaction
	tx4, _ := chain.CreateCoinbaseTx(addr.ScriptAddress(), chainHeight+1)
	errTx4 := txpool.doProcessTx(tx4, chainHeight, utxoSet, false /* do not broadcast */)
	ensure.DeepEqual(t, errTx4, core.ErrCoinbaseTx)
	verifyTxInPool(t, tx4, false, false)

	// mark tx4's output as unspent
	utxoSet.AddUtxo(tx4, 0, chainHeight)

	//tx4(coinbase) <- tx5(m)
	//tx5(m) is admitted into main pool since it spends from a valid UTXO, i.e., tx4(coinbase)
	tx5 := createChildTx(tx4)
	ensure.NotNil(t, tx5)
	verifyDoProcessTx(t, tx5, nil, true, false)

	// mark tx5's output as unspent
	utxoSet.AddUtxo(tx5, 0, chainHeight)

	//tx5(m) <- tx6(m)
	//tx6(m) is admitted into main pool since it spends from a valid UTXO, i.e., tx5
	tx6 := createChildTx(tx5)
	ensure.NotNil(t, tx6)
	verifyDoProcessTx(t, tx6, nil, true, false)

	// mark tx6's output as unspent
	utxoSet.AddUtxo(tx6, 0, chainHeight)

	//tx6(m) <- tx7(m)
	//tx7(m )is admitted into main pool since it spends from a valid UTXO, i.e., tx6
	tx7 := createChildTx(tx6)
	ensure.NotNil(t, tx7)
	verifyDoProcessTx(t, tx7, nil, true, false)

	// get all transactions in the tx pool
	txs := txpool.GetAllTxs()

	// tx6(m) removal
	txpool.removeTx(tx6)

	// get all transactions in tx pool after tx6 removal
	txs1 := txpool.GetAllTxs()

	// test for length
	ensure.DeepEqual(t, len(txs)-1, len(txs1))

	// test txs/txs1 contains exactly the removal transaction, i.e., tx6 removed
	for i := range txs {
		count := 0
		for j := range txs1 {
			if txs[i].Tx.Hash.IsEqual(txs1[j].Tx.Hash) {
				break
			}
			count++
		}
		if count == len(txs1) {
			ensure.DeepEqual(t, txs[i].Tx.Hash, tx6.Hash)
			break
		}
	}

	verifyDoProcessTx(t, tx7, core.ErrDuplicateTxInPool, true, false)

	// tx7(o) <- tx8(o)
	// after tx6 removed from main pool, tx7 moved to orphan pool, tx8 is an orphan transaction also
	tx8 := createChildTx(tx7)
	ensure.NotNil(t, tx8)
	verifyDoProcessTx(t, tx8, core.ErrMissingTxOut, false, false)

	txs2 := txpool.GetAllTxs()

	// add tx6 back to txpool
	txpool.addTx(tx6, chainHeight, 0)

	txs3 := txpool.GetAllTxs()

	ensure.DeepEqual(t, len(txs2)+1, len(txs3))

	// mark tx8's output as unspent
	utxoSet.AddUtxo(tx8, 0, chainHeight)

	// tx8(m) <- tx9(m)
	tx9 := createChildTx(tx8)
	ensure.NotNil(t, tx9)
	verifyDoProcessTx(t, tx9, nil, true, false)
}
