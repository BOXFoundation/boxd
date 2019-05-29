// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package chain

import (
	"encoding/hex"
	"math/big"
	"strings"
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
	"github.com/ethereum/go-ethereum/accounts/abi"
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

func getTailBlock(blockChain *BlockChain) *types.Block {
	tailBlock, _ := blockChain.loadTailBlock()
	return tailBlock
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

// Test blockchain block processing
func TestBlockProcessing(t *testing.T) {
	blockChain := NewTestBlockChain()
	ensure.NotNil(t, blockChain)
	ensure.True(t, blockChain.LongestChainHeight == 0)

	b0 := getTailBlock(blockChain)

	// try to append an existing block: genesis block
	verifyProcessBlock(t, blockChain, b0, core.ErrBlockExists, 0, b0)

	// >>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
	// extend main chain
	// b0 -> b1
	b1 := nextBlock(b0)
	b1.Header.RootHash.SetString("c1fe380cc58ea1bfdb6bf400f925b8d28df9699018d99e03edb15f855cf334e6")
	verifyProcessBlock(t, blockChain, b1, nil, 1, b1)
	balance := getBalance(minerAddr.String(), blockChain.db)
	ensure.DeepEqual(t, balance, uint64(50*core.DuPerBox))

	b1DoubleMint := nextBlock(b1)
	b1DoubleMint.Header.TimeStamp = b1.Header.TimeStamp
	verifyProcessBlock(t, blockChain, b1DoubleMint, core.ErrRepeatedMintAtSameTime, 1, b1)
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
	verifyProcessBlock(t, blockChain, b2ds, core.ErrDoubleSpendTx, 1, b1)
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
	b2.Header.RootHash.SetString("03ccf8dd9752fdd049b2228352af7a2761f5227276d8be33c6c0444385e6ea70")
	verifyProcessBlock(t, blockChain, b2, nil, 2, b2)
	t.Logf("b2 block hash: %s", b2.BlockHash())

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
	b3.Header.RootHash.SetString("90feb7c1db004ec0ee129f7afbb82202569662627caa738e42666fc6e96ce255")
	verifyProcessBlock(t, blockChain, b3, nil, 3, b3)
	balance = getBalance(minerAddr.String(), blockChain.db)
	ensure.DeepEqual(t, balance, uint64(100*core.DuPerBox))
	t.Logf("b3 block hash: %s", b3.BlockHash())

	// >>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
	// extend side chain: fork from b1
	// b0 -> b1 -> b2 -> b3
	//		         \-> b3A
	b3A := nextBlock(b2)
	splitTx, splitAddr = createSplitTx(b2.Txs[0], 0)
	b3A.Txs = append(b3A.Txs, splitTx)
	b3A.Header.TxsRoot = *CalcTxsHash(b3A.Txs)
	b3A.Header.RootHash.SetString("90feb7c1db004ec0ee129f7afbb82202569662627caa738e42666fc6e96ce255")
	verifyProcessBlock(t, blockChain, b3A, core.ErrBlockInSideChain, 3, b3)
	t.Logf("b3A block hash: %s", b3A.BlockHash())

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
	b4A.Header.RootHash.SetString("0d2282d5a7f4e49e62eaaa267f04d3630241b8922e93456531078ec93834f6c8")
	t.Logf("b4A block hash: %s", b4A.BlockHash())
	verifyProcessBlock(t, blockChain, b4A, nil, 4, b4A)

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
	b4.Header.RootHash.SetString("51c3fbc31fb249e03f61c639ffc7520f5c01dbde33536423593bea2aaa0da095")
	t.Logf("b4 block hash: %s", b4.BlockHash())
	verifyProcessBlock(t, blockChain, b4, core.ErrBlockInSideChain, 4, b4A)
	b5 := nextBlock(b4)
	b5.Header.RootHash.SetString("33c885f0749832caf4fb92120d22045f54a7330849be906fd9c377624d84f14e")
	t.Logf("b5 block hash: %s", b5.BlockHash())
	verifyProcessBlock(t, blockChain, b5, nil, 5, b5)

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
	//							 \ -> b3A -> b4A -> b5A -> b6A
	// Tx: miner -> user: 50
	// Tx: miner -> split address: 50
	// Tx: miner -> user: 50
	// Tx: miner -> user: 50
	b5A := nextBlock(b4A)
	b5A.Txs = append(b5A.Txs, createGeneralTx(b4A.Txs[0], 0, 50*core.DuPerBox, userAddr.String(), privKeyMiner, pubKeyMiner))
	b5A.Header.TxsRoot = *CalcTxsHash(b5A.Txs)
	b5A.Header.RootHash.SetString("3e8994993e73bc5c0e0d0509d2b25f695fd8802aa2c7dd692f1efd0b7fe82677")
	t.Logf("b5A block hash: %s", b5A.BlockHash())
	verifyProcessBlock(t, blockChain, b5A, core.ErrBlockInSideChain, 5, b5)

	b6A := nextBlock(b5A)
	b6A.Txs = append(b6A.Txs, createGeneralTx(b3A.Txs[0], 0, 50*core.DuPerBox, userAddr.String(), privKeyMiner, pubKeyMiner))
	b6A.Header.TxsRoot = *CalcTxsHash(b6A.Txs)
	// reorg has happened
	b6A.Header.RootHash.SetString("a22718f2316c661afbb3e935c4a54d01f3840cbb17e4e1ae3567fbee31488a61")
	t.Logf("b6A block hash: %s", b6A.BlockHash())
	verifyProcessBlock(t, blockChain, b6A, core.ErrMissingTxOut, 5, b5A)

	b6A = nextBlock(b5A)
	b6A.Txs = append(b6A.Txs, createGeneralTx(b5A.Txs[0], 0, 50*core.DuPerBox, userAddr.String(), privKeyMiner, pubKeyMiner))
	b6A.Header.TxsRoot = *CalcTxsHash(b6A.Txs)
	b6A.Header.RootHash.SetString("a22718f2316c661afbb3e935c4a54d01f3840cbb17e4e1ae3567fbee31488a61")
	t.Logf("b6A block hash: %s", b6A.BlockHash())
	verifyProcessBlock(t, blockChain, b6A, nil, 6, b6A)

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
	b7ATx := createGeneralTx(b4ASplitTx, 0, 25*core.DuPerBox, userAddr.String(), privKeySplitA, pubKeySplitA)
	b7A.Txs = append(b7A.Txs, b7ATx)
	b7A.Header.TxsRoot = *CalcTxsHash(b7A.Txs)
	b7A.Header.RootHash.SetString("b4d2ea8f010720a088bbfef7c7d2ed6b96cf6502b6ec8f47a787095837b77417")
	t.Logf("b7A block hash: %s", b7A.BlockHash())
	verifyProcessBlock(t, blockChain, b7A, nil, 7, b7A)

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
	b7B.Header.RootHash.SetString("d2d031cf2c2fa1ff6d91d1d74fa58a1cf9a2e732f5a1928c9c3a1fec88d3c6d6")
	t.Logf("b7B block hash: %s", b7B.BlockHash())
	verifyProcessBlock(t, blockChain, b7B, core.ErrBlockInSideChain, 7, b7A)
	b8B := nextBlock(b7B)
	b8B.Header.RootHash.SetString("01986252cf2e0e78760bab02f837e773510d8c1dd930a62f352656934a81534c")
	t.Logf("b8B block hash: %s", b8B.BlockHash())
	verifyProcessBlock(t, blockChain, b8B, nil, 8, b8B)

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
	b6.Header.RootHash.SetString("59be773628a50bc3f0c63abc8378390eabf5571d5bd301391254c647393653fd")
	t.Logf("b6 block hash: %s", b6.BlockHash())
	verifyProcessBlock(t, blockChain, b6, core.ErrBlockInSideChain, 8, b8B)
	b7 := nextBlock(b6)
	b7.Header.RootHash.SetString("b5bff099dfb0ef9416fa1dfb6c7c0c89d0eafe3186e1d3911b1f4dbbb768e6e7")
	t.Logf("b7 block hash: %s", b7.BlockHash())
	verifyProcessBlock(t, blockChain, b7, core.ErrBlockInSideChain, 8, b8B)
	b8 := nextBlock(b7)
	b8.Header.RootHash.SetString("00494e2ee75a07b21c58ba9c58084e06f48b08bc9d292925828daee1824d10e9")
	t.Logf("b8 block hash: %s", b8.BlockHash())
	verifyProcessBlock(t, blockChain, b8, core.ErrBlockInSideChain, 8, b8B)
	b9 := nextBlock(b8)
	b9.Header.RootHash.SetString("03c7d4a41dc98f0a053ebb156c8f1b3339e86ed1f56a31984fa96ecae48ada61")
	t.Logf("b9 block hash: %s", b9.BlockHash())
	verifyProcessBlock(t, blockChain, b9, nil, 9, b9)

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
	txlogic.SignTx(tx, privKeyMiner, pubKeyMiner)
	txHash, _ := tx.TxHash()

	prevHash, _ = b1.Txs[0].TxHash()
	gasUsed, vmValue, gasPrice, gasLimit, nonce := uint64(9912), uint64(0), uint64(6), uint64(20000), uint64(2)
	contractVout, _ := txlogic.MakeContractCreationVout(vmValue, gasLimit, gasPrice, nonce, []byte{1})
	vmTx := types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(prevHash, 0), 0)).
		AppendVout(contractVout)
	txlogic.SignTx(vmTx, privKeyMiner, pubKeyMiner)
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

func createSplitTx(parentTx *types.Transaction, index uint32) (*types.Transaction, string) {

	vIn := txlogic.MakeVinForTest(parentTx, index)
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

	if err := txlogic.SignTx(tx, privKeyMiner, pubKeyMiner); err != nil {
		logger.Errorf("Failed to sign tx. Err: %v", err)
		return nil, ""
	}
	logger.Infof("create a split tx. addr: %s", addr)
	return tx, addr
}

func createGeneralTx(parentTx *types.Transaction, index uint32, value uint64,
	address string, privKey *crypto.PrivateKey, pubKey *crypto.PublicKey) *types.Transaction {
	vIn := txlogic.MakeVinForTest(parentTx, index)
	txOut := txlogic.MakeVout(address, value)
	vOut := []*corepb.TxOut{txOut}
	tx := &types.Transaction{
		Vin:  vIn,
		Vout: vOut,
	}
	if err := txlogic.SignTx(tx, privKey, pubKey); err != nil {
		logger.Errorf("Failed to sign tx. Err: %v", err)
		return nil
	}
	return tx
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

const (
	testExtractPrevHash = "c0e96e998eb01eea5d5acdaeb80acd943477e6119dcd82a419089331229c7453"
	// contract Temp {
	//     function () payable {}
	// }
	testVMScriptCode = "6060604052346000575b60398060166000396000f30060606040525b600b5b5b565b0000a165627a7a723058209cedb722bf57a30e3eb00eeefc392103ea791a2001deed29f5c3809ff10eb1dd0029"

	/*
			pragma solidity ^0.5.1;
			contract Faucet {
		    // Give out ether to anyone who asks
		    function withdraw(uint withdraw_amount) public {
		        // Limit withdrawal amount
		        require(withdraw_amount <= 10000);
		        // Send the amount to the address that requested it
		        msg.sender.transfer(withdraw_amount);
		    }
		    // Accept any incoming amount
		    function () external payable  {}
		    // Create a new ballot with $(_numProposals) different proposals.
		    constructor() public payable {}
			}
	*/
	testFaucetContract = "608060405260f7806100126000396000f3fe6080604052600436106039576000357c0100000000000000000000000000000000000000000000000000000000900480632e1a7d4d14603b575b005b348015604657600080fd5b50607060048036036020811015605b57600080fd5b81019080803590602001909291905050506072565b005b6127108111151515608257600080fd5b3373ffffffffffffffffffffffffffffffffffffffff166108fc829081150290604051600060405180830381858888f1935050505015801560c7573d6000803e3d6000fd5b505056fea165627a7a7230582041951f9857bb67cda6bccbb59f6fdbf38eeddc244530e577d8cad6194941d38c0029"
	// withdraw 2000
	testFaucetCall = "2e1a7d4d00000000000000000000000000000000000000000000000000000000000007d0"
	// withdraw 9000
	testFaucetCall2 = "2e1a7d4d0000000000000000000000000000000000000000000000000000000000002328"
)

func _TestExtractBoxTx(t *testing.T) {
	var tests = []struct {
		value        uint64
		addrStr      string
		code         string
		price, limit uint64
		nonce        uint64
		version      int32
		err          error
	}{
		{100, "b1YMx5kufN2qELzKaoaBWzks2MZknYqqPnh", testVMScriptCode, 100, 20000, 1, 0, nil},
		{0, "", testVMScriptCode, 100, 20000, 2, 0, nil},
	}
	blockChain := NewTestBlockChain()
	for _, tc := range tests {
		var addr types.Address
		if tc.addrStr != "" {
			addr, _ = types.NewAddress(tc.addrStr)
		}
		code, _ := hex.DecodeString(tc.code)
		cs, err := script.MakeContractScriptPubkey(addr, code, tc.price, tc.limit, tc.nonce, tc.version)
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
			btx.Gas() != tc.limit || btx.Nonce() != tc.nonce ||
			btx.Version() != tc.version {
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
	newBlock.Txs = append(newBlock.Txs, coinbaseTx)
	newBlock.Txs = append(newBlock.Txs, txs...)
	newBlock.Header.TxsRoot = *CalcTxsHash(newBlock.Txs)
	newBlock.Header.TimeStamp = timestamp
	return newBlock
}

var (
	userBalance, minerBalance, contractBalance uint64
)

type testContractParam struct {
	gasUsed, vmValue, gasPrice, gasLimit, contractBalance, userRecv uint64

	contractAddr *types.AddressContract
}

func genTestChain(t *testing.T, blockChain *BlockChain) *types.Block {
	b0 := getTailBlock(blockChain)
	// >>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
	// extend main chain
	// b0 -> b1
	b1 := nextBlock(b0)
	b1.Header.RootHash.SetString("c1fe380cc58ea1bfdb6bf400f925b8d28df9699018d99e03edb15f855cf334e6")
	verifyProcessBlock(t, blockChain, b1, nil, 1, b1)
	balance := getBalance(minerAddr.String(), blockChain.db)
	stateBalance, _ := blockChain.GetBalance(minerAddr)
	ensure.DeepEqual(t, balance, stateBalance)
	ensure.DeepEqual(t, balance, BaseSubsidy)
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
		AppendVout(txlogic.MakeVout(minerAddr.String(), BaseSubsidy-userBalance))
	err := txlogic.SignTx(tx, privKeyMiner, pubKeyMiner)
	ensure.DeepEqual(t, err, nil)

	b2 := nextBlockWithTxs(b1, tx)
	b2.Header.RootHash.SetString("97057e28c8faf3dc1343890faa4aaf65765c6ea726eb0125d9188c202bb8a3d7")
	verifyProcessBlock(t, blockChain, b2, nil, 2, b2)
	// check balance
	// for userAddr
	balance = getBalance(userAddr.String(), blockChain.db)
	stateBalance, _ = blockChain.GetBalance(userAddr)
	ensure.DeepEqual(t, balance, stateBalance)
	ensure.DeepEqual(t, balance, userBalance)
	t.Logf("user balance: %d", userBalance)
	// for miner
	balance = getBalance(minerAddr.String(), blockChain.db)
	stateBalance, _ = blockChain.GetBalance(minerAddr)
	ensure.DeepEqual(t, balance, stateBalance)
	ensure.DeepEqual(t, balance, 2*BaseSubsidy-userBalance)
	minerBalance = balance
	t.Logf("b1 -> b2 passed, now tail height: %d", blockChain.LongestChainHeight)
	return b2
}

func contractBlockHandle(
	t *testing.T, blockChain *BlockChain, parent, block *types.Block,
	param *testContractParam, err error, internalTxs ...*types.Transaction,
) {

	gasCost := param.gasUsed * param.gasPrice
	if len(internalTxs) > 0 {
		block.InternalTxs = append(block.InternalTxs, internalTxs...)
		block.Header.InternalTxsRoot.SetBytes(CalcTxsHash(block.InternalTxs)[:])
	}
	block.Header.GasUsed = param.gasUsed
	block.Txs[0].Vout[0].Value += gasCost
	tailBlock := block
	expectUserBalance := userBalance - param.vmValue - gasCost + param.userRecv
	//t.Logf("expectUserBalance: %d, userBalance: %d, vmValue: %d, gasCost: %d",
	//	expectUserBalance, userBalance, param.vmValue, gasCost)
	expectMinerBalance := minerBalance + BaseSubsidy + gasCost
	if err != nil && err == errInsufficientBalanceForGas {
		tailBlock = parent
		expectUserBalance, expectMinerBalance = userBalance, minerBalance
	}
	height := tailBlock.Header.Height
	verifyProcessBlock(t, blockChain, block, err, height, tailBlock)
	// check balance
	// for userAddr
	balance := getBalance(userAddr.String(), blockChain.db)
	stateBalance, _ := blockChain.GetBalance(userAddr)
	t.Logf("user %s balance: %d, stateBalance: %d, expect balance: %d",
		userAddr, balance, stateBalance, expectUserBalance)
	ensure.DeepEqual(t, balance, stateBalance)
	ensure.DeepEqual(t, balance, expectUserBalance)
	userBalance = balance
	t.Logf("user balance: %d", userBalance)
	// for miner
	balance = getBalance(minerAddr.String(), blockChain.db)
	stateBalance, _ = blockChain.GetBalance(minerAddr)
	t.Logf("miner %s balance: %d, stateBalance: %d, expect balance: %d",
		minerAddr, balance, stateBalance, expectMinerBalance)
	ensure.DeepEqual(t, balance, stateBalance)
	ensure.DeepEqual(t, balance, expectMinerBalance)
	minerBalance = balance
	t.Logf("miner balance: %d", minerBalance)
	// for contract address
	balance = getBalance(param.contractAddr.String(), blockChain.db)
	stateBalance, _ = blockChain.GetBalance(param.contractAddr)
	ensure.DeepEqual(t, balance, stateBalance)
	ensure.DeepEqual(t, stateBalance, param.contractBalance)
	contractBalance = stateBalance
	t.Logf("contract address %s balance: %d", param.contractAddr, contractBalance)
}

func TestFaucetContract(t *testing.T) {
	blockChain := NewTestBlockChain()
	ensure.NotNil(t, blockChain)
	ensure.True(t, blockChain.LongestChainHeight == 0)

	// contract blocks test
	b2 := genTestChain(t, blockChain)

	// >>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
	// extend main chain
	// b2 -> b3
	// make creation contract tx
	gasUsed, vmValue, gasPrice, gasLimit := uint64(56160), uint64(10000), uint64(10), uint64(200000)
	vmParam := &testContractParam{
		// gasUsed, vmValue, gasPrice, gasLimit, contractBalance, userRecv, contractAddr
		gasUsed, vmValue, gasPrice, gasLimit, vmValue, 0, nil,
	}

	byteCode, _ := hex.DecodeString(testFaucetContract)
	nonce := uint64(1)
	contractVout, err := txlogic.MakeContractCreationVout(vmValue, gasLimit, gasPrice, nonce, byteCode)
	ensure.Nil(t, err)
	prevHash, _ := b2.Txs[1].TxHash()
	changeValue2 := userBalance - vmValue - gasPrice*gasLimit
	vmTx := types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(prevHash, 0), 0)).
		AppendVout(contractVout).
		AppendVout(txlogic.MakeVout(userAddr.String(), changeValue2))
	txlogic.SignTx(vmTx, privKey, pubKey)
	vmTxHash, _ := vmTx.TxHash()
	t.Logf("vmTx hash: %s", vmTxHash)
	stateDB, err := state.New(&b2.Header.RootHash, nil, blockChain.db)
	ensure.Nil(t, err)
	contractAddr, _ := types.MakeContractAddress(userAddr, nonce)
	vmParam.contractAddr = contractAddr
	t.Logf("contract address: %s", contractAddr)
	gasRefundTx := createGasRefundUtxoTx(userAddr.Hash160(), gasPrice*(gasLimit-gasUsed), nonce)
	b3 := nextBlockWithTxs(b2, vmTx)
	b3.Header.RootHash.SetString("7a999efa9fcc0d58b3a0e0959ef40cfef8df019a7945d7157033d24435690e83")
	b3.Header.UtxoRoot.SetString("7422e52db1ec4d3d2943064d0b6e8471c32aeb79b27fa485377abf9b1bf09703")
	receipt := types.NewReceipt(vmTxHash, contractAddr.Hash160(), false, gasUsed).WithTxIndex(1)
	b3.Header.ReceiptHash = *(new(types.Receipts).Append(receipt).Hash())
	contractBlockHandle(t, blockChain, b2, b3, vmParam, nil, gasRefundTx)
	stateDB, err = state.New(&b3.Header.RootHash, &b3.Header.UtxoRoot, blockChain.db)
	ensure.Nil(t, err)
	ensure.DeepEqual(t, stateDB.GetNonce(*userAddr.Hash160()), nonce)

	gasRefundValue := vmParam.gasPrice * (vmParam.gasLimit - vmParam.gasUsed)
	t.Logf("b2 -> b3 passed, now tail height: %d", blockChain.LongestChainHeight)

	// >>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
	// extend main chain
	// b3 -> b4
	// make call contract tx
	gasUsed, vmValue, gasPrice, gasLimit = uint64(9912), uint64(0), uint64(6), uint64(20000)
	contractBalance := uint64(10000 - 2000) // withdraw 2000, construct contract with 10000
	vmParam = &testContractParam{
		// gasUsed, vmValue, gasPrice, gasLimit, contractBalance, userRecv, contractAddr
		gasUsed, vmValue, gasPrice, gasLimit, contractBalance, 2000, contractAddr,
	}
	byteCode, _ = hex.DecodeString(testFaucetCall)
	nonce++
	contractVout, err = txlogic.MakeContractCallVout(contractAddr.String(),
		vmValue, gasLimit, gasPrice, nonce, byteCode)
	ensure.Nil(t, err)
	// use internal tx vout
	prevHash, _ = b3.InternalTxs[0].TxHash()
	changeValue3 := gasRefundValue - vmValue - gasPrice*gasLimit
	vmTx = types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(prevHash, 0), 0)).
		AppendVout(contractVout).
		AppendVout(txlogic.MakeVout(userAddr.String(), changeValue3))
	gasRefundTx = createGasRefundUtxoTx(userAddr.Hash160(), gasPrice*(gasLimit-gasUsed), nonce)
	op := types.NewOutPoint(types.NormalizeAddressHash(contractAddr.Hash160()), 0)
	contractTx := types.NewTx(0, 0, 0).
		AppendVin(txlogic.MakeContractVin(op, 0)).
		AppendVout(txlogic.MakeVout(userAddr.String(), 2000))
	txlogic.SignTx(vmTx, privKey, pubKey)
	vmTxHash, _ = vmTx.TxHash()
	t.Logf("vmTx hash: %s", vmTxHash)
	b4 := nextBlockWithTxs(b3, vmTx)
	b4.Header.RootHash.SetString("af3defef7e1a7fe8866ba1e14be3a567920a04bae7a77bdbf4095f9a0ecd6f3f")
	b4.Header.UtxoRoot.SetString("c319d99073a9395610b5dbd42814efd5a5e1ae8be837e3cb898eabb382e4dde9")
	receipt = types.NewReceipt(vmTxHash, contractAddr.Hash160(), false, gasUsed).WithTxIndex(1)
	b4.Header.ReceiptHash = *(new(types.Receipts).Append(receipt).Hash())
	contractBlockHandle(t, blockChain, b3, b4, vmParam, nil, gasRefundTx, contractTx)
	stateDB, err = state.New(&b4.Header.RootHash, &b4.Header.UtxoRoot, blockChain.db)
	ensure.Nil(t, err)
	ensure.DeepEqual(t, stateDB.GetNonce(*userAddr.Hash160()), nonce)
	t.Logf("b3 -> b4 passed, now tail height: %d", blockChain.LongestChainHeight)

	// >>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
	// extend main chain
	// b4 -> b5
	// make creation contract tx with insufficient gas
	gasUsed, vmValue, gasPrice, gasLimit = uint64(20000), uint64(1000), uint64(10), uint64(20000)
	vmParam = &testContractParam{
		// gasUsed, vmValue, gasPrice, gasLimit, contractBalance, userRecv, contractAddr
		gasUsed, vmValue, gasPrice, gasLimit, contractBalance, vmValue, vmParam.contractAddr,
	}
	byteCode, _ = hex.DecodeString(testFaucetContract)
	nonce++
	contractVout, err = txlogic.MakeContractCreationVout(vmValue, gasLimit, gasPrice, nonce, byteCode)
	ensure.Nil(t, err)
	prevHash, _ = b3.Txs[1].TxHash()
	changeValue4 := changeValue2 - vmValue - gasPrice*gasLimit
	vmTx = types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(prevHash, 1), 0)).
		AppendVout(contractVout).
		AppendVout(txlogic.MakeVout(userAddr.String(), changeValue4))
	txlogic.SignTx(vmTx, privKey, pubKey)
	// make refund tx
	contractAddrHash := types.CreateAddress(*userAddr.Hash160(), nonce)
	refundTx := types.NewTx(0, 0, 0).
		AppendVin(txlogic.MakeContractVin(
			types.NewOutPoint(types.NormalizeAddressHash(contractAddrHash), 0), 0)).
		AppendVout(txlogic.MakeVout(userAddr.String(), vmValue))
	//
	b5 := nextBlockWithTxs(b4, vmTx)
	b5.Header.RootHash.SetString("0bf64507600e191311f40c2ad178be827e6e43eafbd053d9c7210da1b7a35dd8")
	b5.Header.UtxoRoot.SetString("02a60bc676518aff8d62e0555093c0d150298d6f7ed54f2eb96a5b625cac20ee")
	vmTxHash, _ = vmTx.TxHash()
	receipt = types.NewReceipt(vmTxHash, contractAddr.Hash160(), true, gasUsed).WithTxIndex(1)
	b5.Header.ReceiptHash = *(new(types.Receipts).Append(receipt).Hash())
	contractBlockHandle(t, blockChain, b4, b5, vmParam, nil, refundTx)
	stateDB, err = state.New(&b5.Header.RootHash, &b5.Header.UtxoRoot, blockChain.db)
	ensure.Nil(t, err)
	ensure.DeepEqual(t, stateDB.GetNonce(*userAddr.Hash160()), nonce)
	t.Logf("b4 -> b5 passed, now tail height: %d", blockChain.LongestChainHeight)

	// >>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
	// extend main chain
	// b5 -> b6
	// make creation contract tx with insufficient balance
	gasUsed, vmValue, gasPrice, gasLimit = uint64(20000), uint64(1000), uint64(10), uint64(600000)
	vmParam = &testContractParam{
		// gasUsed, vmValue, gasPrice, gasLimit, contractBalance, userRecv, contractAddr
		gasUsed, vmValue, gasPrice, gasLimit, contractBalance, vmValue, vmParam.contractAddr,
	}
	byteCode, _ = hex.DecodeString(testFaucetContract)
	nonce++
	contractVout, err = txlogic.MakeContractCreationVout(vmValue, gasLimit, gasPrice, nonce, byteCode)
	ensure.Nil(t, err)
	prevHash, _ = b5.InternalTxs[0].TxHash()
	vmTx = types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(prevHash, 0), 0)).
		AppendVout(contractVout)
	txlogic.SignTx(vmTx, privKey, pubKey)
	b6 := nextBlockWithTxs(b5, vmTx)
	b6.Header.RootHash.SetString("")
	b6.Header.UtxoRoot.SetString("")
	contractBlockHandle(t, blockChain, b5, b6, vmParam, errInsufficientBalanceForGas, refundTx)
	nonce--
	t.Logf("b5 -> b6 failed, now tail height: %d", blockChain.LongestChainHeight)

	// >>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
	// extend main chain
	// b5 -> b6
	// make call contract tx with insufficient contract balance
	gasUsed, vmValue, gasPrice, gasLimit = uint64(9914), uint64(0), uint64(6), uint64(20000)
	contractBalance = uint64(0) // withdraw 2000+9000, construct contract with 10000
	vmParam = &testContractParam{
		// gasUsed, vmValue, gasPrice, gasLimit, contractBalance, userRecv, contractAddr
		gasUsed, vmValue, gasPrice, gasLimit, 8000, 0, contractAddr,
	}
	byteCode, _ = hex.DecodeString(testFaucetCall2)
	nonce++
	contractVout, err = txlogic.MakeContractCallVout(contractAddr.String(),
		vmValue, gasLimit, gasPrice, nonce, byteCode)
	ensure.Nil(t, err)
	// use internal tx vout
	prevHash, _ = b5.Txs[1].TxHash()
	changeValue6 := changeValue4 - vmValue - gasPrice*gasLimit
	logger.Warnf("utxo value: %d, gas: %d", changeValue4, gasPrice*gasLimit)
	vmTx = types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(prevHash, 1), 0)).
		AppendVout(contractVout).
		AppendVout(txlogic.MakeVout(userAddr.String(), changeValue6))
	gasRefundTx = createGasRefundUtxoTx(userAddr.Hash160(), gasPrice*(gasLimit-gasUsed), nonce)
	op = types.NewOutPoint(types.NormalizeAddressHash(contractAddr.Hash160()), 0)
	txlogic.SignTx(vmTx, privKey, pubKey)
	vmTxHash, _ = vmTx.TxHash()
	t.Logf("vmTx hash: %s", vmTxHash)
	b6 = nextBlockWithTxs(b5, vmTx)
	b6.Header.RootHash.SetString("b955aba83fd8773d33dab64bb961c2c72cc095fc0529a41984f4b4c39c05b54b")
	b6.Header.UtxoRoot.SetString("02a60bc676518aff8d62e0555093c0d150298d6f7ed54f2eb96a5b625cac20ee")
	receipt = types.NewReceipt(vmTxHash, contractAddr.Hash160(), true, gasUsed).WithTxIndex(1)
	b6.Header.ReceiptHash = *(new(types.Receipts).Append(receipt).Hash())
	contractBlockHandle(t, blockChain, b5, b6, vmParam, nil, gasRefundTx)
	stateDB, err = state.New(&b6.Header.RootHash, &b6.Header.UtxoRoot, blockChain.db)
	ensure.Nil(t, err)
	ensure.DeepEqual(t, stateDB.GetNonce(*userAddr.Hash160()), nonce)
	t.Logf("b5 -> b6 passed, now tail height: %d", blockChain.LongestChainHeight)
}

