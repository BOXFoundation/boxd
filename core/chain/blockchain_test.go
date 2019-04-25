// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package chain

import (
	"context"
	"encoding/hex"
	"math/big"
	"testing"
	"time"

	"github.com/BOXFoundation/boxd/core"
	corepb "github.com/BOXFoundation/boxd/core/pb"
	"github.com/BOXFoundation/boxd/core/txlogic"
	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/crypto"
	"github.com/BOXFoundation/boxd/script"
	"github.com/BOXFoundation/boxd/storage"
	_ "github.com/BOXFoundation/boxd/storage/memdb"
	"github.com/BOXFoundation/boxd/vm"
	"github.com/facebookgo/ensure"
)

// test setup
var (
	privKeyMiner, pubKeyMiner, _ = crypto.NewKeyPair()
	privKey, pubKey, _           = crypto.NewKeyPair()
	minerAddr, _                 = types.NewAddressFromPubKey(pubKeyMiner)
	scriptPubKeyMiner            = script.PayToPubKeyHashScript(minerAddr.Hash())
	userAddr, _                  = types.NewAddressFromPubKey(pubKey)
	scriptPubKeyUser             = script.PayToPubKeyHashScript(userAddr.Hash())

	privKeySplitA, pubKeySplitA, _ = crypto.NewKeyPair()
	privKeySplitB, pubKeySplitB, _ = crypto.NewKeyPair()
	splitAddrA, _                  = types.NewAddressFromPubKey(pubKeySplitA)
	scriptPubKeySplitA             = script.PayToPubKeyHashScript(splitAddrA.Hash())
	splitAddrB, _                  = types.NewAddressFromPubKey(pubKeySplitB)
	scriptPubKeySplitB             = script.PayToPubKeyHashScript(splitAddrB.Hash())
	blockChain                     = NewTestBlockChain()
	timestamp                      = time.Now().Unix()

	addrs   = []string{splitAddrA.String(), splitAddrB.String()}
	weights = []uint64{5, 5}
)

func TestAppendInLoop2(t *testing.T) {
}

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
	timestamp++
	newBlock := types.NewBlock(parentBlock)

	coinbaseTx, _ := CreateCoinbaseTx(minerAddr.Hash(), parentBlock.Header.Height+1)
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

	err := blockChain.ProcessBlock(newBlock, core.DefaultMode /* not broadcast */, "peer1")

	ensure.DeepEqual(t, err, expectedErr)
	ensure.DeepEqual(t, blockChain.LongestChainHeight, expectedChainHeight)
	ensure.DeepEqual(t, getTailBlock(), expectedChainTail)
}

