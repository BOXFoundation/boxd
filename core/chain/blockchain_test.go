// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package chain

import (
	"encoding/hex"
	"testing"
	"time"

	"github.com/BOXFoundation/boxd/core"
	corepb "github.com/BOXFoundation/boxd/core/pb"
	"github.com/BOXFoundation/boxd/core/txlogic"
	"github.com/BOXFoundation/boxd/core/types"
	state "github.com/BOXFoundation/boxd/core/worldstate"
	"github.com/BOXFoundation/boxd/crypto"
	"github.com/BOXFoundation/boxd/script"
	"github.com/BOXFoundation/boxd/storage"
	_ "github.com/BOXFoundation/boxd/storage/memdb"
	"github.com/facebookgo/ensure"
)

// test setup
var (
	privBytesMiner  = []byte{41, 227, 111, 180, 124, 210, 49, 28, 156, 148, 131, 249, 89, 211, 79, 210, 54, 82, 97, 208, 81, 183, 244, 45, 28, 30, 187, 247, 167, 35, 181, 153}
	privBytesUser   = []byte{109, 162, 71, 154, 180, 47, 74, 157, 44, 36, 228, 16, 110, 27, 14, 208, 190, 118, 25, 106, 13, 154, 241, 107, 156, 9, 98, 118, 152, 129, 69, 185}
	privBytesSplitA = []byte{47, 138, 4, 60, 198, 78, 145, 178, 51, 44, 53, 59, 152, 242, 42, 236, 191, 97, 22, 123, 216, 55, 219, 63, 163, 53, 11, 254, 170, 53, 137, 14}
	privBytesSplitB = []byte{145, 10, 128, 128, 115, 9, 15, 190, 255, 16, 161, 222, 31, 70, 36, 124, 45, 241, 204, 50, 38, 207, 24, 79, 40, 12, 87, 90, 54, 203, 47, 226}

	privKeyMiner, pubKeyMiner, _ = crypto.KeyPairFromBytes(privBytesMiner)
	privKey, pubKey, _           = crypto.KeyPairFromBytes(privBytesUser)
	//privKeyMiner, pubKeyMiner, _ = crypto.NewKeyPair()
	//privKey, pubKey, _           = crypto.NewKeyPair()
	minerAddr, _      = types.NewAddressFromPubKey(pubKeyMiner)
	scriptPubKeyMiner = script.PayToPubKeyHashScript(minerAddr.Hash())
	userAddr, _       = types.NewAddressFromPubKey(pubKey)
	scriptPubKeyUser  = script.PayToPubKeyHashScript(userAddr.Hash())

	//privKeySplitA, pubKeySplitA, _ = crypto.NewKeyPair()
	//privKeySplitB, pubKeySplitB, _ = crypto.NewKeyPair()
	privKeySplitA, pubKeySplitA, _ = crypto.KeyPairFromBytes(privBytesSplitA)
	privKeySplitB, pubKeySplitB, _ = crypto.KeyPairFromBytes(privBytesSplitB)
	splitAddrA, _                  = types.NewAddressFromPubKey(pubKeySplitA)
	scriptPubKeySplitA             = script.PayToPubKeyHashScript(splitAddrA.Hash())
	splitAddrB, _                  = types.NewAddressFromPubKey(pubKeySplitB)
	scriptPubKeySplitB             = script.PayToPubKeyHashScript(splitAddrB.Hash())
	//blockChain                     = NewTestBlockChain()
	timestamp = time.Now().Unix()

	addrs   = []types.Address{splitAddrA, splitAddrB}
	weights = []uint64{5, 5}
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
	if err := signTx(tx, privKeyMiner, pubKeyMiner); err != nil {
		logger.Errorf("Failed to sign tx. Err: %v", err)
		return nil, ""
	}
	txHash, _ := tx.TxHash()
	addr := txlogic.MakeSplitAddress(txHash, 1, addrs, weights)
	logger.Infof("create a split tx. addr: %s", addr)
	return tx, addr.String()
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

func getTailBlock(blockChain *BlockChain) *types.Block {
	tailBlock, _ := blockChain.loadTailBlock()
	return tailBlock
}

func TestChainTx(t *testing.T) {
	blockChain := NewTestBlockChain()
	// blockchain
	ensure.NotNil(t, blockChain)
	ensure.True(t, blockChain.LongestChainHeight == 0)
	// contract blocks test
	b2 := genTestChain(t, blockChain)

	txs := make([]*types.Transaction, 0)
	// tx1
	prevHash, _ := b2.Txs[1].TxHash()
	tx := types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(prevHash, 0), 0)).
		AppendVout(txlogic.MakeVout(splitAddrA.String(), 1000000)).
		AppendVout(txlogic.MakeVout(userAddr.String(), 5000000))
	err := signTx(tx, privKey, pubKey)
	ensure.DeepEqual(t, err, nil)
	txs = append(txs, tx)
	// tx2
	prevHash, _ = tx.TxHash()
	t.Logf("tx1: %s", prevHash)
	tx = types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(prevHash, 1), 0)).
		AppendVout(txlogic.MakeVout(splitAddrA.String(), 2000000)).
		AppendVout(txlogic.MakeVout(userAddr.String(), 3000000))
	err = signTx(tx, privKey, pubKey)
	ensure.DeepEqual(t, err, nil)
	txs = append(txs, tx)
	// tx3
	prevHash, _ = tx.TxHash()
	t.Logf("tx2: %s", prevHash)
	tx = types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(prevHash, 1), 0)).
		AppendVout(txlogic.MakeVout(splitAddrA.String(), 2500000)).
		AppendVout(txlogic.MakeVout(userAddr.String(), 500000))
	err = signTx(tx, privKey, pubKey)
	ensure.DeepEqual(t, err, nil)
	txs = append(txs, tx)

	b3 := nextBlockWithTxs(b2, txs...)

	root, utxoRoot, _ := calcRootHash(b2, b3, blockChain, 0)
	b3.Header.RootHash = *root
	if utxoRoot != nil {
		b3.Header.RootHash = *utxoRoot
	}
	// b3.Header.RootHash.SetString("68f2397dbcea4db529d2b5a208ebe690b18494eb8e2177c024bc8348264b8c57")
	verifyProcessBlock(t, blockChain, b3, nil, 3, b3)
	// check balance
	// for userAddr
	balance := getBalance(userAddr.String(), blockChain.db)
	stateBalance, _ := blockChain.GetBalance(userAddr)
	ensure.DeepEqual(t, balance, stateBalance)
	ensure.DeepEqual(t, balance, uint64(500000))
	// for miner
	balance = getBalance(minerAddr.String(), blockChain.db)
	stateBalance, _ = blockChain.GetBalance(minerAddr)
	ensure.DeepEqual(t, balance, stateBalance)
	ensure.DeepEqual(t, balance, 3*BaseSubsidy-6000000)
	// for splitAddrA
	balance = getBalance(splitAddrA.String(), blockChain.db)
	stateBalance, _ = blockChain.GetBalance(splitAddrA)
	ensure.DeepEqual(t, balance, stateBalance)
	ensure.DeepEqual(t, balance, uint64(5500000))
	t.Logf("b2 -> b3 passed, now tail height: %d", blockChain.LongestChainHeight)
}