const (
	/*
		pragma solidity ^0.5.6;  //The lowest compiler version

		contract Coin {
		    // The keyword "public" makes those variables
		    // readable from outside.
		    address public minter;
		    mapping (address => uint) public balances;

		    // Events allow light clients to react on
		    // changes efficiently.
		    event Sent(address from, address to, uint amount);

		    // This is the constructor whose code is
		    // run only when the contract is created.
		    constructor() public {
		        minter = msg.sender;
		    }

		    function mint(address receiver, uint amount) public {
		        if (msg.sender != minter) return;
		        balances[receiver] += amount;
		    }

		    function send(address receiver, uint amount) public {
		        if (balances[msg.sender] < amount) return ;
		        balances[msg.sender] -= amount;
		        balances[receiver] += amount;
		        emit Sent(msg.sender, receiver, amount);
		    }
		}
	*/
	testCoinContract = "608060405234801561001057600080fd5b50336000806101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff16021790555061042d806100606000396000f3fe608060405234801561001057600080fd5b506004361061004c5760003560e01c8063075461721461005157806327e235e31461009b57806340c10f19146100f3578063d0679d3414610141575b600080fd5b61005961018f565b604051808273ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200191505060405180910390f35b6100dd600480360360208110156100b157600080fd5b81019080803573ffffffffffffffffffffffffffffffffffffffff1690602001909291905050506101b4565b6040518082815260200191505060405180910390f35b61013f6004803603604081101561010957600080fd5b81019080803573ffffffffffffffffffffffffffffffffffffffff169060200190929190803590602001909291905050506101cc565b005b61018d6004803603604081101561015757600080fd5b81019080803573ffffffffffffffffffffffffffffffffffffffff16906020019092919080359060200190929190505050610277565b005b6000809054906101000a900473ffffffffffffffffffffffffffffffffffffffff1681565b60016020528060005260406000206000915090505481565b6000809054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff163373ffffffffffffffffffffffffffffffffffffffff161461022557610273565b80600160008473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020600082825401925050819055505b5050565b80600160003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019081526020016000205410156102c3576103fd565b80600160003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019081526020016000206000828254039250508190555080600160008473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020600082825401925050819055507f3990db2d31862302a685e8086b5755072a6e2b5b780af1ee81ece35ee3cd3345338383604051808473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020018373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001828152602001935050505060405180910390a15b505056fea165627a7a723058200cf7e1d90be79a04377bc832a4cd9b545f25e8253d7c83b1c72529f73c0888c60029"
	coinAbi          = `[{"constant":true,"inputs":[],"name":"minter","outputs":[{"name":"","type":"address"}],"payable":false,"stateMutability":"view","type":"function"},{"constant":true,"inputs":[{"name":"","type":"address"}],"name":"balances","outputs":[{"name":"","type":"uint256"}],"payable":false,"stateMutability":"view","type":"function"},{"constant":false,"inputs":[{"name":"receiver","type":"address"},{"name":"amount","type":"uint256"}],"name":"mint","outputs":[],"payable":false,"stateMutability":"nonpayable","type":"function"},{"constant":false,"inputs":[{"name":"receiver","type":"address"},{"name":"amount","type":"uint256"}],"name":"send","outputs":[],"payable":false,"stateMutability":"nonpayable","type":"function"},{"inputs":[],"payable":false,"stateMutability":"nonpayable","type":"constructor"},{"anonymous":false,"inputs":[{"indexed":false,"name":"from","type":"address"},{"indexed":false,"name":"to","type":"address"},{"indexed":false,"name":"amount","type":"uint256"}],"name":"Sent","type":"event"}]`
)