func verifyProcessBlockFromNet(t *testing.T, newBlock *types.Block, expectedErr error, expectedChainHeight uint32, expectedChainTail *types.Block) {

	err := blockChain.ProcessBlock(newBlock, core.DefaultMode /* not broadcast */, "abc")

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

	// >>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
	// extend main chain
	// b0 -> b1
	b1 := nextBlock(b0)
	verifyProcessBlock(t, b1, nil, 1, b1)
	balance := getBalance(minerAddr.String(), blockChain.db)
	ensure.DeepEqual(t, balance, uint64(50*core.DuPerBox))

	b1DoubleMint := nextBlock(b1)
	b1DoubleMint.Header.TimeStamp = b1.Header.TimeStamp
	verifyProcessBlock(t, b1DoubleMint, core.ErrRepeatedMintAtSameTime, 1, b1)
	balance = getBalance(minerAddr.String(), blockChain.db)
	ensure.DeepEqual(t, balance, uint64(50*core.DuPerBox))

	// >>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
	// double spend check
	b2ds := nextBlock(b1)
	// add a tx spending from previous block's coinbase
	b2ds.Txs = append(b2ds.Txs, createGeneralTx(b1.Txs[0], 0, 50*core.DuPerBox, userAddr.String(), privKeyMiner, pubKeyMiner))
	splitTx, splitAddr := createSplitTx(b1.Txs[0], 0)
	b2ds.Txs = append(b2ds.Txs, splitTx)
	b2ds.Header.TxsRoot = *CalcTxsHash(b2ds.Txs)
	verifyProcessBlock(t, b2ds, core.ErrDoubleSpendTx, 1, b1)
	balance = getBalance(minerAddr.String(), blockChain.db)
	ensure.DeepEqual(t, balance, uint64(50*core.DuPerBox))

	// >>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
	// extend main chain
	// b0 -> b1 -> b2
	// Tx: miner -> user: 50
	b2 := nextBlock(b1)
	// add a tx spending from previous block's coinbase
	b2.Txs = append(b2.Txs, createGeneralTx(b1.Txs[0], 0, 50*core.DuPerBox, userAddr.String(), privKeyMiner, pubKeyMiner))
	b2.Header.TxsRoot = *CalcTxsHash(b2.Txs)
	verifyProcessBlock(t, b2, nil, 2, b2)

	// miner balance: 100 - 50 = 50
	// user balance: 50
	balance = getBalance(minerAddr.String(), blockChain.db)
	ensure.DeepEqual(t, balance, uint64(50*core.DuPerBox))
	balance = getBalance(userAddr.String(), blockChain.db)
	ensure.DeepEqual(t, balance, uint64(50*core.DuPerBox))

	// >>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
	// extend main chain
	// b0 -> b1 -> b2 -> b3
	b3 := nextBlock(b2)
	b3.Header.TxsRoot = *CalcTxsHash(b3.Txs)
	verifyProcessBlock(t, b3, nil, 3, b3)
	balance = getBalance(minerAddr.String(), blockChain.db)
	ensure.DeepEqual(t, balance, uint64(100*core.DuPerBox))

	// >>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
	// extend side chain: fork from b1
	// b0 -> b1 -> b2 -> b3
	//		         \-> b3A
	b3A := nextBlock(b2)
	splitTx, splitAddr = createSplitTx(b2.Txs[0], 0)
	b3A.Txs = append(b3A.Txs, splitTx)
	b3A.Header.TxsRoot = *CalcTxsHash(b3A.Txs)
	verifyProcessBlock(t, b3A, core.ErrBlockInSideChain, 3, b3)

	// >>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
	// reorg: side chain grows longer than main chain
	// b0 -> b1 -> b2 -> b3
	//		         \-> b3A -> b4A
	// Tx: miner -> user: 50
	// Tx: miner -> split address: 50
	b4A := nextBlock(b3A)
	b4ATx := createGeneralTx(b3A.Txs[0], 0, 50*core.DuPerBox, splitAddr, privKeyMiner, pubKeyMiner)
	b4A.Txs = append(b4A.Txs, b4ATx)
	b4A.Header.TxsRoot = *CalcTxsHash(b4A.Txs)
	verifyProcessBlock(t, b4A, nil, 4, b4A)

	// check balance
	// miner balance: 4 * 50 - 50 - 50 = 100
	// splitA balance: 25  splitB balance: 25
	balance = getBalance(minerAddr.String(), blockChain.db)
	ensure.DeepEqual(t, balance, uint64(100*core.DuPerBox))
	blanceSplitA := getBalance(splitAddrA.String(), blockChain.db)
	ensure.DeepEqual(t, blanceSplitA, uint64(25*core.DuPerBox))
	blanceSplitB := getBalance(splitAddrB.String(), blockChain.db)
	ensure.DeepEqual(t, blanceSplitB, uint64(25*core.DuPerBox))

	//TODO: add insuffient balance check

	// >>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
	// Extend b3 fork twice to make first chain longer and force reorg
	// b0 -> b1 -> b2  -> b3  -> b4 -> b5
	// 		           -> b3A -> b4A
	// Tx: miner -> user: 50
	b4 := nextBlock(b3)
	verifyProcessBlock(t, b4, core.ErrBlockInSideChain, 4, b4A)
	b5 := nextBlock(b4)
	verifyProcessBlock(t, b5, nil, 5, b5)

	// check balance
	// miner balance: 5 * 50 - 50 = 200
	balance = getBalance(minerAddr.String(), blockChain.db)
	ensure.DeepEqual(t, balance, uint64(200*core.DuPerBox))
	blanceSplitA = getBalance(splitAddrA.String(), blockChain.db)
	ensure.DeepEqual(t, blanceSplitA, uint64(0))
	blanceSplitB = getBalance(splitAddrB.String(), blockChain.db)
	ensure.DeepEqual(t, blanceSplitB, uint64(0))

	// >>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
	// b0 -> b1 -> b2  -> b3  -> b4  -> b5
	// 		           -> b3A -> b4A -> b5A -> b6A
	// Tx: miner -> user: 50
	// Tx: miner -> split address: 50
	// Tx: miner -> user: 50
	// Tx: miner -> user: 50
	b5A := nextBlock(b4A)
	b5A.Txs = append(b5A.Txs, createGeneralTx(b4A.Txs[0], 0, 50*core.DuPerBox, userAddr.String(), privKeyMiner, pubKeyMiner))
	b5A.Header.TxsRoot = *CalcTxsHash(b5A.Txs)
	verifyProcessBlock(t, b5A, core.ErrBlockInSideChain, 5, b5)

	b6A := nextBlock(b5A)
	b6A.Txs = append(b6A.Txs, createGeneralTx(b3A.Txs[0], 0, 50*core.DuPerBox, userAddr.String(), privKeyMiner, pubKeyMiner))
	b6A.Header.TxsRoot = *CalcTxsHash(b6A.Txs)
	// reorg has happened
	verifyProcessBlock(t, b6A, core.ErrMissingTxOut, 5, b5A)

	b6A = nextBlock(b5A)
	b6A.Txs = append(b6A.Txs, createGeneralTx(b5A.Txs[0], 0, 50*core.DuPerBox, userAddr.String(), privKeyMiner, pubKeyMiner))
	b6A.Header.TxsRoot = *CalcTxsHash(b6A.Txs)
	verifyProcessBlock(t, b6A, nil, 6, b6A)

	// check balance
	// miner balance: 6 * 50 - 50 -50 -50 -50 = 100
	balance = getBalance(minerAddr.String(), blockChain.db)
	ensure.DeepEqual(t, balance, uint64(100*core.DuPerBox))
	blanceSplitA = getBalance(splitAddrA.String(), blockChain.db)
	ensure.DeepEqual(t, blanceSplitA, uint64(25*core.DuPerBox))
	blanceSplitB = getBalance(splitAddrB.String(), blockChain.db)
	ensure.DeepEqual(t, blanceSplitB, uint64(25*core.DuPerBox))

	// >>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
	// b0 -> b1 -> b2  -> b3  -> b4  -> b5
	// 		           -> b3A -> b4A -> b5A -> b6A -> b7A
	// Tx: miner -> user: 50
	// Tx: miner -> split address: 50
	// Tx: splitA -> user: 25
	// Tx: miner -> user: 50
	// Tx: miner -> user: 50

	b7A := nextBlock(b6A)
	b4ATxHash, _ := b4ATx.TxHash()
	buf, err := blockChain.db.Get(SplitTxHashKey(b4ATxHash))
	if err != nil || buf == nil {
		logger.Errorf("Failed to get split tx. Err: %v", err)
	}
	b4ASplitTx := new(types.Transaction)
	if err := b4ASplitTx.Unmarshal(buf); err != nil {
		logger.Errorf("Failed to Unmarshal split tx. Err: %v", err)
	}
	logger.Infof("b4ASplitTx: %v", b4ASplitTx)
	b7ATx := createGeneralTx(b4ASplitTx, 0, 25*core.DuPerBox, userAddr.String(), privKeySplitA, pubKeySplitA)
	b7A.Txs = append(b7A.Txs, b7ATx)
	b7A.Header.TxsRoot = *CalcTxsHash(b7A.Txs)
	verifyProcessBlock(t, b7A, nil, 7, b7A)

	// check balance
	// miner balance: 7 * 50 - 50 -50 -50 -50 = 150
	// splitAddrA balance: 0
	// splitAddrB balance: 25
	// user balance: 50 + 50 + 50 + 25 = 175
	balance = getBalance(minerAddr.String(), blockChain.db)
	ensure.DeepEqual(t, balance, uint64(150*core.DuPerBox))
	blanceSplitA = getBalance(splitAddrA.String(), blockChain.db)
	ensure.DeepEqual(t, blanceSplitA, uint64(0))
	blanceSplitB = getBalance(splitAddrB.String(), blockChain.db)
	ensure.DeepEqual(t, blanceSplitB, uint64(25*core.DuPerBox))
	balance = getBalance(userAddr.String(), blockChain.db)
	ensure.DeepEqual(t, balance, uint64(175*core.DuPerBox))

	// >>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
	// force reorg split tx
	// b0 -> b1 -> b2  -> b3  -> b4  -> b5
	// 		           -> b3A -> b4A -> b5A -> b6A -> b7A
	//                                             -> b7B -> b8B
	// Tx: miner -> user: 50
	// Tx: miner -> split address: 50
	// Tx: splitA -> user: 25
	// Tx: miner -> user: 50
	// Tx: miner -> user: 50
	b7B := nextBlock(b6A)
	verifyProcessBlock(t, b7B, core.ErrBlockInSideChain, 7, b7A)
	b8B := nextBlock(b7B)
	verifyProcessBlock(t, b8B, nil, 8, b8B)

	// check balance
	// splitAddrA balance: 25
	// splitAddrB balance: 25
	// user balance: 175 25 = 150
	blanceSplitA = getBalance(splitAddrA.String(), blockChain.db)
	ensure.DeepEqual(t, blanceSplitA, uint64(25*core.DuPerBox))
	blanceSplitB = getBalance(splitAddrB.String(), blockChain.db)
	ensure.DeepEqual(t, blanceSplitB, uint64(25*core.DuPerBox))
	balance = getBalance(userAddr.String(), blockChain.db)
	ensure.DeepEqual(t, balance, uint64(150*core.DuPerBox))

	// >>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
	// force reorg split tx
	// b0 -> b1 -> b2  -> b3  -> b4  -> b5  -> b6  -> b7  -> b8  -> b9
	// 		           -> b3A -> b4A -> b5A -> b6A -> b7A
	//                                             -> b7B -> b8B
	// Tx: miner -> user: 50
	// Tx: miner -> split address: 50
	// Tx: splitA -> user: 25
	b6 := nextBlock(b5)
	verifyProcessBlock(t, b6, core.ErrBlockInSideChain, 8, b8B)
	b7 := nextBlock(b6)
	verifyProcessBlock(t, b7, core.ErrBlockInSideChain, 8, b8B)
	b8 := nextBlock(b7)
	verifyProcessBlock(t, b8, core.ErrBlockInSideChain, 8, b8B)
	b9 := nextBlock(b8)
	verifyProcessBlock(t, b9, nil, 9, b9)

	// check balance
	// miner balance: 9 * 50 - 50 = 400
	// splitAddrA balance: 0
	// splitAddrB balance: 0
	// user balance: 50
	balance = getBalance(minerAddr.String(), blockChain.db)
	ensure.DeepEqual(t, balance, uint64(400*core.DuPerBox))
	blanceSplitA = getBalance(splitAddrA.String(), blockChain.db)
	ensure.DeepEqual(t, blanceSplitA, uint64(0))
	blanceSplitB = getBalance(splitAddrB.String(), blockChain.db)
	ensure.DeepEqual(t, blanceSplitB, uint64(0))
	balance = getBalance(userAddr.String(), blockChain.db)
	ensure.DeepEqual(t, balance, uint64(50*core.DuPerBox))
}