func TestBlockChain_WriteDelTxIndex(t *testing.T) {
	blockChain := NewTestBlockChain()
	ensure.NotNil(t, blockChain)

	b0 := getTailBlock(blockChain)

	b1 := nextBlock(b0)
	blockChain.db.EnableBatch()
	defer blockChain.db.DisableBatch()

	prevHash, _ := b0.Txs[0].TxHash()
	tx := types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(prevHash, 0), 0)).
		AppendVout(txlogic.MakeVout(userAddr.String(), 1000))
	signTx(tx, privKeyMiner, pubKeyMiner)
	txHash, _ := tx.TxHash()

	prevHash, _ = b1.Txs[0].TxHash()
	gasUsed, vmValue, gasPrice, gasLimit, nonce := uint64(9912), uint64(0), uint64(6), uint64(20000), uint64(2)
	contractVout, _ := txlogic.MakeContractCreationVout(userAddr.Hash160(), vmValue, gasLimit, gasPrice, nonce, []byte{1})
	vmTx := types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(prevHash, 0), 0)).
		AppendVout(contractVout)
	signTx(vmTx, privKeyMiner, pubKeyMiner)
	vmTxHash, _ := vmTx.TxHash()

	gasRefundTx := createGasRefundUtxoTx(userAddr.Hash160(), gasPrice*(gasLimit-gasUsed), nonce)
	gasRefundTxHash, _ := gasRefundTx.TxHash()

	contractAddr, _ := types.MakeContractAddress(userAddr, nonce)
	op := types.NewOutPoint(types.NormalizeAddressHash(contractAddr.Hash160()), 0)
	contractTx := types.NewTx(0, 0, 0).
		AppendVin(txlogic.MakeContractVin(op, 0)).
		AppendVout(txlogic.MakeVout(userAddr.String(), 2000))
	contractTxHash, _ := contractTx.TxHash()

	b2 := nextBlockWithTxs(b1, tx, vmTx)
	b2.InternalTxs = append(b2.InternalTxs, gasRefundTx, contractTx)

	splitTx1, _ := createSplitTx(b0.Txs[1], 0)
	splitTx2, _ := createSplitTx(b0.Txs[2], 0)
	splitTxs := make(map[crypto.HashType]*types.Transaction)
	splitTxHash1, _ := splitTx1.TxHash()
	splitTxHash2, _ := splitTx2.TxHash()
	splitTxs[*splitTxHash1] = splitTx1
	splitTxs[*splitTxHash2] = splitTx2

	ensure.Nil(t, blockChain.StoreBlockWithIndex(b2, blockChain.db))
	ensure.Nil(t, blockChain.WriteTxIndex(b2, splitTxs, blockChain.db))
	ensure.Nil(t, blockChain.StoreSplitTxs(splitTxs, blockChain.db))
	blockChain.db.Flush()

	// check
	coinBaseTxHash, _ := b2.Txs[0].TxHash()
	hashes := []*crypto.HashType{coinBaseTxHash, txHash, vmTxHash, gasRefundTxHash,
		contractTxHash, splitTxHash1, splitTxHash2}
	txs := []*types.Transaction{b2.Txs[0], tx, vmTx, gasRefundTx, contractTx, splitTx1, splitTx2}

	for i, hash := range hashes {
		_, tx, err := blockChain.LoadBlockInfoByTxHash(*hash)
		ensure.Nil(t, err)
		//txHash, _ := tx.TxHash()
		//iHash, _ := txs[i].TxHash()
		//t.Logf("txhash: %s, txs[%d] hash: %s", txHash, i, iHash)
		ensure.DeepEqual(t, txs[i], tx)
	}
	ensure.Nil(t, blockChain.DelTxIndex(b2, splitTxs, blockChain.db))
	ensure.Nil(t, blockChain.DelSplitTxs(splitTxs, blockChain.db))
	blockChain.db.Flush()
	for _, hash := range hashes {
		_, _, err := blockChain.LoadBlockInfoByTxHash(*hash)
		ensure.NotNil(t, err)
	}
}

