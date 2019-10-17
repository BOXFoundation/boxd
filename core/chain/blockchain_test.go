// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package chain

import (
	"encoding/hex"
	"math/big"
	"testing"
	"time"

	"github.com/BOXFoundation/boxd/core"
	"github.com/BOXFoundation/boxd/core/txlogic"
	"github.com/BOXFoundation/boxd/core/types"
	state "github.com/BOXFoundation/boxd/core/worldstate"
	"github.com/BOXFoundation/boxd/crypto"
	"github.com/BOXFoundation/boxd/script"
	"github.com/BOXFoundation/boxd/storage"
	_ "github.com/BOXFoundation/boxd/storage/memdb"
	"github.com/BOXFoundation/boxd/util/bloom"
	"github.com/facebookgo/ensure"
)

const (
	gasPrice = core.FixedGasPrice
)

// test setup
var (
	privBytesMiner  = []byte{41, 227, 111, 180, 124, 210, 49, 28, 156, 148, 131, 249, 89, 211, 79, 210, 54, 82, 97, 208, 81, 183, 244, 45, 28, 30, 187, 247, 167, 35, 181, 153}
	privBytesUser   = []byte{109, 162, 71, 154, 180, 47, 74, 157, 44, 36, 228, 16, 110, 27, 14, 208, 190, 118, 25, 106, 13, 154, 241, 107, 156, 9, 98, 118, 152, 129, 69, 185}
	privBytesSplitA = []byte{47, 138, 4, 60, 198, 78, 145, 178, 51, 44, 53, 59, 152, 242, 42, 236, 191, 97, 22, 123, 216, 55, 219, 63, 163, 53, 11, 254, 170, 53, 137, 14}
	privBytesSplitB = []byte{145, 10, 128, 128, 115, 9, 15, 190, 255, 16, 161, 222, 31, 70, 36, 124, 45, 241, 204, 50, 38, 207, 24, 79, 40, 12, 87, 90, 54, 203, 47, 226}

	privKeyMiner, pubKeyMiner, _ = crypto.KeyPairFromBytes(privBytesMiner)
	privKey, pubKey, _           = crypto.KeyPairFromBytes(privBytesUser)
	minerAddr, _                 = types.NewAddressFromPubKey(pubKeyMiner)
	scriptPubKeyMiner            = script.PayToPubKeyHashScript(minerAddr.Hash())
	userAddr, _                  = types.NewAddressFromPubKey(pubKey)
	scriptPubKeyUser             = script.PayToPubKeyHashScript(userAddr.Hash())

	privKeySplitA, pubKeySplitA, _ = crypto.KeyPairFromBytes(privBytesSplitA)
	privKeySplitB, pubKeySplitB, _ = crypto.KeyPairFromBytes(privBytesSplitB)
	splitAddrA, _                  = types.NewAddressFromPubKey(pubKeySplitA)
	splitAddrB, _                  = types.NewAddressFromPubKey(pubKeySplitB)
	//blockChain                     = NewTestBlockChain()
	startTime = time.Now().Unix()
	timestamp = startTime

	addrs   = []*types.AddressHash{splitAddrA.Hash160(), splitAddrB.Hash160()}
	weights = []uint32{5, 5}
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

func createSplitTx(
	parentTx *types.Transaction, index uint32, amount uint64,
) (*types.Transaction, string) {

	prevHash, _ := parentTx.TxHash()
	vIn := txlogic.MakeVin(types.NewOutPoint(prevHash, index), 0)
	txOut := &types.TxOut{
		Value:        amount,
		ScriptPubKey: *scriptPubKeyMiner,
	}
	splitAddrOut := txlogic.MakeSplitAddrVout(addrs, weights)
	tx := &types.Transaction{
		Vin:  []*types.TxIn{vIn},
		Vout: []*types.TxOut{txOut, splitAddrOut},
	}
	if err := txlogic.SignTx(tx, privKeyMiner, pubKeyMiner); err != nil {
		logger.Errorf("Failed to sign tx. Err: %v", err)
		return nil, ""
	}
	txHash, _ := tx.TxHash()
	addr := txlogic.MakeSplitAddress(txHash, 1, addrs, weights)
	logger.Infof("create a split tx. addr: %s", addr)
	return tx, addr.String()
}

func getBalance(address *types.AddressHash, db storage.Table) uint64 {
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

func makeCoinbaseTx(
	parent *types.Block, block *types.Block, chain *BlockChain, statedb *state.StateDB,
) (*types.Transaction, error) {

	amount := CalcBlockSubsidy(block.Header.Height)
	nonce := statedb.GetNonce(block.Header.BookKeeper)
	statedb.AddBalance(block.Header.BookKeeper, new(big.Int).SetUint64(amount))

	return chain.MakeInternalContractTx(block.Header.BookKeeper, amount, nonce+1, block.Header.Height, "calcBonus")
}

func makeCoinbaseTxV2(
	parent *types.Block, block *types.Block, chain *BlockChain,
) (*types.Transaction, error) {

	amount := CalcBlockSubsidy(block.Header.Height)
	return chain.MakeInternalContractTx(block.Header.BookKeeper, amount,
		uint64(parent.Header.Height+1), block.Header.Height, "calcBonus")
}

// generate a child block with contract tx
func nextBlockWithTxs(
	parent *types.Block, chain *BlockChain, txs ...*types.Transaction,
) *types.Block {

	timestamp++
	newBlock := types.NewBlock(parent)
	statedb, _ := state.New(&parent.Header.RootHash, &parent.Header.UtxoRoot, chain.DB())
	newBlock.Header.BookKeeper = *minerAddr.Hash160()
	coinbaseTx, _ := makeCoinbaseTx(parent, newBlock, chain, statedb)
	newBlock.Txs = []*types.Transaction{coinbaseTx}
	newBlock.Header.TxsRoot = *CalcTxsHash(newBlock.Txs)
	newBlock.Header.TimeStamp = timestamp
	newBlock.Header.Bloom = bloom.NewFilterWithMK(types.BloomBitLength, types.BloomHashNum)

	newBlock.Txs = append(newBlock.Txs, txs...)
	newBlock.Header.TxsRoot = *CalcTxsHash(newBlock.Txs)
	return newBlock
}

func nextBlockV2(parentBlock *types.Block, chain *BlockChain) *types.Block {
	timestamp++
	newBlock := types.NewBlock(parentBlock)
	newBlock.Header.BookKeeper = *minerAddr.Hash160()
	coinbaseTx, _ := makeCoinbaseTxV2(parentBlock, newBlock, chain)

	newBlock.Txs = []*types.Transaction{coinbaseTx}
	newBlock.Header.TxsRoot = *CalcTxsHash(newBlock.Txs)
	newBlock.Header.TimeStamp = timestamp
	newBlock.Header.Bloom = bloom.NewFilterWithMK(types.BloomBitLength, types.BloomHashNum)

	return newBlock
}

// generate a child block with contract tx
func nextBlockWithTxsV2(
	parent *types.Block, chain *BlockChain, txs ...*types.Transaction,
) *types.Block {
	newBlock := nextBlockV2(parent, chain)
	newBlock.Txs = append(newBlock.Txs, txs...)
	newBlock.Header.TxsRoot = *CalcTxsHash(newBlock.Txs)
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
	changeValue := userBalance - 10000000 - core.TransferFee
	tx := types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(prevHash, 0), 0)).
		AppendVout(txlogic.MakeVout(splitAddrA.Hash160(), 10000000)).
		AppendVout(txlogic.MakeVout(userAddr.Hash160(), changeValue))
	err := txlogic.SignTx(tx, privKey, pubKey)
	ensure.DeepEqual(t, err, nil)
	txs = append(txs, tx)
	// tx2
	prevHash, _ = tx.TxHash()
	t.Logf("tx1: %s", prevHash)
	changeValue -= 20000000 + core.TransferFee
	tx = types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(prevHash, 1), 0)).
		AppendVout(txlogic.MakeVout(splitAddrA.Hash160(), 20000000)).
		AppendVout(txlogic.MakeVout(userAddr.Hash160(), changeValue))
	err = txlogic.SignTx(tx, privKey, pubKey)
	ensure.DeepEqual(t, err, nil)
	txs = append(txs, tx)
	// tx3
	prevHash, _ = tx.TxHash()
	t.Logf("tx2: %s", prevHash)
	changeValue -= 25000000 + core.TransferFee
	tx = types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(prevHash, 1), 0)).
		AppendVout(txlogic.MakeVout(splitAddrA.Hash160(), 25000000)).
		AppendVout(txlogic.MakeVout(userAddr.Hash160(), changeValue))
	err = txlogic.SignTx(tx, privKey, pubKey)
	ensure.DeepEqual(t, err, nil)
	txs = append(txs, tx)

	b3 := nextBlockWithTxs(b2, blockChain, txs...)
	if err := calcRootHash(b2, b3, blockChain); err != nil {
		t.Fatal(err)
	}
	verifyProcessBlock(t, blockChain, b3, nil, 3, b3)
	// check balance
	// for userAddr
	balance := getBalance(userAddr.Hash160(), blockChain.db)
	stateBalance := blockChain.tailState.GetBalance(*userAddr.Hash160()).Uint64()
	ensure.DeepEqual(t, balance, stateBalance)
	ensure.DeepEqual(t, balance, changeValue)
	// for miner
	balance = getBalance(minerAddr.Hash160(), blockChain.db)
	stateBalance = blockChain.tailState.GetBalance(*minerAddr.Hash160()).Uint64()
	ensure.DeepEqual(t, balance, stateBalance)
	ensure.DeepEqual(t, balance, BaseSubsidy-userBalance-core.TransferFee)
	// for splitAddrA
	balance = getBalance(splitAddrA.Hash160(), blockChain.db)
	stateBalance = blockChain.tailState.GetBalance(*splitAddrA.Hash160()).Uint64()
	ensure.DeepEqual(t, balance, stateBalance)
	ensure.DeepEqual(t, balance, uint64(55000000))
	t.Logf("b2 -> b3 passed, now tail height: %d", blockChain.LongestChainHeight)
}