func TestBlockChain_WriteDelTxIndex(t *testing.T) {
	ensure.NotNil(t, blockChain)

	b0 := getTailBlock()

	b1 := nextBlock(b0)
	blockChain.db.EnableBatch()
	ensure.Nil(t, blockChain.StoreBlockWithStateInBatch(b1, nil, blockChain.db))

	txhash, _ := b1.Txs[0].TxHash()

	ensure.Nil(t, blockChain.WriteTxIndex(b1, map[crypto.HashType]*types.Transaction{}, blockChain.db))
	blockChain.db.Flush()

	_, tx, err := blockChain.LoadBlockInfoByTxHash(*txhash)
	ensure.Nil(t, err)
	ensure.DeepEqual(t, b1.Txs[0], tx)

	ensure.Nil(t, blockChain.DelTxIndex(b1, map[crypto.HashType]*types.Transaction{}, blockChain.db))
	blockChain.db.Flush()
	_, _, err = blockChain.LoadBlockInfoByTxHash(*txhash)
	ensure.NotNil(t, err)
}

func createSplitTx(parentTx *types.Transaction, index uint32) (*types.Transaction, string) {

	vIn := makeVin(parentTx, index)
	txOut := &corepb.TxOut{
		Value:        50 * core.DuPerBox,
		ScriptPubKey: *scriptPubKeyMiner,
	}
	splitAddrOut := txlogic.MakeSplitAddrVout(addrs, weights)
	tx := &types.Transaction{
		Vin:  vIn,
		Vout: []*corepb.TxOut{txOut, splitAddrOut},
	}

	addr, err := txlogic.MakeSplitAddr(addrs, weights)
	if err != nil {
		logger.Errorf("failed to make split addr. Err: %+v", err)
	}

	if err := signTx(tx, privKeyMiner, pubKeyMiner); err != nil {
		logger.Errorf("Failed to sign tx. Err: %v", err)
		return nil, ""
	}
	logger.Infof("create a split tx. addr: %s", addr)
	return tx, addr
}

