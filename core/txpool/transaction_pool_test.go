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

	// for Txs with multiple children
	txOutIdx0 = uint32(0)
	txOutIdx1 = uint32(1)
	txOutIdx2 = uint32(2)

	value1 = int64(1)
	value2 = int64(2)
	value3 = int64(3)

	privKey, pubKey, _ = crypto.NewKeyPair()
	addr, _            = types.NewAddressFromPubKey(pubKey)
	scriptAddr         = addr.ScriptAddress()
	scriptPubKey       = script.PayToPubKeyHashScript(scriptAddr)
	coinbaseTx, _      = chain.CreateCoinbaseTx(addr, chainHeight)
)

// create a child tx spending parent tx's output
func createChildTx(parentTx *types.Transaction, txOutIdx uint32, values ...int64) *types.Transaction {
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
	vOut := []*corepb.TxOut{}
	for _, val := range values {
		txOut := &corepb.TxOut{
			Value:        val,
			ScriptPubKey: *scriptPubKey,
		}
		vOut = append(vOut, txOut)
	}
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

// create a child tx spending multiple parents tx's output
func createChildTxWithMultipleParentTx(txOutIdx uint32, value int64, parentTxs ...*types.Transaction) *types.Transaction {
	vIn := []*types.TxIn{}
	for _, parentTx := range parentTxs {
		outPoint := types.OutPoint{
			Hash:  *getTxHash(parentTx),
			Index: txOutIdx,
		}
		txIn := &types.TxIn{
			PrevOutPoint: outPoint,
			ScriptSig:    []byte{},
			Sequence:     0,
		}
		vIn = append(vIn, txIn)
	}
	txOut := &corepb.TxOut{
		Value:        value,
		ScriptPubKey: *scriptPubKey,
	}
	vOut := []*corepb.TxOut{txOut}
	tx := &types.Transaction{
		Version: 1,
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
	tx0 := createChildTx(coinbaseTx, txOutIdx0, value3)
	ensure.NotNil(t, tx0)
	verifyDoProcessTx(t, tx0, core.ErrOrphanTransaction, false, true)

	// a duplicate tx0, already exists in tx pool
	verifyDoProcessTx(t, tx0, core.ErrDuplicateTxInPool, false, true)

	// mark tx0's output as unspent
	utxoSet.AddUtxo(tx0, txOutIdx0, chainHeight)

	// tx0(o) <- tx1(m)
	// tx1A is admitted into main pool since it spends from a valid UTXO, i.e., tx0
	tx1A := createChildTx(tx0, txOutIdx0, value2)
	ensure.NotNil(t, tx1A)
	verifyDoProcessTx(t, tx1A, nil, true, false)

	// tx1A is already in the main pool. Ignore.
	verifyDoProcessTx(t, tx1A, core.ErrDuplicateTxInPool, true, false)

	// mark tx1A's output as unspent
	utxoSet.AddUtxo(tx1A, txOutIdx0, chainHeight)

	// tx1B(o)
	// tx1B is rejected since orphan transaction cannot be admitted into the pool
	tx1B := createChildTx(coinbaseTx, txOutIdx0, value1)
	ensure.NotNil(t, tx1B)
	verifyDoProcessTx(t, tx1B, core.ErrOrphanTransaction, false, true)

	// mark tx1B's output as unspent
	utxoSet.AddUtxo(tx1B, txOutIdx0, chainHeight)

	// tx2 is a child transaction with multiple parent txs, i.e., tx1A, tx1B
	tx2 := createChildTxWithMultipleParentTx(txOutIdx0, value3, tx1A, tx1B)
	ensure.NotNil(t, tx2)
	verifyDoProcessTx(t, tx2, nil, true, false)

	// mark tx2's output as unspent
	utxoSet.AddUtxo(tx2, txOutIdx0, chainHeight)

	// tx2(m) <-  tx3(m)
	tx3 := createChildTx(tx2, txOutIdx0, value1, value1, value1)
	ensure.NotNil(t, tx3)
	verifyDoProcessTx(t, tx3, nil, true, false)

	// mark tx3's 1st output as unspent
	utxoSet.AddUtxo(tx3, txOutIdx0, chainHeight)

	// tx3 has multiple children tx, i.e., tx4A, tx4B
	// tx3(m) <- tx4A(m)
	// tx4A(m) is admitted into main pool since it spends from a valid UTXO, i.e., tx3
	tx4A := createChildTx(tx3, txOutIdx0, value1)
	ensure.NotNil(t, tx4A)
	verifyDoProcessTx(t, tx4A, nil, true, false)

	// mark tx3's 2st output as unspent
	utxoSet.AddUtxo(tx3, txOutIdx1, chainHeight)

	// tx3(m) <- tx4B(m)
	// tx4B(m) is admitted into main pool since it spends from a valid UTXO, i.e., tx3
	tx4B := createChildTx(tx3, txOutIdx1, value1)
	ensure.NotNil(t, tx4B)
	verifyDoProcessTx(t, tx4B, nil, true, false)

	// mark tx3's 3rd output as unspent
	utxoSet.AddUtxo(tx3, txOutIdx2, chainHeight)

	// tx3(m) <- tx4C()
	// tx4C is attempting to spend more value than the sum of all of its inputs, i.e., tx3
	// Ignore.
	tx4C := createChildTx(tx3, txOutIdx2, value2)
	ensure.NotNil(t, tx4C)
	verifyDoProcessTx(t, tx4C, core.ErrSpendTooHigh, false, false)

	// tx5 is a coinbase transaction
	tx5, _ := chain.CreateCoinbaseTx(addr, chainHeight+1)
	errTx5 := txpool.doProcessTx(tx5, chainHeight, utxoSet, false /* do not broadcast */)
	ensure.DeepEqual(t, errTx5, core.ErrCoinbaseTx)
	verifyTxInPool(t, tx5, false, false)

	// mark tx5's output as unspent
	utxoSet.AddUtxo(tx5, txOutIdx0, chainHeight)

	//tx5(coinbase) <- tx6(m)
	//tx6(m) is admitted into main pool since it spends from a valid UTXO, i.e., tx5(coinbase)
	tx6 := createChildTx(tx5, txOutIdx0, value1)
	ensure.NotNil(t, tx6)
	verifyDoProcessTx(t, tx6, nil, true, false)

	// mark tx6's output as unspent
	utxoSet.AddUtxo(tx6, txOutIdx0, chainHeight)

	//tx7(o) is not in main pool
	tx7 := createChildTx(tx6, txOutIdx0, value1)
	ensure.NotNil(t, tx7)

	//tx7(o) <- tx8(o)
	//tx8 is not in main pool, so tx8 is orphan tx
	tx8 := createChildTx(tx7, txOutIdx0, value1)
	ensure.NotNil(t, tx8)
	verifyDoProcessTx(t, tx8, core.ErrOrphanTransaction, false, true)

	// tx8(o) <- tx9(o)
	tx9 := createChildTx(tx8, txOutIdx0, value1)
	ensure.NotNil(t, tx9)
	verifyDoProcessTx(t, tx9, core.ErrOrphanTransaction, false, true)

	// tx9(o) <- tx10(o)
	tx10 := createChildTx(tx9, txOutIdx0, value1)
	ensure.NotNil(t, tx10)
	verifyDoProcessTx(t, tx10, core.ErrOrphanTransaction, false, true)

	// add tx7 back to txpool
	txpool.addTx(tx7, chainHeight, 0)

	// mark tx7's output as unspent
	utxoSet.AddUtxo(tx7, txOutIdx0, chainHeight)

	// mark tx8's output as unspent
	utxoSet.AddUtxo(tx8, txOutIdx0, chainHeight)

	// mark tx9's output as unspent
	utxoSet.AddUtxo(tx9, txOutIdx0, chainHeight)

	// mark tx10's output as unspent
	utxoSet.AddUtxo(tx10, txOutIdx0, chainHeight)

	// tx10 <- tx11(m)
	tx11 := createChildTx(tx10, txOutIdx0, value1)
	ensure.NotNil(t, tx11)
	verifyDoProcessTx(t, tx11, nil, true, false)

	// get all transactions in the tx pool
	txs := txpool.GetAllTxs()

	// tx7(m) removal
	txpool.removeTx(tx7)

	// get all transactions in tx pool after tx7 removal
	txs1 := txpool.GetAllTxs()

	// test for length
	ensure.DeepEqual(t, len(txs)-1, len(txs1))

	// test txs/txs1 contains exactly the removal transaction, i.e., tx7 removed
	for i := range txs {
		count := 0
		for j := range txs1 {
			hash1, _ := txs[i].Tx.TxHash()
			hash2, _ := txs1[j].Tx.TxHash()
			if hash1.IsEqual(hash2) {
				break
			}
			count++
		}

		if count == len(txs1) {
			hash1, _ := txs[i].Tx.TxHash()
			hash7, _ := tx7.TxHash()
			ensure.DeepEqual(t, hash1, hash7)
			break
		}
	}

	// tx8(o)
	// after tx7 removed from main pool, tx8 moved to orphan pool, so are tx9(o), tx10(o), tx11(o)
	verifyDoProcessTx(t, tx8, core.ErrDuplicateTxInPool, false, true)
	verifyDoProcessTx(t, tx9, core.ErrDuplicateTxInPool, false, true)
	verifyDoProcessTx(t, tx10, core.ErrDuplicateTxInPool, false, true)

	// tx11(o) <- tx12(o)
	// tx12 is an orphan transaction also
	tx12 := createChildTx(tx11, txOutIdx0, value1)
	ensure.NotNil(t, tx12)
	verifyDoProcessTx(t, tx12, core.ErrMissingTxOut, false, false)
}