func TestBlockChain_WriteDelTxIndex(t *testing.T) {
	blockChain := NewTestBlockChain()
	ensure.NotNil(t, blockChain)

	b0 := getTailBlock(blockChain)

	b1 := nextBlockV2(b0, blockChain)
	// blockChain.db.EnableBatch()
	// defer blockChain.db.DisableBatch()
	batch := blockChain.db.NewBatch()
	defer batch.Close()

	prevHash, _ := b0.Txs[0].TxHash()
	tx := types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(prevHash, 0), 0)).
		AppendVout(txlogic.MakeVout(userAddr.Hash160(), 1000))
	txlogic.SignTx(tx, privKeyMiner, pubKeyMiner)
	txHash, _ := tx.TxHash()

	prevHash, _ = b1.Txs[0].TxHash()
	gasUsed, vmValue, gasLimit, nonce := uint64(9912), uint64(0), uint64(20000), uint64(2)
	contractVout, _ := txlogic.MakeContractCreationVout(userAddr.Hash160(), vmValue,
		gasLimit, nonce)
	vmTx := types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(prevHash, 0), 0)).
		AppendVout(contractVout).
		WithData(types.ContractDataType, []byte{1})
	txlogic.SignTx(vmTx, privKeyMiner, pubKeyMiner)
	vmTxHash, _ := vmTx.TxHash()

	gasRefundTx := createGasRefundUtxoTx(userAddr.Hash160(), gasPrice*(gasLimit-gasUsed), nonce)
	gasRefundTxHash, _ := gasRefundTx.TxHash()

	contractAddr, _ := types.MakeContractAddress(userAddr, nonce)
	op := types.NewOutPoint(types.NormalizeAddressHash(contractAddr.Hash160()), 0)
	contractTx := types.NewTx(0, 0, 0).
		AppendVin(txlogic.MakeContractVin(op, 1, 0)).
		AppendVout(txlogic.MakeVout(userAddr.Hash160(), 2000))
	contractTxHash, _ := contractTx.TxHash()

	b2 := nextBlockWithTxs(b1, blockChain, tx, vmTx)
	b2.InternalTxs = append(b2.InternalTxs, gasRefundTx, contractTx)

	splitAmount := uint64(50 * core.DuPerBox)
	splitTx1, _ := createSplitTx(b0.Txs[1], 0, splitAmount)
	splitTx2, _ := createSplitTx(b0.Txs[2], 0, splitAmount)
	splitTxs := make(map[crypto.HashType]*types.Transaction)
	splitTxHash1, _ := splitTx1.TxHash()
	splitTxHash2, _ := splitTx2.TxHash()
	splitTxs[*splitTxHash1] = splitTx1
	splitTxs[*splitTxHash2] = splitTx2

	ensure.Nil(t, blockChain.StoreBlockWithIndex(b2, batch))
	ensure.Nil(t, blockChain.WriteTxIndex(b2, splitTxs, batch))
	ensure.Nil(t, blockChain.StoreSplitTxs(splitTxs, batch))
	batch.Write()

	// check
	coinBaseTxHash, _ := b2.Txs[0].TxHash()
	hashes := []*crypto.HashType{coinBaseTxHash, txHash, vmTxHash, gasRefundTxHash,
		contractTxHash, splitTxHash1, splitTxHash2}
	txs := []*types.Transaction{b2.Txs[0], tx, vmTx, gasRefundTx, contractTx, splitTx1, splitTx2}

	for i, hash := range hashes {
		_, tx, _, err := blockChain.LoadBlockInfoByTxHash(*hash)
		ensure.Nil(t, err)
		ensure.DeepEqual(t, txs[i], tx)
	}
	ensure.Nil(t, blockChain.DelTxIndex(b2, splitTxs, batch))
	ensure.Nil(t, blockChain.DelSplitTxs(splitTxs, batch))
	batch.Write()
	for _, hash := range hashes {
		_, _, _, err := blockChain.LoadBlockInfoByTxHash(*hash)
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

func calcRootHash(parent, block *types.Block, chain *BlockChain, stdb ...*state.StateDB) error {
	logger.Info("================ start calc root hash ================")
	defer logger.Info("---------------- end calc root hash ----------------")
	statedb, err := state.New(&parent.Header.RootHash, &parent.Header.UtxoRoot, chain.DB())
	if err != nil {
		return err
	}
	if stdb != nil {
		statedb = stdb[0]
	}
	logger.Infof("new statedb with root: %s and utxo root: %s block %s, height %d",
		parent.Header.RootHash, parent.Header.UtxoRoot, block.BlockHash(), block.Header.Height)
	utxoSet := NewUtxoSet()
	if err := utxoSet.LoadBlockUtxos(block, true, chain.DB()); err != nil {
		return err
	}
	totalFees, err := validateBlockInputs(block.Txs, utxoSet)
	if err != nil {
		return err
	}
	block.Txs[0].Vout[0].Value += totalFees
	block.Txs[0].ResetTxHash()
	blockCopy := block.Copy()
	chain.SplitBlockOutputs(blockCopy)
	if err := utxoSet.ApplyBlock(blockCopy); err != nil {
		return err
	}
	statedb.AddBalance(block.Header.BookKeeper, new(big.Int).SetUint64(block.Txs[0].Vout[0].Value))
	receipts, gasUsed, utxoTxs, err :=
		chain.StateProcessor().Process(block, statedb, utxoSet)
	if err != nil {
		return err
	}

	chain.UpdateNormalTxBalanceState(blockCopy, utxoSet, statedb)
	// update genesis contract utxo in utxoSet
	op := types.NewOutPoint(types.NormalizeAddressHash(&ContractAddr), 0)
	genesisUtxoWrap := utxoSet.GetUtxo(op)
	genesisUtxoWrap.SetValue(genesisUtxoWrap.Value() + gasUsed)

	if len(utxoTxs) > 0 {
		block.InternalTxs = utxoTxs
		internalTxsRoot := CalcTxsHash(utxoTxs)
		block.Header.InternalTxsRoot = *internalTxsRoot
		if err := utxoSet.ApplyInternalTxs(block); err != nil {
			return err
		}
	}
	if err := chain.UpdateContractUtxoState(statedb, utxoSet); err != nil {
		return err
	}

	root, utxoRoot, err := statedb.Commit(false)
	if err != nil {
		return err
	}
	logger.Infof("statedb commit with root: %s, utxo root: %s, block height: %d",
		root, utxoRoot, block.Header.Height)
	block.Header.RootHash = *root
	if utxoRoot != nil {
		block.Header.UtxoRoot = *utxoRoot
	}

	block.Header.GasUsed = gasUsed
	if len(receipts) > 0 {
		block.Header.ReceiptHash = *receipts.Hash()
	}
	block.Header.TxsRoot = *CalcTxsHash(block.Txs)
	block.Hash = nil
	return nil
}

// Test blockchain block processing
func TestBlockProcessing(t *testing.T) {
	blockChain := NewTestBlockChain()
	blockChainI := NewTestBlockChain()
	blockChainA := NewTestBlockChain()
	blockChainB := NewTestBlockChain()
	ensure.NotNil(t, blockChain)
	ensure.True(t, blockChain.LongestChainHeight == 0)

	b2 := genTestChain(t, blockChain)
	genTestChain(t, blockChainI)
	genTestChain(t, blockChainA)
	genTestChain(t, blockChainB)

	bAUserBalance := userBalance

	// >>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
	// extend main chain
	// b0 -> b1 -> b2 -> b3
	//
	vmValue, gasLimit := uint64(10000), uint64(200000)
	vmParam := &testContractParam{
		vmValue, gasLimit, vmValue, 0, nil,
	}
	byteCode, _ := hex.DecodeString(testFaucetContract)
	nonce := uint64(1)
	contractVout, _ := txlogic.MakeContractCreationVout(userAddr.Hash160(),
		vmValue, gasLimit, nonce)
	prevHash, _ := b2.Txs[1].TxHash()
	changeValue := userBalance - vmValue - gasPrice*gasLimit
	vmTx := types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(prevHash, 0), 0)).
		AppendVout(contractVout).
		AppendVout(txlogic.MakeVout(userAddr.Hash160(), changeValue)).
		WithData(types.ContractDataType, byteCode)
	txlogic.SignTx(vmTx, privKey, pubKey)
	contractAddrFaucet, _ := types.MakeContractAddress(userAddr, nonce)
	vmParam.contractAddr = contractAddrFaucet
	b3 := nextBlockWithTxs(b2, blockChain, vmTx)
	if err := calcRootHash(b2, b3, blockChain); err != nil {
		t.Fatal(err)
	}
	gasRefundValue := gasPrice*gasLimit - b3.Header.GasUsed
	t.Logf("b3 gas refund value: %d", gasRefundValue)
	contractBlockHandle(t, blockChain, b3, b3, vmParam, nil)
	t.Logf("faucet contract addr %s %x balance: %d", contractAddrFaucet,
		contractAddrFaucet.Hash(), blockChain.tailState.GetBalance(*contractAddrFaucet.Hash160()))
	bUserBalance := userBalance

	t.Logf("b3 block hash: %s", b3.BlockHash())
	t.Logf("b2 -> b3 passed, now tail height: %d", blockChain.LongestChainHeight)
	//
	if err := blockChainI.ProcessBlock(b3, core.DefaultMode, "peer1"); err != nil {
		t.Fatal(err)
	}

	// >>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
	// extend side chain: fork from b1
	// b0 -> b1 -> b2 -> b3
	//		          \ -> b3A
	vmValue, gasLimit = uint64(0), uint64(400000)
	vmParam = &testContractParam{
		vmValue, gasLimit, vmParam.contractBalance, 0, nil,
	}
	byteCode, _ = hex.DecodeString(testCoinContract)
	nonce = uint64(1)
	contractVout, _ = txlogic.MakeContractCreationVout(userAddr.Hash160(),
		vmValue, gasLimit, nonce)
	prevHash, _ = b2.Txs[1].TxHash()
	changeValueA := bAUserBalance - vmValue - gasPrice*gasLimit
	vmTx = types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(prevHash, 0), 0)).
		AppendVout(contractVout).
		AppendVout(txlogic.MakeVout(userAddr.Hash160(), changeValueA)).
		WithData(types.ContractDataType, byteCode)
	txlogic.SignTx(vmTx, privKey, pubKey)
	prevHash, _ = vmTx.TxHash()
	t.Logf("prevHash: %s", prevHash)
	contractAddrCoin, _ := types.MakeContractAddress(userAddr, nonce)
	vmParam.contractAddr = contractAddrCoin
	// split tx
	changeValueA -= core.TransferFee
	splitTx := types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(prevHash, 1), 0)).
		AppendVout(txlogic.MakeSplitAddrVout(addrs, weights)).
		AppendVout(txlogic.MakeVout(userAddr.Hash160(), changeValueA))
	txlogic.SignTx(splitTx, privKey, pubKey)
	prevHash, _ = splitTx.TxHash()
	splitAddr := txlogic.MakeSplitAddress(prevHash, 0, addrs, weights)
	logger.Infof("create a split tx. addr: %s", splitAddr)
	// b3A
	b3A := nextBlockWithTxs(b2, blockChain, vmTx, splitTx)
	if err := calcRootHash(b2, b3A, blockChainA); err != nil {
		t.Fatal(err)
	}
	contractBlockHandle(t, blockChain, b3A, b3, vmParam, core.ErrBlockInSideChain)
	t.Logf("b3A block hash: %s", b3A.BlockHash())
	t.Logf("b2 -> b3A failed: %s", core.ErrBlockInSideChain)
	gasRefundA := gasLimit*gasPrice - b3A.Header.GasUsed
	//
	if err := blockChainA.ProcessBlock(b3A, core.DefaultMode, "peer1"); err != nil {
		t.Fatal(err)
	}
	if err := blockChainB.ProcessBlock(b3A, core.DefaultMode, "peer1"); err != nil {
		t.Fatal(err)
	}

	// >>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
	// reorg: side chain grows longer than main chain
	// b0 -> b1 -> b2 -> b3
	//		           \-> b3A -> b4A
	//
	toSplitAmount := uint64(50000) + 2*core.TransferFee
	changeValueA = changeValueA - toSplitAmount - core.TransferFee
	toSplitTx := types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(prevHash, 1), 0)).
		AppendVout(txlogic.MakeVout(splitAddr.Hash160(), toSplitAmount)).
		AppendVout(txlogic.MakeVout(userAddr.Hash160(), changeValueA))
	txlogic.SignTx(toSplitTx, privKey, pubKey)
	toSplitTxHash, _ := toSplitTx.TxHash()
	t.Logf("toSplitTxHash: %s", toSplitTxHash)
	b4A := nextBlockWithTxsV2(b3A, blockChain, toSplitTx)
	//
	if err := calcRootHash(b3A, b4A, blockChainA); err != nil {
		t.Fatal(err)
	}
	if err := blockChainA.ProcessBlock(b4A, core.DefaultMode, "peer1"); err != nil {
		t.Fatal(err)
	}
	if err := blockChainB.ProcessBlock(b4A, core.DefaultMode, "peer1"); err != nil {
		t.Fatal(err)
	}
	//
	vmParam = &testContractParam{contractAddr: vmParam.contractAddr}
	userBalance = changeValueA + gasRefundA
	contractBlockHandle(t, blockChain, b4A, b4A, vmParam, nil)
	bAUserBalance = userBalance
	t.Logf("b4A block hash: %s", b4A.BlockHash())
	t.Logf("b3A -> b4A passed, now tail height: %d", blockChain.LongestChainHeight)

	// check split address balance
	blanceSplitA := getBalance(splitAddrA.Hash160(), blockChain.db)
	ensure.DeepEqual(t, blanceSplitA, toSplitAmount/2)
	blanceSplitB := getBalance(splitAddrB.Hash160(), blockChain.db)
	ensure.DeepEqual(t, blanceSplitB, toSplitAmount/2)

	//TODO: add insuffient balance check

	// >>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
	// Extend b3 fork twice to make first chain longer and force reorg
	// b0 -> b1 -> b2  -> b3  -> b4 -> b5
	// 		            \-> b3A -> b4A
	b4 := nextBlockV2(b3, blockChain)
	vmParam = &testContractParam{contractAddr: contractAddrFaucet}
	//
	if err := calcRootHash(b3, b4, blockChainI); err != nil {
		t.Fatal(err)
	}
	if err := blockChainI.ProcessBlock(b4, core.DefaultMode, "peer1"); err != nil {
		t.Fatal(err)
	}
	//
	userBalance = bAUserBalance
	contractBlockHandle(t, blockChain, b4, b4A, vmParam, core.ErrBlockInSideChain)
	t.Logf("b4 block hash: %s", b4.BlockHash())
	t.Logf("b3 -> b4 failed, now tail height: %d", blockChain.LongestChainHeight)
	// process b5
	vmValue, gasLimit = uint64(0), uint64(20000)
	contractBalance := uint64(10000 - 2000) // withdraw 2000, construct contract with 10000
	vmParam = &testContractParam{
		vmValue, gasLimit, contractBalance, 2000, contractAddrFaucet,
	}
	byteCode, _ = hex.DecodeString(testFaucetCall)
	nonce = 2
	contractVout, _ = txlogic.MakeContractCallVout(userAddr.Hash160(),
		contractAddrFaucet.Hash160(), vmValue, gasLimit, nonce)
	// use internal tx vout
	prevHash, _ = b3.InternalTxs[0].TxHash()
	changeValue5 := gasRefundValue - vmValue - gasPrice*gasLimit
	vmTx = types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(prevHash, 0), 0)).
		AppendVout(contractVout).
		AppendVout(txlogic.MakeVout(userAddr.Hash160(), changeValue5)).
		WithData(types.ContractDataType, byteCode)
	txlogic.SignTx(vmTx, privKey, pubKey)
	b5 := nextBlockWithTxsV2(b4, blockChain, vmTx)
	userBalance = bUserBalance
	//
	if err := calcRootHash(b4, b5, blockChainI); err != nil {
		t.Fatal(err)
	}
	if err := blockChainI.ProcessBlock(b5, core.DefaultMode, "peer1"); err != nil {
		t.Fatal(err)
	}
	//
	contractBlockHandle(t, blockChain, b5, b5, vmParam, nil)
	bUserBalance = userBalance
	t.Logf("b5 block hash: %s", b5.BlockHash())
	t.Logf("b4 -> b5 passed, now tail height: %d", blockChain.LongestChainHeight)

	// check balance
	blanceSplitA = getBalance(splitAddrA.Hash160(), blockChain.db)
	ensure.DeepEqual(t, blanceSplitA, uint64(0))
	blanceSplitB = getBalance(splitAddrB.Hash160(), blockChain.db)
	ensure.DeepEqual(t, blanceSplitB, uint64(0))

	// b0 -> b1 -> b2  -> b3  -> b4  -> b5
	//							 \ -> b3A -> b4A -> b5A -> b6A
	b5A := nextBlockV2(b4A, blockChain)
	vmParam = &testContractParam{contractAddr: contractAddrCoin, contractBalance: 8000}
	//
	if err := calcRootHash(b4A, b5A, blockChainA); err != nil {
		t.Fatal(err)
	}
	if err := blockChainA.ProcessBlock(b5A, core.DefaultMode, "peer1"); err != nil {
		t.Fatal(err)
	}
	if err := blockChainB.ProcessBlock(b5A, core.DefaultMode, "peer1"); err != nil {
		t.Fatal(err)
	}
	//
	contractBlockHandle(t, blockChain, b5A, b5, vmParam, core.ErrBlockInSideChain)
	t.Logf("b5A block hash: %s", b5A.BlockHash())
	t.Logf("b4A -> b5A failed, now tail height: %d", blockChain.LongestChainHeight)
	// b6A
	toUserAmount := uint64(80000)
	changeValueATemp := blockChainA.TailState().GetBalance(*minerAddr.Hash160()).Uint64() - toUserAmount
	prevHash, _ = b2.Txs[1].TxHash()
	toUserTx := types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(prevHash, 2), 0)).
		AppendVout(txlogic.MakeVout(userAddr.Hash160(), toUserAmount)).
		AppendVout(txlogic.MakeVout(minerAddr.Hash160(), changeValueATemp-core.TransferFee))
	txlogic.SignTx(toUserTx, privKeyMiner, pubKeyMiner)
	b6A := nextBlockWithTxsV2(b5A, blockChain, toUserTx)
	verifyProcessBlock(t, blockChain, b6A, core.ErrMissingTxOut, 5, b5)
	t.Logf("b5A -> b6A failed, now tail height: %d", blockChain.LongestChainHeight)

	vmValue, gasLimit = uint64(0), uint64(30000)
	vmParam = &testContractParam{
		vmValue, gasLimit, 0, 0, contractAddrCoin,
	}
	byteCode, _ = hex.DecodeString(mintCall)
	nonce = 2
	contractVout, _ = txlogic.MakeContractCallVout(userAddr.Hash160(),
		contractAddrCoin.Hash160(), vmValue, gasLimit, nonce)
	changeValueA = changeValueA - gasPrice*gasLimit
	// prev hash
	ensure.True(t, blockChainA.splitTxOutputs(toSplitTx))
	prevHash, _ = toSplitTx.TxHash()
	vmTx = types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(prevHash, 2), 0)).
		AppendVout(contractVout).
		AppendVout(txlogic.MakeVout(userAddr.Hash160(), changeValueA)).
		WithData(types.ContractDataType, byteCode)
	t.Logf("b6a change value: %d", changeValueA)
	txlogic.SignTx(vmTx, privKey, pubKey)
	b6A = nextBlockWithTxsV2(b5A, blockChain, vmTx)
	//
	if err := calcRootHash(b5A, b6A, blockChainA); err != nil {
		t.Fatal(err)
	}
	if err := blockChainA.ProcessBlock(b6A, core.DefaultMode, "peer1"); err != nil {
		t.Fatal(err)
	}
	if err := blockChainB.ProcessBlock(b6A, core.DefaultMode, "peer1"); err != nil {
		t.Fatal(err)
	}
	//
	userBalance = bAUserBalance
	contractBlockHandle(t, blockChain, b6A, b6A, vmParam, nil)
	bAUserBalance = userBalance
	b6AUserBalance := userBalance
	t.Logf("b6A block hash: %s", b6A.BlockHash())
	t.Logf("b5A -> b6A pass, now tail height: %d", blockChain.LongestChainHeight)
	// check balance
	blanceSplitA = getBalance(splitAddrA.Hash160(), blockChain.db)
	ensure.DeepEqual(t, blanceSplitA, toSplitAmount/2)
	blanceSplitB = getBalance(splitAddrB.Hash160(), blockChain.db)
	ensure.DeepEqual(t, blanceSplitB, toSplitAmount/2)

	// >>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
	// b0 -> b1 -> b2  -> b3  -> b4  -> b5
	// 		             -> b3A -> b4A -> b5A -> b6A -> b7A
	buf, err := blockChain.db.Get(SplitTxHashKey(toSplitTxHash))
	if err != nil || buf == nil {
		logger.Errorf("Failed to get split tx. Err: %v", err)
	}
	b4ASplitTx := new(types.Transaction)
	if err := b4ASplitTx.Unmarshal(buf); err != nil {
		logger.Errorf("Failed to Unmarshal split tx. Err: %v", err)
	}
	b4ASplitTxHash, _ := b4ASplitTx.TxHash()
	ensure.DeepEqual(t, prevHash, b4ASplitTxHash)
	b7ATx := types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(b4ASplitTxHash, 0), 0)).
		AppendVout(txlogic.MakeVout(userAddr.Hash160(), toSplitAmount/2-core.TransferFee))
	txlogic.SignTx(b7ATx, privKeySplitA, pubKeySplitA)
	b7A := nextBlockWithTxsV2(b6A, blockChain, b7ATx)
	userBalance = bAUserBalance + toSplitAmount/2 - core.TransferFee
	vmParam = &testContractParam{contractAddr: contractAddrCoin}
	//
	if err := calcRootHash(b6A, b7A, blockChainA); err != nil {
		t.Fatal(err)
	}
	if err := blockChainA.ProcessBlock(b7A, core.DefaultMode, "peer1"); err != nil {
		t.Fatal(err)
	}
	//
	contractBlockHandle(t, blockChain, b7A, b7A, vmParam, nil)
	bAUserBalance = userBalance
	t.Logf("b7A block hash: %s", b7A.BlockHash())
	t.Logf("b6A -> b7A pass, now tail height: %d", blockChain.LongestChainHeight)

	// check balance
	blanceSplitA = getBalance(splitAddrA.Hash160(), blockChain.db)
	ensure.DeepEqual(t, blanceSplitA, uint64(0))
	blanceSplitB = getBalance(splitAddrB.Hash160(), blockChain.db)
	ensure.DeepEqual(t, blanceSplitB, toSplitAmount/2)

	// >>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
	// force reorg split tx
	// b0 -> b1 -> b2  -> b3  -> b4  -> b5
	// 		             -> b3A -> b4A -> b5A -> b6A -> b7A
	//                                             -> b7B -> b8B
	vmValue, gasLimit = uint64(0), uint64(40000)
	vmParam = &testContractParam{
		vmValue, gasLimit, 0, 0, contractAddrCoin,
	}
	byteCode, _ = hex.DecodeString(sendCall)
	nonce = uint64(3)
	contractVout, _ = txlogic.MakeContractCallVout(userAddr.Hash160(),
		contractAddrCoin.Hash160(), vmValue, gasLimit, nonce)
	prevHash, _ = b6A.Txs[1].TxHash()
	changeValueA = changeValueA - vmValue - gasPrice*gasLimit
	vmTx = types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(prevHash, 1), 0)).
		AppendVout(contractVout).
		AppendVout(txlogic.MakeVout(userAddr.Hash160(), changeValueA)).
		WithData(types.ContractDataType, byteCode)
	t.Logf("b7b change value: %d", changeValueA)
	txlogic.SignTx(vmTx, privKey, pubKey)
	b7B := nextBlockWithTxsV2(b6A, blockChain, vmTx)
	//
	if err := calcRootHash(b6A, b7B, blockChainB); err != nil {
		t.Fatal(err)
	}
	if err := blockChainB.ProcessBlock(b7B, core.DefaultMode, "peer1"); err != nil {
		t.Fatal(err)
	}
	//
	userBalance = bAUserBalance
	contractBlockHandle(t, blockChain, b7B, b7A, vmParam, core.ErrBlockInSideChain)
	t.Logf("b7B block hash: %s", b7B.BlockHash())
	t.Logf("b6A -> b7B failed, now tail height: %d", blockChain.LongestChainHeight)

	b8B := nextBlockV2(b7B, blockChain)
	//
	if err := calcRootHash(b7B, b8B, blockChainB); err != nil {
		t.Fatal(err)
	}
	if err := blockChainB.ProcessBlock(b8B, core.DefaultMode, "peer1"); err != nil {
		t.Fatal(err)
	}
	//
	userBalance = blockChainB.TailState().GetBalance(*userAddr.Hash160()).Uint64()
	ensure.DeepEqual(t, userBalance, b6AUserBalance-b7B.Header.GasUsed)
	vmParam = &testContractParam{contractAddr: contractAddrCoin}
	contractBlockHandle(t, blockChain, b8B, b8B, vmParam, nil)
	t.Logf("b8B block hash: %s", b8B.BlockHash())
	t.Logf("b7B -> b8B pass, now tail height: %d", blockChain.LongestChainHeight)

	// check split balance
	blanceSplitA = getBalance(splitAddrA.Hash160(), blockChain.db)
	ensure.DeepEqual(t, blanceSplitA, toSplitAmount/2)
	blanceSplitB = getBalance(splitAddrB.Hash160(), blockChain.db)
	ensure.DeepEqual(t, blanceSplitB, toSplitAmount/2)

	// >>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
	// force reorg split tx
	// b0 -> b1 -> b2  -> b3  -> b4  -> b5  -> b6  -> b7  -> b8  -> b9
	// 		           -> b3A -> b4A -> b5A -> b6A -> b7A
	//                                             -> b7B -> b8B
	b6 := nextBlockV2(b5, blockChain)
	//
	if err := calcRootHash(b5, b6, blockChainI); err != nil {
		t.Fatal(err)
	}
	if err := blockChainI.ProcessBlock(b6, core.DefaultMode, "peer1"); err != nil {
		t.Fatal(err)
	}
	//
	t.Logf("b6 block hash: %s", b6.BlockHash())
	verifyProcessBlock(t, blockChain, b6, core.ErrBlockInSideChain, 8, b8B)

	b7 := nextBlockV2(b6, blockChain)
	//
	if err := calcRootHash(b6, b7, blockChainI); err != nil {
		t.Fatal(err)
	}
	if err := blockChainI.ProcessBlock(b7, core.DefaultMode, "peer1"); err != nil {
		t.Fatal(err)
	}
	//
	t.Logf("b7 block hash: %s", b7.BlockHash())
	verifyProcessBlock(t, blockChain, b7, core.ErrBlockInSideChain, 8, b8B)

	b8 := nextBlockV2(b7, blockChain)
	//
	if err := calcRootHash(b7, b8, blockChainI); err != nil {
		t.Fatal(err)
	}
	if err := blockChainI.ProcessBlock(b8, core.DefaultMode, "peer1"); err != nil {
		t.Fatal(err)
	}
	//
	t.Logf("b8 block hash: %s", b8.BlockHash())
	verifyProcessBlock(t, blockChain, b8, core.ErrBlockInSideChain, 8, b8B)

	vmValue, gasLimit = uint64(0), uint64(20000)
	//contractBalance = uint64(0) // withdraw 2000+9000, construct contract with 10000
	vmParam = &testContractParam{
		// vmValue, gasLimit, contractBalance, userRecv, contractAddr
		vmValue, gasLimit, 8000, 0, contractAddrFaucet,
	}
	byteCode, _ = hex.DecodeString(testFaucetCall2)
	nonce = 3
	contractVout, _ = txlogic.MakeContractCallVout(userAddr.Hash160(),
		contractAddrFaucet.Hash160(), vmValue, gasLimit, nonce)
	prevHash, _ = b3.Txs[1].TxHash()
	changeValue = changeValue - vmValue - gasPrice*gasLimit
	vmTx = types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(prevHash, 1), 0)).
		AppendVout(contractVout).
		AppendVout(txlogic.MakeVout(userAddr.Hash160(), changeValue)).
		WithData(types.ContractDataType, byteCode)
	txlogic.SignTx(vmTx, privKey, pubKey)
	b9 := nextBlockWithTxsV2(b8, blockChain, vmTx)
	//
	if err := calcRootHash(b8, b9, blockChainI); err != nil {
		t.Fatal(err)
	}
	if err := blockChainI.ProcessBlock(b9, core.DefaultMode, "peer1"); err != nil {
		t.Fatal(err)
	}
	//
	userBalance = bUserBalance
	contractBlockHandle(t, blockChain, b9, b9, vmParam, nil)
	t.Logf("b9 block hash: %s", b9.BlockHash())
	t.Logf("b8 -> b9 passed, now tail height: %d", blockChain.LongestChainHeight)

	// check balance
	blanceSplitA = getBalance(splitAddrA.Hash160(), blockChain.db)
	ensure.DeepEqual(t, blanceSplitA, uint64(0))
	blanceSplitB = getBalance(splitAddrB.Hash160(), blockChain.db)
	ensure.DeepEqual(t, blanceSplitB, uint64(0))

	// >>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
	// extend main chain
	// b9 -> b10
	b10 := nextBlockV2(b9, blockChain)
	if err := calcRootHash(b9, b10, blockChain); err != nil {
		t.Fatal(err)
	}
	verifyProcessBlock(t, blockChain, b10, nil, 10, b10)
	t.Logf("b9 -> b10 passed, now tail height: %d", blockChain.LongestChainHeight)

	b10DoubleMint := nextBlockV2(b10, blockChain)
	b10DoubleMint.Header.TimeStamp = b10.Header.TimeStamp
	b10DoubleMint.Header.BookKeeper = b10.Header.BookKeeper
	if err := calcRootHash(b10, b10DoubleMint, blockChain); err != nil {
		t.Fatal(err)
	}
	verifyProcessBlock(t, blockChain, b10DoubleMint, core.ErrRepeatedMintAtSameTime, 10, b10)

	// >>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
	// double spend check
	prevHash, _ = b9.Txs[1].TxHash()
	changeValueATemp = changeValue - toUserAmount
	toUserTx = types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(prevHash, 1), 0)).
		AppendVout(txlogic.MakeVout(userAddr.Hash160(), toUserAmount)).
		AppendVout(txlogic.MakeVout(userAddr.Hash160(), changeValueATemp))
	txlogic.SignTx(toUserTx, privKey, pubKey)
	splitTx = types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(prevHash, 1), 0)).
		AppendVout(txlogic.MakeSplitAddrVout(addrs, weights)).
		AppendVout(txlogic.MakeVout(userAddr.Hash160(), changeValue))
	txlogic.SignTx(splitTx, privKey, pubKey)
	d10ds := nextBlockWithTxsV2(b10, blockChain, toUserTx, splitTx)
	verifyProcessBlock(t, blockChain, d10ds, core.ErrDoubleSpendTx, 10, b10)
}