func createGeneralTx(parentTx *types.Transaction, index uint32, value uint64,
	address string, privKey *crypto.PrivateKey, pubKey *crypto.PublicKey) *types.Transaction {
	vIn := makeVin(parentTx, index)
	txOut := txlogic.MakeVout(address, value)
	vOut := []*corepb.TxOut{txOut}
	tx := &types.Transaction{
		Vin:  vIn,
		Vout: vOut,
	}
	if err := signTx(tx, privKey, pubKey); err != nil {
		logger.Errorf("Failed to sign tx. Err: %v", err)
		return nil
	}
	return tx
}

func signTx(tx *types.Transaction, privKey *crypto.PrivateKey, pubKey *crypto.PublicKey) error {

	addr, _ := types.NewAddressFromPubKey(pubKey)
	scriptPubKey := script.PayToPubKeyHashScript(addr.Hash())
	// sign it
	for txInIdx, txIn := range tx.Vin {
		sigHash, err := script.CalcTxHashForSig(*scriptPubKey, tx, txInIdx)
		if err != nil {
			return err
		}
		sig, err := crypto.Sign(privKey, sigHash)
		if err != nil {
			return err
		}
		scriptSig := script.SignatureScript(sig, pubKey.Serialize())
		txIn.ScriptSig = *scriptSig

		// test to ensure
		if err = script.Validate(scriptSig, scriptPubKey, tx, txInIdx); err != nil {
			logger.Errorf("failed to validate tx. Err: %v", err)
			return err
		}
	}
	return nil
}

