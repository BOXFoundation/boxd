// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package chain

import (
	"bytes"
	"encoding/hex"
	"math/big"
	"strings"
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
	privBytesPre    = []byte{41, 251, 240, 17, 102, 252, 49, 201, 65, 202, 220, 22, 89, 165, 246, 132, 248, 28, 34, 193, 17, 62, 90, 165, 176, 175, 40, 183, 221, 69, 50, 105}

	privKeyMiner, pubKeyMiner, _ = crypto.KeyPairFromBytes(privBytesMiner)
	privKey, pubKey, _           = crypto.KeyPairFromBytes(privBytesUser)
	minerAddr, _                 = types.NewAddressFromPubKey(pubKeyMiner)
	scriptPubKeyMiner            = script.PayToPubKeyHashScript(minerAddr.Hash())
	userAddr, _                  = types.NewAddressFromPubKey(pubKey)
	scriptPubKeyUser             = script.PayToPubKeyHashScript(userAddr.Hash())

	privKeyPre, pubKeyPre, _ = crypto.KeyPairFromBytes(privBytesPre)
	preAddr, _               = types.NewAddressFromPubKey(pubKeyPre)

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
	// because contract address' utxoes may have many AddrUtxoKey,
	// only to leave contract type utxo and to skip others
	for _, k := range keys {
		ks := bytes.Split(k, []byte("/"))
		k3, k4 := ks[3], ks[4]
		if string(k4) != "0" {
			continue
		}
		if strings.HasSuffix(string(k3), "000000000000000000000000") {
			keys = [][]byte{k}
			break
		}
	}
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

// generate a child block with txs
func nextBlockV3(
	parent *types.Block, chain *BlockChain, nonce uint64, txs ...*types.Transaction,
) *types.Block {

	timestamp++
	newBlock := types.NewBlock(parent)
	newBlock.Header.BookKeeper = *minerAddr.Hash160()
	// tx fee
	txFee := uint64(0)
	for _, tx := range txs {
		if o, _ := txlogic.CheckAndGetContractVout(tx); o != nil {
			continue
		}
		txFee += core.TransferFee + tx.ExtraFee()
	}
	// coinbase tx
	height := newBlock.Header.Height
	adminAddr, _ := types.NewAddress(Admin)
	if nonce == 0 {
		nonce = chain.tailState.GetNonce(*adminAddr.Hash160())
	}
	coinbaseTx, _ := MakeCoinBaseContractTx(newBlock.Header.BookKeeper,
		CalcBlockSubsidy(height), txFee, nonce+1, height)
	//
	newBlock.Txs = append(newBlock.Txs, coinbaseTx)
	newBlock.Txs = append(newBlock.Txs, txs...)
	newBlock.Header.TimeStamp = timestamp
	newBlock.Header.Bloom = bloom.NewFilterWithMK(types.BloomBitLength, types.BloomHashNum)
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
	prevHash, _ := b2.Txs[2].TxHash()
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

	b3 := nextBlockV3(b2, blockChain, 0, txs...)
	if err := connectBlock(b2, b3, blockChain); err != nil {
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
	ensure.DeepEqual(t, balance, minerBalance+3*core.TransferFee)
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

	b1 := nextBlockV3(b0, blockChain, 0)
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

	b2 := nextBlockV3(b1, blockChain, 0, tx, vmTx)
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

func connectBlock(parent, block *types.Block, bc *BlockChain) error {
	logger.Info("================ start calc root hash ================")
	defer logger.Info("---------------- end calc root hash ----------------")

	bookkeeper := minerAddr.Hash160()
	statedb, err := state.New(&parent.Header.RootHash, &parent.Header.UtxoRoot, bc.DB())
	if err != nil {
		return err
	}
	genesisContractBalanceOld := statedb.GetBalance(ContractAddr).Uint64()
	logger.Infof("Before execute block %d statedb root: %s utxo root: %s genesis contract balance: %d",
		block.Header.Height, statedb.RootHash(), statedb.UtxoRoot(), genesisContractBalanceOld)

	utxoSet := NewUtxoSet()
	if err := utxoSet.LoadBlockUtxos(block, true, bc.DB()); err != nil {
		return err
	}
	blockCopy := block.Copy()
	bc.SplitBlockOutputs(blockCopy)
	if err := utxoSet.ApplyBlock(blockCopy, bc.IsContractAddr2); err != nil {
		return err
	}
	bc.UpdateNormalTxBalanceState(blockCopy, utxoSet, statedb)
	//
	reward := block.Txs[0].Vout[0].Value

	adminAddr, err := types.NewAddress(Admin)
	if err != nil {
		return err
	}
	statedb.AddBalance(*adminAddr.Hash160(), new(big.Int).SetUint64(block.Txs[0].Vout[0].Value))
	if len(block.Txs[0].Vout) == 2 {
		statedb.AddBalance(*bookkeeper, new(big.Int).SetUint64(block.Txs[0].Vout[1].Value))
	}
	//
	receipts, gasUsed, utxoTxs, err := bc.StateProcessor().Process(block, statedb, utxoSet)
	if err != nil {
		return err
	}
	logger.Infof("gas used for block %d: %d, bookkeeper %s reward: %d",
		block.Header.Height, gasUsed, block.Header.BookKeeper, reward+gasUsed)
	coinbaseHash, err := block.Txs[0].TxHash()
	if err != nil {
		return err
	}
	opCoinbase := types.NewOutPoint(coinbaseHash, 1)

	if gasUsed > 0 {
		statedb.AddBalance(*bookkeeper, big.NewInt(int64(gasUsed)))
		if len(block.Txs[0].Vout) == 2 {
			block.Txs[0].Vout[1].Value += gasUsed
		} else {
			block.Txs[0].AppendVout(txlogic.MakeVout(bookkeeper, gasUsed))
		}
		utxoSet.SpendUtxo(*opCoinbase)
		block.Txs[0].ResetTxHash()
		utxoSet.AddUtxo(block.Txs[0], 1, block.Header.Height)
		newHash, _ := block.Txs[0].TxHash()
		receipts[0].WithTxHash(newHash)
	}

	// apply internal txs.
	block.InternalTxs = utxoTxs
	if len(utxoTxs) > 0 {
		if err := utxoSet.ApplyInternalTxs(block); err != nil {
			return err
		}
	}
	if err := bc.UpdateContractUtxoState(statedb, utxoSet); err != nil {
		return err
	}

	root, utxoRoot, err := statedb.Commit(false)
	if err != nil {
		return err
	}

	logger.Infof("After execute block %d statedb root: %s utxo root: %s genesis contract balance: %d",
		block.Header.Height, statedb.RootHash(), statedb.UtxoRoot(), statedb.GetBalance(ContractAddr))
	logger.Infof("genesis contract balance change, previous %d, coinbase value: "+
		"%d, gas used %d, now %d in statedb", genesisContractBalanceOld,
		block.Txs[0].Vout[0].Value, gasUsed, statedb.GetBalance(ContractAddr))

	bc.UtxoSetCache()[block.Header.Height] = utxoSet

	block.Header.GasUsed = gasUsed
	block.Header.RootHash = *root

	txsRoot := CalcTxsHash(block.Txs)
	block.Header.TxsRoot = *txsRoot
	if len(utxoTxs) > 0 {
		internalTxsRoot := CalcTxsHash(utxoTxs)
		block.Header.InternalTxsRoot = *internalTxsRoot
	}
	if utxoRoot != nil {
		block.Header.UtxoRoot = *utxoRoot
	}
	block.Header.Bloom = types.CreateReceiptsBloom(receipts)
	if len(receipts) > 0 {
		block.Header.ReceiptHash = *receipts.Hash()
		bc.ReceiptsCache()[block.Header.Height] = receipts
	}
	block.Hash = nil
	logger.Infof("block %s height: %d have state root %s utxo root %s",
		block.BlockHash(), block.Header.Height, root, utxoRoot)
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
	prevHash, _ := b2.Txs[2].TxHash()
	changeValue := userBalance - vmValue - gasPrice*gasLimit
	vmTx := types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(prevHash, 0), 0)).
		AppendVout(contractVout).
		AppendVout(txlogic.MakeVout(userAddr.Hash160(), changeValue)).
		WithData(types.ContractDataType, byteCode)
	txlogic.SignTx(vmTx, privKey, pubKey)
	contractAddrFaucet, _ := types.MakeContractAddress(userAddr, nonce)
	vmParam.contractAddr = contractAddrFaucet
	b3 := nextBlockV3(b2, blockChain, 0, vmTx)
	if err := connectBlock(b2, b3, blockChain); err != nil {
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
	prevHash, _ = b2.Txs[2].TxHash()
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
	minerNonce := uint64(3)
	b3A := nextBlockV3(b2, blockChain, minerNonce, vmTx, splitTx)
	if err := connectBlock(b2, b3A, blockChainA); err != nil {
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
	minerBalance = blockChainA.tailState.GetBalance(*minerAddr.Hash160()).Uint64()
	toSplitAmount := uint64(50000) + 2*core.TransferFee
	changeValueA = changeValueA - toSplitAmount - core.TransferFee
	toSplitTx := types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(prevHash, 1), 0)).
		AppendVout(txlogic.MakeVout(splitAddr.Hash160(), toSplitAmount)).
		AppendVout(txlogic.MakeVout(userAddr.Hash160(), changeValueA))
	txlogic.SignTx(toSplitTx, privKey, pubKey)
	toSplitTxHash, _ := toSplitTx.TxHash()
	t.Logf("toSplitTxHash: %s", toSplitTxHash)
	b4A := nextBlockV3(b3A, blockChain, 0, toSplitTx)
	//
	if err := connectBlock(b3A, b4A, blockChainA); err != nil {
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
	minerNonce++
	b4 := nextBlockV3(b3, blockChain, minerNonce)
	vmParam = &testContractParam{contractAddr: contractAddrFaucet}
	//
	if err := connectBlock(b3, b4, blockChainI); err != nil {
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
	minerBalance = blockChainI.tailState.GetBalance(*minerAddr.Hash160()).Uint64()
	toContractAmount := uint64(1000)
	vmValue, gasLimit = uint64(0), uint64(20000)
	contractBalance := uint64(10000-2000) + toContractAmount // withdraw 2000, construct with 10000
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
	// send box to genesis contract address
	prevHash, _ = b3.Txs[1].TxHash()
	changeValue -= toContractAmount + core.TransferFee
	toContractTx := types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(prevHash, 1), 0)).
		AppendVout(txlogic.MakeVout(contractAddrFaucet.Hash160(), toContractAmount)).
		AppendVout(txlogic.MakeVout(userAddr.Hash160(), changeValue))
	txlogic.SignTx(toContractTx, privKey, pubKey)
	//
	b5 := nextBlockV3(b4, blockChain, 0, vmTx, toContractTx)
	userBalance = bUserBalance - toContractAmount - core.TransferFee
	//
	if err := connectBlock(b4, b5, blockChainI); err != nil {
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
	minerNonce++
	b5A := nextBlockV3(b4A, blockChain, minerNonce)
	vmParam = &testContractParam{contractAddr: contractAddrCoin, contractBalance: contractBalance}
	//
	if err := connectBlock(b4A, b5A, blockChainA); err != nil {
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
	minerBalance = blockChainA.tailState.GetBalance(*minerAddr.Hash160()).Uint64()
	toUserAmount := uint64(80000)
	changeValueATemp := blockChainA.TailState().GetBalance(*minerAddr.Hash160()).Uint64() - toUserAmount
	prevHash, _ = b2.Txs[1].TxHash()
	toUserTx := types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(prevHash, 2), 0)).
		AppendVout(txlogic.MakeVout(userAddr.Hash160(), toUserAmount)).
		AppendVout(txlogic.MakeVout(minerAddr.Hash160(), changeValueATemp-core.TransferFee))
	txlogic.SignTx(toUserTx, privKeyMiner, pubKeyMiner)
	b6A := nextBlockV3(b5A, blockChain, 0, toUserTx)
	if err := connectBlock(b5A, b6A, blockChainA); err != nil {
		t.Fatal(err)
	}
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
	b6A = nextBlockV3(b5A, blockChain, 0, vmTx)
	//
	if err := connectBlock(b5A, b6A, blockChainA); err != nil {
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
	b7A := nextBlockV3(b6A, blockChain, 0, b7ATx)
	userBalance = bAUserBalance + toSplitAmount/2 - core.TransferFee
	vmParam = &testContractParam{contractAddr: contractAddrCoin}
	//
	if err := connectBlock(b6A, b7A, blockChainA); err != nil {
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
	minerNonce = uint64(7)
	b7B := nextBlockV3(b6A, blockChain, minerNonce, vmTx)
	//
	if err := connectBlock(b6A, b7B, blockChainB); err != nil {
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

	minerBalance = blockChainB.tailState.GetBalance(*minerAddr.Hash160()).Uint64()
	b8B := nextBlockV3(b7B, blockChain, 0)
	//
	if err := connectBlock(b7B, b8B, blockChainB); err != nil {
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
	//  		           -> b3A -> b4A -> b5A -> b6A -> b7A
	//                                             -> b7B -> b8B
	minerNonce = uint64(6)
	b6 := nextBlockV3(b5, blockChain, minerNonce)
	//
	if err := connectBlock(b5, b6, blockChainI); err != nil {
		t.Fatal(err)
	}
	if err := blockChainI.ProcessBlock(b6, core.DefaultMode, "peer1"); err != nil {
		t.Fatal(err)
	}
	//
	t.Logf("b6 block hash: %s", b6.BlockHash())
	verifyProcessBlock(t, blockChain, b6, core.ErrBlockInSideChain, 8, b8B)

	minerNonce++
	b7 := nextBlockV3(b6, blockChain, minerNonce)
	//
	if err := connectBlock(b6, b7, blockChainI); err != nil {
		t.Fatal(err)
	}
	if err := blockChainI.ProcessBlock(b7, core.DefaultMode, "peer1"); err != nil {
		t.Fatal(err)
	}
	//
	t.Logf("b7 block hash: %s", b7.BlockHash())
	verifyProcessBlock(t, blockChain, b7, core.ErrBlockInSideChain, 8, b8B)

	minerNonce++
	b8 := nextBlockV3(b7, blockChain, minerNonce)
	//
	if err := connectBlock(b7, b8, blockChainI); err != nil {
		t.Fatal(err)
	}
	if err := blockChainI.ProcessBlock(b8, core.DefaultMode, "peer1"); err != nil {
		t.Fatal(err)
	}
	//
	t.Logf("b8 block hash: %s", b8.BlockHash())
	verifyProcessBlock(t, blockChain, b8, core.ErrBlockInSideChain, 8, b8B)

	minerBalance = blockChainI.tailState.GetBalance(*minerAddr.Hash160()).Uint64()
	vmValue, gasLimit = uint64(0), uint64(20000)
	//contractBalance = uint64(0) // withdraw 2000+9000, construct contract with 10000
	vmParam = &testContractParam{
		// vmValue, gasLimit, contractBalance, userRecv, contractAddr
		vmValue, gasLimit, contractBalance, 0, contractAddrFaucet,
	}
	byteCode, _ = hex.DecodeString(testFaucetCall2)
	nonce = 3
	contractVout, _ = txlogic.MakeContractCallVout(userAddr.Hash160(),
		contractAddrFaucet.Hash160(), vmValue, gasLimit, nonce)
	prevHash, _ = b5.Txs[2].TxHash()
	changeValue = changeValue - vmValue - gasPrice*gasLimit
	vmTx = types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(prevHash, 1), 0)).
		AppendVout(contractVout).
		AppendVout(txlogic.MakeVout(userAddr.Hash160(), changeValue)).
		WithData(types.ContractDataType, byteCode)
	txlogic.SignTx(vmTx, privKey, pubKey)
	b9 := nextBlockV3(b8, blockChain, 0, vmTx)
	//
	if err := connectBlock(b8, b9, blockChainI); err != nil {
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
	b10 := nextBlockV3(b9, blockChain, 0)
	if err := connectBlock(b9, b10, blockChain); err != nil {
		t.Fatal(err)
	}
	verifyProcessBlock(t, blockChain, b10, nil, 10, b10)
	t.Logf("b9 -> b10 passed, now tail height: %d", blockChain.LongestChainHeight)

	b10DoubleMint := nextBlockV3(b10, blockChain, 0)
	b10DoubleMint.Header.TimeStamp = b10.Header.TimeStamp
	b10DoubleMint.Header.BookKeeper = b10.Header.BookKeeper
	if err := connectBlock(b10, b10DoubleMint, blockChain); err != nil {
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
	d10ds := nextBlockV3(b10, blockChain, 0, toUserTx, splitTx)
	verifyProcessBlock(t, blockChain, d10ds, core.ErrDoubleSpendTx, 10, b10)
}
