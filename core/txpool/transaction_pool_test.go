// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package txpool

import (
	"os"
	"testing"

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
	coinbaseTx, _      = chain.CreateCoinbaseTx(addr, chainHeight)
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

	// mark tx1's output as unspent
	utxoSet.AddUtxo(coinbaseTx, 0, chainHeight)

	// coinbase() <- tx1(m)
	// tx2 is admitted into main pool since it spends from a valid UTXO, i.e., coinbaseTx
	tx1 := createChildTx(coinbaseTx)
	ensure.NotNil(t, tx1)
	verifyDoProcessTx(t, tx1, nil, true, false)
}