func makeVin(tx *types.Transaction, index uint32) []*types.TxIn {
	hash, _ := tx.TxHash()
	outPoint := types.OutPoint{
		Hash:  *hash,
		Index: index,
	}
	txIn := &types.TxIn{
		PrevOutPoint: outPoint,
		ScriptSig:    []byte{},
		Sequence:     0,
	}
	vIn := []*types.TxIn{
		txIn,
	}
	return vIn
}

func getTxHash(tx *types.Transaction) *crypto.HashType {
	txHash, _ := tx.TxHash()
	return txHash
}

func getBalance(address string, db storage.Table) uint64 {
	utxoKey := AddrAllUtxoKey(address)
	keys := db.KeysWithPrefix(utxoKey)
	values, err := db.MultiGet(keys...)
	if err != nil {
		logger.Fatalf("failed to multget from db. Err: %+v", err)
	}
	var blances uint64
	for i, value := range values {
		var utxoWrap *types.UtxoWrap
		if utxoWrap, err = DeserializeUtxoWrap(value); err != nil {
			logger.Errorf("Deserialize error %s, key = %s, body = %v",
				err, string(keys[i]), string(value))
			continue
		}
		if utxoWrap == nil {
			logger.Warnf("invalid utxo in db, key: %s, value: %+v", keys[i], utxoWrap)
			continue
		}
		blances += utxoWrap.Value()
	}
	return blances
}

type testDBReader struct{}

func (r *testDBReader) Get(key []byte) ([]byte, error) {
	addr := "b1ndoQmEd83y4Fza5PzbUQDYpT3mV772J5o"
	address, _ := types.NewAddress(addr)
	addrPkh, _ := types.NewAddressPubKeyHash(address.Hash())
	addrScript := *script.PayToPubKeyHashScript(addrPkh.Hash())
	utxoWrap := types.NewUtxoWrap(1000, addrScript, 0)
	return SerializeUtxoWrap(utxoWrap)
}

func (r *testDBReader) MultiGet(key ...[]byte) ([][]byte, error) { return nil, nil }

func (r *testDBReader) Has(key []byte) (bool, error) { return false, nil }

func (r *testDBReader) Keys() [][]byte { return nil }

func (r *testDBReader) IterKeys(ctx context.Context) <-chan []byte { return nil }

func (r *testDBReader) KeysWithPrefix(prefix []byte) [][]byte { return nil }

func (r *testDBReader) IterKeysWithPrefix(ctx context.Context, prefix []byte) <-chan []byte {
	return nil
}

const (
	testBlockSubsidy = 50 * uint64(core.DuPerBox)

	testExtractPrevHash = "c0e96e998eb01eea5d5acdaeb80acd943477e6119dcd82a419089331229c7453"
	// contract Temp {
	//     function () payable {}
	// }
	testVMScriptCode = "6060604052346000575b60398060166000396000f30060606040525b600b5b5b565b0000a165627a7a723058209cedb722bf57a30e3eb00eeefc392103ea791a2001deed29f5c3809ff10eb1dd0029"

	/*
		pragma solidity ^0.5.1;
		contract Faucet {
				// Give out box to anyone who asks
		    function withdraw(uint withdraw_amount) public {
						// Limit withdrawal amount
		        require(withdraw_amount <= 10000);
		        // Send the amount to the address that requested it
		        msg.sender.transfer(withdraw_amount);
		    }
		    // Accept any incoming amount
		    function () external payable  {}
		}
	*/
	testVMCreationCode = "608060405234801561001057600080fd5b5060f78061001f6000396000f3fe60806" +
		"04052600436106039576000357c010000000000000000000000000000000000000000000000000" +
		"0000000900480632e1a7d4d14603b575b005b348015604657600080fd5b5060706004803603602" +
		"0811015605b57600080fd5b81019080803590602001909291905050506072565b005b612710811" +
		"1151515608257600080fd5b3373ffffffffffffffffffffffffffffffffffffffff166108fc829" +
		"081150290604051600060405180830381858888f1935050505015801560c7573d6000803e3d600" +
		"0fd5b505056fea165627a7a723058202f9f6d2a7c03458fe70c89e92de0447e8e0d48be3411c23" +
		"0a56f9057cc10c9ad0029"
	testVMCallCode = "07546172"
)