func verifyProcessBlock(
	t *testing.T, blockChain *BlockChain, newBlock *types.Block, expectedErr error,
	expectedChainHeight uint32, expectedChainTail *types.Block,
) {
	err := blockChain.ProcessBlock(newBlock, core.DefaultMode /* not broadcast */, "peer1")
	ensure.DeepEqual(t, err, expectedErr)
	ensure.DeepEqual(t, blockChain.LongestChainHeight, expectedChainHeight)
	ensure.DeepEqual(t, getTailBlock(blockChain), expectedChainTail)
}

func calcRootHash(parent, block *types.Block, chain *BlockChain, gascost uint64) (*crypto.HashType, *crypto.HashType, error) {

	statedb, err := state.New(&parent.Header.RootHash, &parent.Header.UtxoRoot, chain.DB())
	if err != nil {
		return nil, nil, err
	}
	logger.Infof("new statedb with root: %s and utxo root: %s block height %d",
		parent.Header.RootHash, parent.Header.UtxoRoot, block.Header.Height)
	utxoSet := NewUtxoSet()
	if err := utxoSet.LoadBlockUtxos(block, true, chain.DB()); err != nil {
		return nil, nil, err
	}
	blockCopy := block.Copy()
	chain.SplitBlockOutputs(blockCopy)
	if err := utxoSet.ApplyBlock(blockCopy); err != nil {
		return nil, nil, err
	}

	chain.SetBlockTxs(block)
	_, _, _, utxoTxs, err :=
		chain.StateProcessor().Process(block, statedb, utxoSet)
	if err != nil {
		return nil, nil, err
	}

	blockCopy.Txs[0].Vout[0].Value += gascost
	block.Txs[0].Vout[0].Value += gascost

	block.Txs[0].ResetTxHash()
	// handle coinbase utxo
	if gascost > 0 {
		for k, v := range utxoSet.GetUtxos() {
			if v.IsCoinBase() {
				v.SetValue(block.Txs[0].Vout[0].Value)
				delete(utxoSet.All(), k)
				utxoSet.AddUtxo(block.Txs[0], 0, block.Header.Height)
				break
			}
		}
	}
	chain.UpdateNormalTxBalanceState(blockCopy, utxoSet, statedb)
	block.InternalTxs = utxoTxs
	if len(utxoTxs) > 0 {
		if err := utxoSet.ApplyInternalTxs(block); err != nil {
			return nil, nil, err
		}
	}
	if err := chain.UpdateUtxoState(statedb, utxoSet); err != nil {
		return nil, nil, err
	}

	root, utxoRoot, err := statedb.Commit(false)
	if err != nil {
		return nil, nil, err
	}
	block.InternalTxs = nil
	block.Txs[0].Vout[0].Value -= gascost
	return root, utxoRoot, nil
}