func TestCoinContract(t *testing.T) {

	var mintCall, sendCall, balancesUserCall, balancesReceiverCall string
	// balances
	receiver, err := types.NewContractAddress("b5WYphc4yBPH18gyFthS1bHyRcEvM6xANuT")
	if err != nil {
		t.Fatal(err)
	}
	func() {
		abiObj, err := abi.JSON(strings.NewReader(coinAbi))
		if err != nil {
			t.Fatal(err)
		}
		// mint 8000000
		//toAddress := types.BytesToAddressHash([]byte("andone"))
		input, err := abiObj.Pack("mint", *userAddr.Hash160(), big.NewInt(8000000))
		//input, err := abiObj.Pack("mint", toAddress, big.NewInt(8000000))
		if err != nil {
			t.Fatal(err)
		}
		mintCall = hex.EncodeToString(input)
		t.Logf("mint 8000000: %s", mintCall)
		// sent 2000000
		input, err = abiObj.Pack("send", *receiver.Hash160(), big.NewInt(2000000))
		if err != nil {
			t.Fatal(err)
		}
		sendCall = hex.EncodeToString(input)
		t.Logf("send 2000000: %s", sendCall)
		// balances user addr
		input, err = abiObj.Pack("balances", *userAddr.Hash160())
		if err != nil {
			t.Fatal(err)
		}
		balancesUserCall = hex.EncodeToString(input)
		t.Logf("balancesUser: %s", balancesUserCall)
		// balances test Addr
		input, err = abiObj.Pack("balances", receiver.Hash160())
		if err != nil {
			t.Fatal(err)
		}
		balancesReceiverCall = hex.EncodeToString(input)
		t.Logf("balances %s: %s", receiver, balancesReceiverCall)
	}()

	blockChain := NewTestBlockChain()
	// blockchain
	ensure.NotNil(t, blockChain)
	ensure.True(t, blockChain.LongestChainHeight == 0)
	// contract blocks test
	b2 := genTestChain(t, blockChain)
	// >>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
	// extend main chain
	// b2 -> b3
	// make creation contract tx
	gasUsed, vmValue, gasPrice, gasLimit := uint64(246403), uint64(0), uint64(10), uint64(400000)
	vmParam := &testContractParam{
		// gasUsed, vmValue, gasPrice, gasLimit, contractBalance, userRecv, contractAddr
		gasUsed, vmValue, gasPrice, gasLimit, vmValue, 0, nil,
	}
	byteCode, _ := hex.DecodeString(testCoinContract)
	nonce := uint64(1)
	contractVout, err := txlogic.MakeContractCreationVout(vmValue, gasLimit, gasPrice, nonce, byteCode)
	ensure.Nil(t, err)
	prevHash, _ := b2.Txs[1].TxHash()
	changeValue := userBalance - vmValue - gasPrice*gasLimit
	vmTx := types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(prevHash, 0), 0)).
		AppendVout(contractVout).
		AppendVout(txlogic.MakeVout(userAddr.String(), changeValue))
	txlogic.SignTx(vmTx, privKey, pubKey)
	contractAddr, _ := types.MakeContractAddress(userAddr, nonce)
	vmParam.contractAddr = contractAddr
	t.Logf("contract address: %s", contractAddr)
	// TODO
	gasRefundTx := createGasRefundUtxoTx(userAddr.Hash160(), gasPrice*(gasLimit-gasUsed), nonce)
	b3 := nextBlockWithTxs(b2, vmTx)
	b3.Header.RootHash.SetString("e64182ce966ec345c08a8f139331e7b96c3cf6069c5e2f55a080a0061fa09ae1")
	utxoRootHash := "15bc283003801d99c320ba35e0e9e0246a7e7fadfc20b2c7737e4a0f81ee2d6b"
	b3.Header.UtxoRoot.SetString(utxoRootHash)
	vmTxHash, _ := vmTx.TxHash()
	receipt := types.NewReceipt(vmTxHash, contractAddr.Hash160(), false, gasUsed).WithTxIndex(1)
	b3.Header.ReceiptHash = *(new(types.Receipts).Append(receipt).Hash())
	contractBlockHandle(t, blockChain, b2, b3, vmParam, nil, gasRefundTx)
	stateDB, err := state.New(&b3.Header.RootHash, &b3.Header.UtxoRoot, blockChain.db)
	ensure.Nil(t, err)
	t.Logf("user nonce: %d", stateDB.GetNonce(*userAddr.Hash160()))
	//refundValue := vmParam.gasPrice * (vmParam.gasLimit - vmParam.gasUsed)
	t.Logf("b2 -> b3 passed, now tail height: %d", blockChain.LongestChainHeight)

	// >>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
	// extend main chain
	// b3 -> b4
	// make mint 8000000 call contract tx
	gasUsed, vmValue, gasPrice, gasLimit = uint64(23177), uint64(0), uint64(6), uint64(30000)
	vmParam = &testContractParam{
		// gasUsed, vmValue, gasPrice, gasLimit, contractBalance, userRecv, contractAddr
		gasUsed, vmValue, gasPrice, gasLimit, 0, 0, contractAddr,
	}
	byteCode, _ = hex.DecodeString(mintCall)
	nonce++
	contractVout, err = txlogic.MakeContractCallVout(contractAddr.String(),
		vmValue, gasLimit, gasPrice, nonce, byteCode)
	ensure.Nil(t, err)
	prevHash, _ = b3.Txs[1].TxHash()
	changeValue = changeValue - vmValue - gasPrice*gasLimit
	vmTx = types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(prevHash, 1), 0)).
		AppendVout(contractVout).
		AppendVout(txlogic.MakeVout(userAddr.String(), changeValue))
	txlogic.SignTx(vmTx, privKey, pubKey)
	gasRefundTx = createGasRefundUtxoTx(userAddr.Hash160(), gasPrice*(gasLimit-gasUsed), nonce)
	b4 := nextBlockWithTxs(b3, vmTx)
	b4.Header.RootHash.SetString("74094671ea59c16d215cb0797f51d8ded5a1e0c8932bddb82b2a86f7b9366fe6")
	b4.Header.UtxoRoot.SetString(utxoRootHash)
	vmTxHash, _ = vmTx.TxHash()
	receipt = types.NewReceipt(vmTxHash, contractAddr.Hash160(), false, gasUsed).WithTxIndex(1)
	b4.Header.ReceiptHash = *(new(types.Receipts).Append(receipt).Hash())
	contractBlockHandle(t, blockChain, b3, b4, vmParam, nil, gasRefundTx)
	stateDB, err = state.New(&b4.Header.RootHash, &b4.Header.UtxoRoot, blockChain.db)
	ensure.Nil(t, err)
	t.Logf("user nonce: %d", stateDB.GetNonce(*userAddr.Hash160()))
	t.Logf("b3 -> b4 passed, now tail height: %d", blockChain.LongestChainHeight)

	// >>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
	// extend main chain
	// b4 -> b5
	// make send 2000000 call contract tx
	gasUsed, vmValue, gasPrice, gasLimit = uint64(30219), uint64(0), uint64(6), uint64(40000)
	vmParam = &testContractParam{
		// gasUsed, vmValue, gasPrice, gasLimit, contractBalance, userRecv, contractAddr
		gasUsed, vmValue, gasPrice, gasLimit, 0, 0, contractAddr,
	}
	byteCode, _ = hex.DecodeString(sendCall)
	nonce++
	contractVout, err = txlogic.MakeContractCallVout(contractAddr.String(),
		vmValue, gasLimit, gasPrice, nonce, byteCode)
	ensure.Nil(t, err)
	// use internal tx vout
	prevHash, _ = b4.Txs[1].TxHash()
	changeValue = changeValue - vmValue - gasPrice*gasLimit
	vmTx = types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(prevHash, 1), 0)).
		AppendVout(contractVout).
		AppendVout(txlogic.MakeVout(userAddr.String(), changeValue))
	txlogic.SignTx(vmTx, privKey, pubKey)
	gasRefundTx = createGasRefundUtxoTx(userAddr.Hash160(), gasPrice*(gasLimit-gasUsed), nonce)
	b5 := nextBlockWithTxs(b4, vmTx)
	b5.Header.RootHash.SetString("aa0c32471602b461406f808a27d0f911f4e440ff62280c5c059b52a556bd5130")
	b5.Header.UtxoRoot.SetString(utxoRootHash)
	vmTxHash, _ = vmTx.TxHash()
	receipt = types.NewReceipt(vmTxHash, contractAddr.Hash160(), false, gasUsed).WithTxIndex(1)
	b5.Header.ReceiptHash = *(new(types.Receipts).Append(receipt).Hash())
	contractBlockHandle(t, blockChain, b4, b5, vmParam, nil, gasRefundTx)
	stateDB, err = state.New(&b5.Header.RootHash, &b5.Header.UtxoRoot, blockChain.db)
	ensure.Nil(t, err)
	t.Logf("user nonce: %d", stateDB.GetNonce(*userAddr.Hash160()))
	t.Logf("b4 -> b5 passed, now tail height: %d", blockChain.LongestChainHeight)

	// >>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
	// extend main chain
	// b5 -> b6
	// make balances user call contract tx
	gasUsed, vmValue, gasPrice, gasLimit = uint64(2825), uint64(0), uint64(6), uint64(4000)
	vmParam = &testContractParam{
		// gasUsed, vmValue, gasPrice, gasLimit, contractBalance, userRecv, contractAddr
		gasUsed, vmValue, gasPrice, gasLimit, 0, 0, contractAddr,
	}
	byteCode, _ = hex.DecodeString(balancesUserCall)
	nonce++
	contractVout, err = txlogic.MakeContractCallVout(contractAddr.String(),
		vmValue, gasLimit, gasPrice, nonce, byteCode)
	ensure.Nil(t, err)
	// use internal tx vout
	prevHash, _ = b5.Txs[1].TxHash()
	changeValue = changeValue - vmValue - gasPrice*gasLimit
	vmTx = types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(prevHash, 1), 0)).
		AppendVout(contractVout).
		AppendVout(txlogic.MakeVout(userAddr.String(), changeValue))
	txlogic.SignTx(vmTx, privKey, pubKey)
	gasRefundTx = createGasRefundUtxoTx(userAddr.Hash160(), gasPrice*(gasLimit-gasUsed), nonce)
	b6 := nextBlockWithTxs(b5, vmTx)
	b6.Header.RootHash.SetString("404aa027eb404b7d3608ccc4f87f7e866916cf4ef646176088e68ad63b0e8779")
	b6.Header.UtxoRoot.SetString(utxoRootHash)
	vmTxHash, _ = vmTx.TxHash()
	receipt = types.NewReceipt(vmTxHash, contractAddr.Hash160(), false, gasUsed).WithTxIndex(1)
	b6.Header.ReceiptHash = *(new(types.Receipts).Append(receipt).Hash())
	contractBlockHandle(t, blockChain, b5, b6, vmParam, nil, gasRefundTx)
	stateDB, err = state.New(&b5.Header.RootHash, &b5.Header.UtxoRoot, blockChain.db)
	ensure.Nil(t, err)
	t.Logf("user nonce: %d", stateDB.GetNonce(*userAddr.Hash160()))
	t.Logf("b5 -> b6 passed, now tail height: %d", blockChain.LongestChainHeight)
	// check balances
	// return 0x5b8d80 = 6000000, check okay

	// >>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
	// extend main chain
	// b6 -> b7
	// make balances receiver call contract tx
	gasUsed, vmValue, gasPrice, gasLimit = uint64(2825), uint64(0), uint64(6), uint64(4000)
	vmParam = &testContractParam{
		// gasUsed, vmValue, gasPrice, gasLimit, contractBalance, userRecv, contractAddr
		gasUsed, vmValue, gasPrice, gasLimit, 0, 0, contractAddr,
	}
	byteCode, _ = hex.DecodeString(balancesReceiverCall)
	nonce++
	contractVout, err = txlogic.MakeContractCallVout(contractAddr.String(),
		vmValue, gasLimit, gasPrice, nonce, byteCode)
	ensure.Nil(t, err)
	// use internal tx vout
	prevHash, _ = b6.Txs[1].TxHash()
	changeValue = changeValue - vmValue - gasPrice*gasLimit
	vmTx = types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(prevHash, 1), 0)).
		AppendVout(contractVout).
		AppendVout(txlogic.MakeVout(userAddr.String(), changeValue))
	txlogic.SignTx(vmTx, privKey, pubKey)
	gasRefundTx = createGasRefundUtxoTx(userAddr.Hash160(), gasPrice*(gasLimit-gasUsed), nonce)
	gasRefundTxHash, _ := gasRefundTx.TxHash()
	t.Logf("refund tx: %s", gasRefundTxHash)
	b7 := nextBlockWithTxs(b6, vmTx)
	b7.Header.RootHash.SetString("a0589aeda39379bc4a5e3c368ea43c3acbd5e5395af9c60bc890826d5a28ae5d")
	b7.Header.UtxoRoot.SetString(utxoRootHash)
	vmTxHash, _ = vmTx.TxHash()
	receipt = types.NewReceipt(vmTxHash, contractAddr.Hash160(), false, gasUsed).WithTxIndex(1)
	b7.Header.ReceiptHash = *(new(types.Receipts).Append(receipt).Hash())
	contractBlockHandle(t, blockChain, b6, b7, vmParam, nil, gasRefundTx)
	stateDB, err = state.New(&b7.Header.RootHash, &b7.Header.UtxoRoot, blockChain.db)
	ensure.Nil(t, err)
	t.Logf("user nonce: %d", stateDB.GetNonce(*userAddr.Hash160()))
	t.Logf("b6 -> b7 passed, now tail height: %d", blockChain.LongestChainHeight)
	// check balances
	// return 0x1e8480 = 2000000, check okay
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
	err := txlogic.SignTx(tx, privKey, pubKey)
	ensure.DeepEqual(t, err, nil)
	txs = append(txs, tx)
	// tx2
	prevHash, _ = tx.TxHash()
	t.Logf("tx1: %s", prevHash)
	tx = types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(prevHash, 1), 0)).
		AppendVout(txlogic.MakeVout(splitAddrA.String(), 2000000)).
		AppendVout(txlogic.MakeVout(userAddr.String(), 3000000))
	err = txlogic.SignTx(tx, privKey, pubKey)
	ensure.DeepEqual(t, err, nil)
	txs = append(txs, tx)
	// tx3
	prevHash, _ = tx.TxHash()
	t.Logf("tx2: %s", prevHash)
	tx = types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(prevHash, 1), 0)).
		AppendVout(txlogic.MakeVout(splitAddrA.String(), 2500000)).
		AppendVout(txlogic.MakeVout(userAddr.String(), 500000))
	err = txlogic.SignTx(tx, privKey, pubKey)
	ensure.DeepEqual(t, err, nil)
	txs = append(txs, tx)

	b3 := nextBlockWithTxs(b2, txs...)
	b3.Header.RootHash.SetString("f8f8a8687f0c1cbb4013c5cc7b3971ff93feb262ab5201906aed82976ff3f714")
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