func _TestExtractBoxTx(t *testing.T) {
	var tests = []struct {
		value        uint64
		addrStr      string
		code         string
		price, limit uint64
		version      int32
		err          error
	}{
		{100, "b1YMx5kufN2qELzKaoaBWzks2MZknYqqPnh", testVMScriptCode, 100, 20000, 0, nil},
		{0, "", testVMScriptCode, 100, 20000, 0, nil},
	}
	for _, tc := range tests {
		var addr types.Address
		if tc.addrStr != "" {
			addr, _ = types.NewAddress(tc.addrStr)
		}
		code, _ := hex.DecodeString(tc.code)
		cs, err := script.MakeContractScriptPubkey(addr, code, tc.price, tc.limit, tc.version)
		if err != nil {
			t.Fatal(err)
		}
		hash := new(crypto.HashType)
		hashBytes, _ := hex.DecodeString(testExtractPrevHash)
		hash.SetBytes(hashBytes)
		prevOp := types.NewOutPoint(hash, 0)
		txin := types.NewTxIn(prevOp, nil, 0)
		txout := types.NewTxOut(tc.value, *cs)
		tx := types.NewTx(0, 4455, 100).AppendVin(txin).AppendVout(txout)
		btx, err := blockChain.ExtractVMTransactions(tx)
		if err != nil {
			t.Fatal(err)
		}
		// check
		sender, _ := types.NewAddress("b1ndoQmEd83y4Fza5PzbUQDYpT3mV772J5o")
		hashWith, _ := tx.TxHash()
		if *btx.OriginTxHash() != *hashWith ||
			*btx.From() != *sender.Hash160() ||
			(btx.To() != nil && *btx.To() != *addr.Hash160()) ||
			btx.Value().Cmp(big.NewInt(int64(tc.value))) != 0 ||
			btx.GasPrice().Cmp(big.NewInt(int64(tc.price))) != 0 ||
			btx.Gas() != tc.limit || btx.Version() != tc.version {
			t.Fatalf("want: %+v, got BoxTransaction: %+v", tc, btx)
		}
	}
}

// generate a child block with contract tx
func nextBlockWithTxs(parent *types.Block, txs ...*types.Transaction) *types.Block {
	timestamp++
	newBlock := types.NewBlock(parent)

	coinbaseTx, _ := CreateCoinbaseTx(minerAddr.Hash(), parent.Header.Height+1)
	// use time to ensure we create a different/unique block each time
	coinbaseTx.Vin[0].Sequence = uint32(time.Now().UnixNano())
	newBlock.Txs = append(append(newBlock.Txs, coinbaseTx), txs...)
	newBlock.Header.TxsRoot = *CalcTxsHash(newBlock.Txs)
	newBlock.Header.TimeStamp = timestamp
	return newBlock
}

var (
	userBalance, minerBalance, contractBalance uint64
)

type testContractParam struct {
	gasUsed, vmValue, gasPrice, gasLimit, contractBalance uint64

	contractAddr *types.AddressContract
}

func genTestChain(t *testing.T) *types.Block {
	b0 := getTailBlock()
	// >>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
	// extend main chain
	// b0 -> b1
	b1 := nextBlock(b0)
	verifyProcessBlock(t, b1, nil, 1, b1)
	balance := getBalance(minerAddr.String(), blockChain.db)
	stateBalance, _ := blockChain.GetBalance(minerAddr)
	ensure.DeepEqual(t, balance, stateBalance)
	ensure.DeepEqual(t, balance, testBlockSubsidy)
	t.Logf("b0 -> b1 passed, now tail height: %d", blockChain.LongestChainHeight)

	// >>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
	// extend main chain
	// b1 -> b2
	// transfer some box to userAddr
	userBalance = uint64(6000000)
	prevHash, _ := b1.Txs[0].TxHash()
	tx := types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(prevHash, 0), 0)).
		AppendVout(txlogic.MakeVout(userAddr.String(), userBalance)).
		AppendVout(txlogic.MakeVout(minerAddr.String(), testBlockSubsidy-userBalance))
	err := signTx(tx, privKeyMiner, pubKeyMiner)
	ensure.DeepEqual(t, err, nil)

	b2 := nextBlockWithTxs(b1, tx)
	verifyProcessBlock(t, b2, nil, 2, b2)
	// check balance
	// for userAddr
	balance = getBalance(userAddr.String(), blockChain.db)
	stateBalance, _ = blockChain.GetBalance(userAddr)
	ensure.DeepEqual(t, balance, stateBalance)
	ensure.DeepEqual(t, balance, userBalance)
	// for miner
	balance = getBalance(minerAddr.String(), blockChain.db)
	stateBalance, _ = blockChain.GetBalance(minerAddr)
	ensure.DeepEqual(t, balance, stateBalance)
	ensure.DeepEqual(t, balance, 2*testBlockSubsidy-userBalance)
	minerBalance = balance
	t.Logf("b1 -> b2 passed, now tail height: %d", blockChain.LongestChainHeight)
	return b2
}