// Test blockchain block processing
func TestBlockProcessing(t *testing.T) {
	blockChain := NewTestBlockChain()
	ensure.NotNil(t, blockChain)
	ensure.True(t, blockChain.LongestChainHeight == 0)

	b0 := getTailBlock(blockChain)

	// try to append an existing block: genesis block
	verifyProcessBlock(t, blockChain, b0, core.ErrBlockExists, 0, b0)

	// >>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
	// extend main chain
	// b0 -> b1
	// miner: 50box
	b1 := nextBlock(b0)
	root, utxoRoot, err := calcRootHash(b0, b1, blockChain, 0)
	b1.Header.RootHash = *root
	if utxoRoot != nil {
		b1.Header.RootHash = *utxoRoot
	}
	verifyProcessBlock(t, blockChain, b1, nil, 1, b1)
	balance := getBalance(minerAddr.String(), blockChain.db)
	ensure.DeepEqual(t, balance, uint64(50*core.DuPerBox))

	b1DoubleMint := nextBlock(b1)
	b1DoubleMint.Header.TimeStamp = b1.Header.TimeStamp
	b1DoubleMint.Header.BookKeeper = *minerAddr.Hash160()
	verifyProcessBlock(t, blockChain, b1DoubleMint, core.ErrRepeatedMintAtSameTime, 1, b1)
	balance = getBalance(minerAddr.String(), blockChain.db)
	ensure.DeepEqual(t, balance, uint64(50*core.DuPerBox))

	// >>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
	// double spend check
	b2ds := nextBlock(b1)
	// add a tx spending from previous block's coinbase
	root, utxoRoot, err = calcRootHash(b1, b2ds, blockChain, 0)
	b2ds.Header.RootHash = *root
	if utxoRoot != nil {
		b2ds.Header.RootHash = *utxoRoot
	}
	b2ds.Txs = append(b2ds.Txs, createGeneralTx(b1.Txs[0], 0, 50*core.DuPerBox, userAddr.String(), privKeyMiner, pubKeyMiner))
	splitTx, splitAddr := createSplitTx(b1.Txs[0], 0)
	b2ds.Txs = append(b2ds.Txs, splitTx)
	b2ds.Header.TxsRoot = *CalcTxsHash(b2ds.Txs)
	verifyProcessBlock(t, blockChain, b2ds, core.ErrDoubleSpendTx, 1, b1)
	balance = getBalance(minerAddr.String(), blockChain.db)
	ensure.DeepEqual(t, balance, uint64(50*core.DuPerBox))

	// >>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
	// extend main chain
	// b0 -> b1 -> b2
	// Tx: miner -> user: 50
	// miner: 50 box
	// user: 50 box
	b2 := nextBlock(b1)
	// add a tx spending from previous block's coinbase
	b2.Txs = append(b2.Txs, createGeneralTx(b1.Txs[0], 0, 50*core.DuPerBox, userAddr.String(), privKeyMiner, pubKeyMiner))
	b2.Header.TxsRoot = *CalcTxsHash(b2.Txs)
	root, utxoRoot, err = calcRootHash(b1, b2, blockChain, 0)
	b2.Header.RootHash = *root
	if utxoRoot != nil {
		b2.Header.RootHash = *utxoRoot
	}
	// b2.Header.RootHash.SetString("03ccf8dd9752fdd049b2228352af7a2761f5227276d8be33c6c0444385e6ea70")
	verifyProcessBlock(t, blockChain, b2, nil, 2, b2)
	t.Logf("b2 block hash: %s", b2.BlockHash())

	// miner balance: 100 - 50 = 50
	// user balance: 50
	balance = getBalance(minerAddr.String(), blockChain.db)
	ensure.DeepEqual(t, balance, uint64(50*core.DuPerBox))
	balance = getBalance(userAddr.String(), blockChain.db)
	ensure.DeepEqual(t, balance, uint64(50*core.DuPerBox))

	// >>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
	// extend main chain
	// b0 -> b1 -> b2 -> b3
	//
	// miner: 10000561600
	// user: 4999428400
	gasUsed, vmValue, gasPrice, gasLimit := uint64(56160), uint64(10000), uint64(10), uint64(200000)
	vmParam := &testContractParam{
		gasUsed, vmValue, gasPrice, gasLimit, vmValue, 0, nil,
	}
	userBalance, minerBalance = 50*uint64(core.DuPerBox), 50*uint64(core.DuPerBox)
	byteCode, _ := hex.DecodeString(testFaucetContract)
	nonce := uint64(1)
	contractVout, _ := txlogic.MakeContractCreationVout(userAddr.Hash160(), vmValue, gasLimit, gasPrice, nonce, byteCode)
	prevHash, _ := b2.Txs[1].TxHash()
	changeValue := userBalance - vmValue - gasPrice*gasLimit
	vmTx := types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(prevHash, 0), 0)).
		AppendVout(contractVout).
		AppendVout(txlogic.MakeVout(userAddr.String(), changeValue))
	signTx(vmTx, privKey, pubKey)
	vmTxHash, _ := vmTx.TxHash()
	stateDB, err := state.New(&b2.Header.RootHash, nil, blockChain.db)
	ensure.Nil(t, err)
	contractAddrFaucet, _ := types.MakeContractAddress(userAddr, nonce)
	vmParam.contractAddr = contractAddrFaucet
	gasRefundValue := gasPrice * (gasLimit - gasUsed)
	t.Logf("b3 gas refund value: %d", gasRefundValue)
	gasRefundTx := createGasRefundUtxoTx(userAddr.Hash160(), gasRefundValue, nonce)
	b3 := nextBlockWithTxs(b2, vmTx)
	b3.Header.RootHash.SetString("97dd3da30c07a81f55855d94469be511275cdec3a59d3cad15a6973d131b1b3d")
	b3.Header.UtxoRoot.SetString("66d8948a76d01de1a99893f92f0c0ab27666ecb8c0e817e9400ceddb40463a0e")
	receipt := types.NewReceipt(vmTxHash, vmParam.contractAddr.Hash160(), false, gasUsed).WithTxIndex(1)
	b3.Header.ReceiptHash = *(new(types.Receipts).Append(receipt).Hash())
	contractBlockHandle(t, blockChain, b2, b3, b3, vmParam, nil, gasRefundTx)
	bUserBalance := userBalance
	bMinerBalance := minerBalance
	stateDB, err = state.New(&b3.Header.RootHash, &b3.Header.UtxoRoot, blockChain.db)
	ensure.Nil(t, err)
	ensure.DeepEqual(t, stateDB.GetNonce(*userAddr.Hash160()), nonce)

	t.Logf("b3 block hash: %s", b3.BlockHash())
	t.Logf("b2 -> b3 passed, now tail height: %d", blockChain.LongestChainHeight)

	// >>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
	// extend side chain: fork from b1
	// b0 -> b1 -> b2 -> b3
	//		          \ -> b3A
	gasUsed, vmValue, gasPrice, gasLimit = uint64(246403), uint64(0), uint64(10), uint64(400000)
	vmParam = &testContractParam{
		gasUsed, vmValue, gasPrice, gasLimit, vmParam.contractBalance, 0, nil,
	}
	userBalanceA := 50 * uint64(core.DuPerBox)
	byteCode, _ = hex.DecodeString(testCoinContract)
	nonce = uint64(1)
	contractVout, _ = txlogic.MakeContractCreationVout(userAddr.Hash160(), vmValue, gasLimit, gasPrice, nonce, byteCode)
	prevHash, _ = b2.Txs[1].TxHash()
	changeValueA := userBalanceA - vmValue - gasPrice*gasLimit
	vmTx = types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(prevHash, 0), 0)).
		AppendVout(contractVout).
		AppendVout(txlogic.MakeVout(userAddr.String(), changeValueA))
	signTx(vmTx, privKey, pubKey)
	contractAddrCoin, _ := types.MakeContractAddress(userAddr, nonce)
	vmParam.contractAddr = contractAddrCoin
	gasRefundTx = createGasRefundUtxoTx(userAddr.Hash160(), gasPrice*(gasLimit-gasUsed), nonce)
	// split tx
	splitTx, splitAddr = createSplitTx(b2.Txs[0], 0)
	b3A := nextBlockWithTxs(b2, splitTx, vmTx)
	b3A.Header.RootHash.SetString("a0dc75408e1afa110cde963a8e8aec7cd80955bbdd11ada3ed949ccc90e63907")
	utxoRootHash := "c90c4406bc5d3f01cf782adecb92d0ced4cb7f966d144244cfeb6eb2d683a61e"
	b3A.Header.UtxoRoot.SetString(utxoRootHash)

	// gascost = gasPrice * gasUsed
	// root, utxoRoot, err = calcRootHash(b2, b3A, blockChain, gascost)
	// if err != nil {
	// 	logger.Errorf("root: %v", err)
	// }
	// logger.Errorf("root: %v", root)
	// b3A.Header.RootHash = *root
	// if utxoRoot != nil {
	// 	b3A.Header.UtxoRoot = *utxoRoot
	// }

	vmTxHash, _ = vmTx.TxHash()
	receipt = types.NewReceipt(vmTxHash, vmParam.contractAddr.Hash160(), false, gasUsed).WithTxIndex(2)
	b3A.Header.ReceiptHash = *(new(types.Receipts).Append(receipt).Hash())
	contractBlockHandle(t, blockChain, b2, b3A, b3, vmParam, core.ErrBlockInSideChain, gasRefundTx)
	// stateDB, err = state.New(&b3A.Header.RootHash, &b3A.Header.UtxoRoot, blockChain.db)
	// ensure.Nil(t, err)
	// t.Logf("user nonce: %d", stateDB.GetNonce(*userAddr.Hash160()))
	//refundValue := vmParam.gasPrice * (vmParam.gasLimit - vmParam.gasUsed)
	t.Logf("b3A block hash: %s", b3A.BlockHash())
	t.Logf("b2 -> b3A failed: %s", core.ErrBlockInSideChain)
	b3AHash, _ := b3A.Txs[0].TxHash()

	// >>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
	// reorg: side chain grows longer than main chain
	// b0 -> b1 -> b2 -> b3
	//		           \-> b3A -> b4A
	// Tx: miner -> user: 50
	// Tx: miner -> split address: 50
	//
	b4A := nextBlock(b3A)
	b4ATx := types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(b3AHash, 0), 0)).
		AppendVout(txlogic.MakeVout(splitAddr, 50*core.DuPerBox)).
		AppendVout(txlogic.MakeVout(minerAddr.String(), b3A.Txs[0].Vout[0].Value-50*core.DuPerBox))
	signTx(b4ATx, privKeyMiner, pubKeyMiner)
	b4A.Txs = append(b4A.Txs, b4ATx)
	b4A.Header.RootHash.SetString("9b45d26564abecc543b1aef0068e34f0fafbebf97e794d1486af933882aa4cba")
	b4A.Header.UtxoRoot.SetString(utxoRootHash)

	// root, utxoRoot, err = calcRootHash(b4A, b3A, blockChain, 0)
	// b4A.Header.RootHash = *root
	// if utxoRoot != nil {
	// 	b4A.Header.UtxoRoot = *utxoRoot
	// }

	vmParam = &testContractParam{contractAddr: vmParam.contractAddr}
	userBalance = 50*uint64(core.DuPerBox) - gasUsed*gasPrice
	minerBalance = 50*uint64(core.DuPerBox) + gasUsed*gasPrice
	contractBlockHandle(t, blockChain, b3A, b4A, b4A, vmParam, nil)
	t.Logf("b4A block hash: %s", b4A.BlockHash())
	bAUserBalance, bAMinerBalance := userBalance, minerBalance
	t.Logf("b3A -> b4A passed, now tail height: %d", blockChain.LongestChainHeight)

	// check split address balance
	// splitA balance: 25  splitB balance: 25
	blanceSplitA := getBalance(splitAddrA.String(), blockChain.db)
	ensure.DeepEqual(t, blanceSplitA, uint64(25*core.DuPerBox))
	blanceSplitB := getBalance(splitAddrB.String(), blockChain.db)
	ensure.DeepEqual(t, blanceSplitB, uint64(25*core.DuPerBox))

	//TODO: add insuffient balance check

	// >>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
	// Extend b3 fork twice to make first chain longer and force reorg
	// b0 -> b1 -> b2  -> b3  -> b4 -> b5
	// 		            \-> b3A -> b4A
	// Tx: miner -> user: 50
	// user: 50 box - gas
	// mienr: 2 * 50 box + gas
	b4 := nextBlock(b3)
	b4.Header.RootHash.SetString("99500cad5332878ffc3f833fec14b1fe97b588c444e5e0933529c7ec327af5d6")
	b4.Header.UtxoRoot = b3.Header.UtxoRoot
	//verifyProcessBlock(t, blockChain, b4, core.ErrBlockInSideChain, 4, b4A)
	vmParam = &testContractParam{contractAddr: contractAddrFaucet}
	userBalance = 50*uint64(core.DuPerBox) - gasUsed*gasPrice
	minerBalance = 2*50*uint64(core.DuPerBox) + gasUsed*gasPrice
	contractBlockHandle(t, blockChain, b3, b4, b4A, vmParam, core.ErrBlockInSideChain)
	t.Logf("b4 block hash: %s", b4.BlockHash())
	t.Logf("b3 -> b4 pass, now tail height: %d", blockChain.LongestChainHeight)

	// process b5
	gasUsed, vmValue, gasPrice, gasLimit = uint64(9912), uint64(0), uint64(6), uint64(20000)
	contractBalance := uint64(10000 - 2000) // withdraw 2000, construct contract with 10000
	vmParam = &testContractParam{
		gasUsed, vmValue, gasPrice, gasLimit, contractBalance, 2000, contractAddrFaucet,
	}
	byteCode, _ = hex.DecodeString(testFaucetCall)
	nonce = 2
	contractVout, _ = txlogic.MakeContractCallVout(userAddr.Hash160(), contractAddrFaucet.Hash160(), vmValue,
		gasLimit, gasPrice, nonce, byteCode)
	// use internal tx vout
	prevHash, _ = b3.InternalTxs[0].TxHash()
	changeValue5 := gasRefundValue - vmValue - gasPrice*gasLimit
	vmTx = types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(prevHash, 0), 0)).
		AppendVout(contractVout).
		AppendVout(txlogic.MakeVout(userAddr.String(), changeValue5))
	signTx(vmTx, privKey, pubKey)
	gasRefundTx = createGasRefundUtxoTx(userAddr.Hash160(), gasPrice*(gasLimit-gasUsed), nonce)
	op := types.NewOutPoint(types.NormalizeAddressHash(contractAddrFaucet.Hash160()), 0)
	contractTx := types.NewTx(0, 0, 0).
		AppendVin(txlogic.MakeContractVin(op, 0)).
		AppendVout(txlogic.MakeVout(userAddr.String(), 2000))
	vmTxHash, _ = vmTx.TxHash()
	b5 := nextBlockWithTxs(b4, vmTx)
	b5.Header.RootHash.SetString("ca958bc22a6247a57c84047e1cc4a0267ea5a1f720073165866ed2cc81fcb063")
	b5.Header.UtxoRoot = b3.Header.UtxoRoot
	receipt = types.NewReceipt(vmTxHash, contractAddrFaucet.Hash160(), false, gasUsed).WithTxIndex(1)
	b5.Header.ReceiptHash = *(new(types.Receipts).Append(receipt).Hash())
	userBalance, minerBalance = bUserBalance, bMinerBalance+50*core.DuPerBox
	contractBlockHandle(t, blockChain, b4, b5, b5, vmParam, nil, gasRefundTx, contractTx)
	bUserBalance, bMinerBalance = userBalance, minerBalance
	t.Logf("b5 block hash: %s", b5.BlockHash())
	t.Logf("b4 -> b5 passed, now tail height: %d", blockChain.LongestChainHeight)

	// check balance
	blanceSplitA = getBalance(splitAddrA.String(), blockChain.db)
	ensure.DeepEqual(t, blanceSplitA, uint64(0))
	blanceSplitB = getBalance(splitAddrB.String(), blockChain.db)
	ensure.DeepEqual(t, blanceSplitB, uint64(0))

	// >>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
	// b0 -> b1 -> b2  -> b3  -> b4  -> b5
	//							 \ -> b3A -> b4A -> b5A -> b6A
	b5A := nextBlock(b4A)
	b5A.Txs = append(b5A.Txs, createGeneralTx(b4A.Txs[0], 0, 50*core.DuPerBox,
		userAddr.String(), privKeyMiner, pubKeyMiner))
	b5A.Header.RootHash.SetString("7688d443d4df8e614559b132bf80d97795176bf2aa814488912c33d54740f645")
	b5A.Header.UtxoRoot.SetBytes(b4A.Header.UtxoRoot[:])
	vmParam = &testContractParam{contractAddr: contractAddrCoin, contractBalance: 8000}
	bAUserBalance = bAUserBalance + 50*core.DuPerBox
	contractBlockHandle(t, blockChain, b4A, b5A, b5, vmParam, core.ErrBlockInSideChain)
	t.Logf("b5A block hash: %s", b5A.BlockHash())
	t.Logf("b4A -> b5A failed, now tail height: %d", blockChain.LongestChainHeight)

	b6A := nextBlock(b5A)
	b6A.Txs = append(b6A.Txs, createGeneralTx(b3A.Txs[0], 0, 50*core.DuPerBox,
		userAddr.String(), privKeyMiner, pubKeyMiner))
	b6A.Header.TxsRoot = *CalcTxsHash(b6A.Txs)
	// reorg has happened
	b6A.Header.RootHash.SetString("a22718f2316c661afbb3e935c4a54d01f3840cbb17e4e1ae3567fbee31488a61")
	verifyProcessBlock(t, blockChain, b6A, core.ErrMissingTxOut, 5, b5A)
	t.Logf("b5A -> b6A failed, now tail height: %d", blockChain.LongestChainHeight)

	gasUsed, vmValue, gasPrice, gasLimit = uint64(23177), uint64(0), uint64(6), uint64(30000)
	vmParam = &testContractParam{
		gasUsed, vmValue, gasPrice, gasLimit, 0, 0, contractAddrCoin,
	}
	byteCode, _ = hex.DecodeString(mintCall)
	nonce = 2
	contractVout, _ = txlogic.MakeContractCallVout(userAddr.Hash160(), contractAddrCoin.Hash160(), vmValue,
		gasLimit, gasPrice, nonce, byteCode)
	prevHash, _ = b5A.Txs[1].TxHash()
	changeValueA = 50*core.DuPerBox - vmValue - gasPrice*gasLimit
	vmTx = types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(prevHash, 0), 0)).
		AppendVout(contractVout).
		AppendVout(txlogic.MakeVout(userAddr.String(), changeValueA))
	signTx(vmTx, privKey, pubKey)
	gasRefundTx = createGasRefundUtxoTx(userAddr.Hash160(), gasPrice*(gasLimit-gasUsed), nonce)
	b6A = nextBlockWithTxs(b5A, vmTx)
	b6A.Header.RootHash.SetString("421b80141d389d901d91786849b11568ade3a1c569db544d97c59e09c2140acf")
	b6A.Header.UtxoRoot = b3A.Header.UtxoRoot
	vmTxHash, _ = vmTx.TxHash()
	receipt = types.NewReceipt(vmTxHash, contractAddrCoin.Hash160(), false, gasUsed).WithTxIndex(1)
	b6A.Header.ReceiptHash = *(new(types.Receipts).Append(receipt).Hash())
	userBalance, minerBalance = bAUserBalance, bAMinerBalance
	contractBlockHandle(t, blockChain, b5A, b6A, b6A, vmParam, nil, gasRefundTx)
	bAUserBalance, bAMinerBalance = userBalance, minerBalance
	b6AUserBalance, b6AMinerBalance := userBalance, minerBalance
	t.Logf("b6A block hash: %s", b6A.BlockHash())
	t.Logf("b5A -> b6A pass, now tail height: %d", blockChain.LongestChainHeight)

	// check balance
	blanceSplitA = getBalance(splitAddrA.String(), blockChain.db)
	ensure.DeepEqual(t, blanceSplitA, uint64(25*core.DuPerBox))
	blanceSplitB = getBalance(splitAddrB.String(), blockChain.db)
	ensure.DeepEqual(t, blanceSplitB, uint64(25*core.DuPerBox))

	// >>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
	// b0 -> b1 -> b2  -> b3  -> b4  -> b5
	// 		             -> b3A -> b4A -> b5A -> b6A -> b7A
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
	b7ATx := createGeneralTx(b4ASplitTx, 0, 25*core.DuPerBox, userAddr.String(),
		privKeySplitA, pubKeySplitA)
	b7A.Txs = append(b7A.Txs, b7ATx)
	b7A.Header.RootHash.SetString("f4a3248f3629fcdf02ebdab1f65bf01f87e7b9e991618a4e4e5b7dd69baeb339")
	b7A.Header.UtxoRoot.SetBytes(b6A.Header.UtxoRoot[:])
	userBalance = bAUserBalance + 25*core.DuPerBox
	vmParam = &testContractParam{contractAddr: contractAddrCoin}
	contractBlockHandle(t, blockChain, b6A, b7A, b7A, vmParam, nil)
	bAUserBalance, bAMinerBalance = userBalance, minerBalance
	t.Logf("b7A block hash: %s", b7A.BlockHash())
	t.Logf("b6A -> b7A pass, now tail height: %d", blockChain.LongestChainHeight)

	// check balance
	// splitAddrA balance: 0
	// splitAddrB balance: 25
	blanceSplitA = getBalance(splitAddrA.String(), blockChain.db)
	ensure.DeepEqual(t, blanceSplitA, uint64(0))
	blanceSplitB = getBalance(splitAddrB.String(), blockChain.db)
	ensure.DeepEqual(t, blanceSplitB, uint64(25*core.DuPerBox))

	// >>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
	// force reorg split tx
	// b0 -> b1 -> b2  -> b3  -> b4  -> b5
	// 		             -> b3A -> b4A -> b5A -> b6A -> b7A
	//                                             -> b7B -> b8B
	gasUsed, vmValue, gasPrice, gasLimit = uint64(30219), uint64(0), uint64(6), uint64(40000)
	vmParam = &testContractParam{
		gasUsed, vmValue, gasPrice, gasLimit, 0, 0, contractAddrCoin,
	}
	byteCode, _ = hex.DecodeString(sendCall)
	nonce = uint64(3)
	contractVout, _ = txlogic.MakeContractCallVout(userAddr.Hash160(), contractAddrCoin.Hash160(),
		vmValue, gasLimit, gasPrice, nonce, byteCode)
	prevHash, _ = b6A.Txs[1].TxHash()
	changeValueA = changeValueA - vmValue - gasPrice*gasLimit
	vmTx = types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(prevHash, 1), 0)).
		AppendVout(contractVout).
		AppendVout(txlogic.MakeVout(userAddr.String(), changeValueA))
	signTx(vmTx, privKey, pubKey)
	gasRefundTx = createGasRefundUtxoTx(userAddr.Hash160(), gasPrice*(gasLimit-gasUsed), nonce)
	b7B := nextBlockWithTxs(b6A, vmTx)
	b7B.Header.RootHash.SetString("06b26ef05aa4c09e3b1b4c7d72d8657d75d8e3a7bbf5bc8069308a1c39c70528")
	b7B.Header.UtxoRoot.SetBytes(b6A.Header.UtxoRoot[:])
	vmTxHash, _ = vmTx.TxHash()
	receipt = types.NewReceipt(vmTxHash, contractAddrCoin.Hash160(), false, gasUsed).WithTxIndex(1)
	b7B.Header.ReceiptHash = *(new(types.Receipts).Append(receipt).Hash())
	userBalance, minerBalance = bAUserBalance, bAMinerBalance
	contractBlockHandle(t, blockChain, b6A, b7B, b7A, vmParam, core.ErrBlockInSideChain, gasRefundTx)
	t.Logf("b7B block hash: %s", b7B.BlockHash())
	t.Logf("b6A -> b7B failed, now tail height: %d", blockChain.LongestChainHeight)

	b8B := nextBlock(b7B)
	b8B.Header.RootHash.SetString("94af28c33721275fa1863a7422497067a3c5c65c3a5ff575dc798980855892af")
	b8B.Header.UtxoRoot.SetBytes(b7B.Header.UtxoRoot[:])
	userBalance, minerBalance = b6AUserBalance-gasUsed*gasPrice,
		b6AMinerBalance+50*core.DuPerBox+(gasUsed*gasPrice)
	vmParam = &testContractParam{contractAddr: contractAddrCoin}
	contractBlockHandle(t, blockChain, b7B, b8B, b8B, vmParam, nil)
	t.Logf("b8B block hash: %s", b8B.BlockHash())
	t.Logf("b7B -> b8B pass, now tail height: %d", blockChain.LongestChainHeight)

	// check split balance
	// splitAddrA balance: 25
	// splitAddrB balance: 25
	blanceSplitA = getBalance(splitAddrA.String(), blockChain.db)
	ensure.DeepEqual(t, blanceSplitA, uint64(25*core.DuPerBox))
	blanceSplitB = getBalance(splitAddrB.String(), blockChain.db)
	ensure.DeepEqual(t, blanceSplitB, uint64(25*core.DuPerBox))

	// >>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
	// force reorg split tx
	// b0 -> b1 -> b2  -> b3  -> b4  -> b5  -> b6  -> b7  -> b8  -> b9
	// 		           -> b3A -> b4A -> b5A -> b6A -> b7A
	//                                             -> b7B -> b8B
	// Tx: miner -> user: 50
	// Tx: miner -> split address: 50
	// Tx: splitA -> user: 25
	b6 := nextBlock(b5)
	b6.Header.RootHash.SetString("542a763b38455373ab9cfa592cc0466f7da44ef728f15d887922d5351ffc0143")
	b6.Header.UtxoRoot.SetBytes(b5.Header.UtxoRoot[:])
	t.Logf("b6 block hash: %s", b6.BlockHash())
	verifyProcessBlock(t, blockChain, b6, core.ErrBlockInSideChain, 8, b8B)
	b7 := nextBlock(b6)
	b7.Header.RootHash.SetString("6bfd9a22169ba679e7ac43d0fa0074152fa802c27a43014e9be035e82f479c16")
	b7.Header.UtxoRoot.SetBytes(b6.Header.UtxoRoot[:])
	t.Logf("b7 block hash: %s", b7.BlockHash())
	verifyProcessBlock(t, blockChain, b7, core.ErrBlockInSideChain, 8, b8B)
	b8 := nextBlock(b7)
	b8.Header.RootHash.SetString("85e1e279541e261a317c089003f6d7375b28a9d4aafd25066fea568d588cc035")
	b8.Header.UtxoRoot.SetBytes(b7.Header.UtxoRoot[:])
	t.Logf("b8 block hash: %s", b8.BlockHash())
	verifyProcessBlock(t, blockChain, b8, core.ErrBlockInSideChain, 8, b8B)

	gasUsed, vmValue, gasPrice, gasLimit = uint64(9914), uint64(0), uint64(6), uint64(20000)
	contractBalance = uint64(0) // withdraw 2000+9000, construct contract with 10000
	vmParam = &testContractParam{
		// gasUsed, vmValue, gasPrice, gasLimit, contractBalance, userRecv, contractAddr
		gasUsed, vmValue, gasPrice, gasLimit, 8000, 0, contractAddrFaucet,
	}
	byteCode, _ = hex.DecodeString(testFaucetCall2)
	nonce = 3
	contractVout, _ = txlogic.MakeContractCallVout(userAddr.Hash160(), contractAddrFaucet.Hash160(),
		vmValue, gasLimit, gasPrice, nonce, byteCode)
	prevHash, _ = b3.Txs[1].TxHash()
	changeValue = changeValue - vmValue - gasPrice*gasLimit
	vmTx = types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(prevHash, 1), 0)).
		AppendVout(contractVout).
		AppendVout(txlogic.MakeVout(userAddr.String(), changeValue))
	gasRefundTx = createGasRefundUtxoTx(userAddr.Hash160(), gasPrice*(gasLimit-gasUsed), nonce)
	signTx(vmTx, privKey, pubKey)
	vmTxHash, _ = vmTx.TxHash()
	b9 := nextBlockWithTxs(b8, vmTx)
	b9.Header.RootHash.SetString("0f653d453e8d64b69b0afbce218d7e7ce30cb7113f146c351d844e13bf1d5126")
	b9.Header.UtxoRoot.SetBytes(b8.Header.UtxoRoot[:])
	receipt = types.NewReceipt(vmTxHash, contractAddrFaucet.Hash160(), true, gasUsed).WithTxIndex(1)
	b9.Header.ReceiptHash = *(new(types.Receipts).Append(receipt).Hash())
	userBalance = bUserBalance
	minerBalance = bMinerBalance + 50*core.DuPerBox*3
	contractBlockHandle(t, blockChain, b8, b9, b9, vmParam, nil, gasRefundTx)
	t.Logf("b9 block hash: %s", b9.BlockHash())
	t.Logf("b8 -> b9 passed, now tail height: %d", blockChain.LongestChainHeight)

	// check balance
	// splitAddrA balance: 0
	// splitAddrB balance: 0
	blanceSplitA = getBalance(splitAddrA.String(), blockChain.db)
	ensure.DeepEqual(t, blanceSplitA, uint64(0))
	blanceSplitB = getBalance(splitAddrB.String(), blockChain.db)
	ensure.DeepEqual(t, blanceSplitB, uint64(0))
}