func contractBlockHandle(
	t *testing.T, vmTx *types.Transaction, parent *types.Block,
	param *testContractParam, err error,
) *types.Block {

	block := nextBlockWithTxs(parent, vmTx)
	gasCost := param.gasUsed * param.gasPrice
	refundValue := uint64(0)
	if err == nil {
		refundValue = param.gasPrice*param.gasLimit - gasCost
		block.InternalTxs = append(block.InternalTxs,
			createGasRefundUtxoTx(userAddr.Hash160(), refundValue))
		block.Header.InternalTxsRoot.SetBytes(CalcTxsHash(block.InternalTxs)[:])
	}
	block.Header.GasUsed = param.gasUsed
	block.Txs[0].Vout[0].Value += gasCost
	tailBlock := block
	expectUserBalance := userBalance - param.vmValue - gasCost
	//t.Logf("expectUserBalance: %d, userBalance: %d, vmValue: %d, gasCost: %d",
	//	expectUserBalance, userBalance, param.vmValue, gasCost)
	expectMinerBalance := minerBalance + testBlockSubsidy + gasCost
	if err != nil && err == vm.ErrInsufficientBalance {
		tailBlock = parent
		expectUserBalance, expectMinerBalance = userBalance, minerBalance
	}
	height := tailBlock.Header.Height
	verifyProcessBlockFromNet(t, block, nil, height, tailBlock)
	// check balance
	// for userAddr
	balance := getBalance(userAddr.String(), blockChain.db)
	stateBalance, _ := blockChain.GetBalance(userAddr)
	t.Logf("userBalance: %d in utxos, %d in state", balance, stateBalance)
	ensure.DeepEqual(t, balance, stateBalance)
	ensure.DeepEqual(t, balance, expectUserBalance)
	userBalance = balance
	// for miner
	balance = getBalance(minerAddr.String(), blockChain.db)
	stateBalance, _ = blockChain.GetBalance(minerAddr)
	ensure.DeepEqual(t, balance, stateBalance)
	ensure.DeepEqual(t, balance, expectMinerBalance)
	minerBalance = balance
	// for contract address
	balance = getBalance(param.contractAddr.String(), blockChain.db)
	stateBalance, _ = blockChain.GetBalance(param.contractAddr)
	ensure.DeepEqual(t, balance, stateBalance)
	ensure.DeepEqual(t, balance, param.contractBalance)
	contractBalance = balance

	return block
}

func TestBlockProcessingWithContractTX(t *testing.T) {
	ensure.NotNil(t, blockChain)
	ensure.True(t, blockChain.LongestChainHeight == 0)

	// contract blocks test
	b2 := genTestChain(t)

	// >>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
	// extend main chain
	// b2 -> b3
	// make creation contract tx
	vmValue, gasPrice, gasLimit := uint64(0), uint64(10), uint64(200000)
	vmParam := &testContractParam{
		// gasUsed, vmValue, gasPrice, gasLimit
		56252, vmValue, gasPrice, gasLimit, vmValue, nil,
	}

	byteCode, _ := hex.DecodeString(testVMCreationCode)
	contractVout, err := txlogic.MakeContractCreationVout(vmValue, gasLimit, gasPrice, byteCode)
	ensure.Nil(t, err)
	prevHash, _ := b2.Txs[1].TxHash()
	changeValue2 := userBalance - vmValue - gasPrice*gasLimit
	vmTx := types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(prevHash, 0), 0)).
		AppendVout(contractVout).
		AppendVout(txlogic.MakeVout(userAddr.String(), changeValue2))
	signTx(vmTx, privKey, pubKey)
	stateDB := blockChain.stateDBCache[blockChain.LongestChainHeight]
	nonce := stateDB.GetNonce(*userAddr.Hash160())
	vmParam.contractAddr, _ = types.MakeContractAddress(userAddr, nonce)
	t.Logf("contract address: %s", vmParam.contractAddr)
	b3 := contractBlockHandle(t, vmTx, b2, vmParam, nil)

	refundValue := vmParam.gasPrice * (vmParam.gasLimit - vmParam.gasUsed)
	t.Logf("b2 -> b3 passed, now tail height: %d", blockChain.LongestChainHeight)

	// >>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
	// extend main chain
	// b3 -> b4
	// make call contract tx
	vmValue, gasPrice, gasLimit = uint64(666), uint64(6), uint64(3000)
	vmParam = &testContractParam{
		// gasUsed, vmValue, gasPrice, gasLimit
		2203, vmValue, gasPrice, gasLimit, vmValue, vmParam.contractAddr,
	}
	byteCode, _ = hex.DecodeString(testVMCallCode)
	contractVout, err = txlogic.MakeContractCallVout(vmParam.contractAddr.String(),
		vmValue, gasLimit, gasPrice, byteCode)
	ensure.Nil(t, err)
	// use internal tx vout
	prevHash, _ = b3.InternalTxs[0].TxHash()
	changeValue3 := refundValue - vmValue - gasPrice*gasLimit
	vmTx = types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(prevHash, 0), 0)).
		AppendVout(contractVout).
		AppendVout(txlogic.MakeVout(userAddr.String(), changeValue3))
	signTx(vmTx, privKey, pubKey)
	b4 := contractBlockHandle(t, vmTx, b3, vmParam, nil)
	t.Logf("b3 -> b4 passed, now tail height: %d", blockChain.LongestChainHeight)

	// >>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
	// extend main chain
	// b4 -> b5
	// make creation contract tx with insufficient gas
	vmValue, gasPrice, gasLimit = uint64(0), uint64(10), uint64(8000)
	vmParam = &testContractParam{
		// gasUsed, vmValue, gasPrice, gasLimit
		8000, vmValue, gasPrice, gasLimit, 666, vmParam.contractAddr,
	}
	byteCode, _ = hex.DecodeString(testVMCreationCode)
	contractVout, err = txlogic.MakeContractCreationVout(vmValue, gasLimit, gasPrice, byteCode)
	ensure.Nil(t, err)
	prevHash, _ = b3.Txs[1].TxHash()
	changeValue4 := changeValue2 - vmValue - gasPrice*gasLimit
	vmTx = types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(prevHash, 1), 0)).
		AppendVout(contractVout).
		AppendVout(txlogic.MakeVout(userAddr.String(), changeValue4))
	signTx(vmTx, privKey, pubKey)
	contractBlockHandle(t, vmTx, b4, vmParam, core.ErrInvalidInternalTxs)

	t.Logf("b4 -> b5 passed, now tail height: %d", blockChain.LongestChainHeight)
}

func TestFaucetContractBlock(t *testing.T) {
	ensure.NotNil(t, blockChain)
	ensure.True(t, blockChain.LongestChainHeight == 0)

	// contract blocks test
	b2 := genTestChain(t)

	// >>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
	// extend main chain
	// b2 -> b3
	// make creation contract tx
	vmValue, gasPrice, gasLimit := uint64(0), uint64(10), uint64(200000)
	vmParam := &testContractParam{
		// gasUsed, vmValue, gasPrice, gasLimit
		56252, vmValue, gasPrice, gasLimit, vmValue, nil,
	}
	byteCode, _ := hex.DecodeString(testVMCreationCode)
	contractVout, err := txlogic.MakeContractCreationVout(vmValue, gasLimit, gasPrice, byteCode)
	ensure.Nil(t, err)
	prevHash, _ := b2.Txs[1].TxHash()
	changeValue2 := userBalance - vmValue - gasPrice*gasLimit
	vmTx := types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(prevHash, 0), 0)).
		AppendVout(contractVout).
		AppendVout(txlogic.MakeVout(userAddr.String(), changeValue2))
	signTx(vmTx, privKey, pubKey)
	stateDB := blockChain.stateDBCache[blockChain.LongestChainHeight]
	nonce := stateDB.GetNonce(*userAddr.Hash160())
	vmParam.contractAddr, _ = types.MakeContractAddress(userAddr, nonce)
	t.Logf("contract address: %s", vmParam.contractAddr)
	contractBlockHandle(t, vmTx, b2, vmParam, nil)

	t.Logf("b2 -> b3 passed, now tail height: %d", blockChain.LongestChainHeight)

}
