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
	b1.Header.RootHash.SetString("21f2a68960cdc2eb60910e0c80a8d61aa44ae8ce58e38a7bd13d0d24c7d89341")
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
	b2.Header.RootHash.SetString("bb69b01a6533fc0cad78a352aa554aea6f3a0fe390f523c1910742e50925d2af")
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
	b3.Header.RootHash.SetString("eda5ad0d1f613344be6fbddaed11386d0e6deb1e429ff0bb0daa4fbddc49a9a2")
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
	b3A.Header.RootHash.SetString("eda5ad0d1f613344be6fbddaed11386d0e6deb1e429ff0bb0daa4fbddc49a9a2")
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
	b4A.Header.RootHash.SetString("543f8bd7dc6de47023b6a25e940c2a55c2c86827de22abc359ded17a30fd9054")
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
	b4.Header.RootHash.SetString("e3064212a29fe1f77b6729e95d83034812c2e1038b37ff553a5ebc829ffc0878")
	t.Logf("b4 block hash: %s", b4.BlockHash())
	verifyProcessBlock(t, blockChain, b4, core.ErrBlockInSideChain, 4, b4A)
	b5 := nextBlock(b4)
	b5.Header.RootHash.SetString("56210d5e1322e2511c9ac65221bf3a5f61fb071ac5b6e82edc9066fc5effe3f7")
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
	// 		           -> b3A -> b4A -> b5A -> b6A
	// Tx: miner -> user: 50
	// Tx: miner -> split address: 50
	// Tx: miner -> user: 50
	// Tx: miner -> user: 50
	b5A := nextBlock(b4A)
	b5A.Txs = append(b5A.Txs, createGeneralTx(b4A.Txs[0], 0, 50*core.DuPerBox, userAddr.String(), privKeyMiner, pubKeyMiner))
	b5A.Header.TxsRoot = *CalcTxsHash(b5A.Txs)
	b5A.Header.RootHash.SetString("cacec7a6dc8783b967911741ae825ec272e79f67cdcd8a11aae0b6a512f0478f")
	t.Logf("b5A block hash: %s", b5A.BlockHash())
	verifyProcessBlock(t, blockChain, b5A, core.ErrBlockInSideChain, 5, b5)

	b6A := nextBlock(b5A)
	b6A.Txs = append(b6A.Txs, createGeneralTx(b3A.Txs[0], 0, 50*core.DuPerBox, userAddr.String(), privKeyMiner, pubKeyMiner))
	b6A.Header.TxsRoot = *CalcTxsHash(b6A.Txs)
	// reorg has happened
	b6A.Header.RootHash.SetString("cacec7a6dc8783b967911741ae825ec272e79f67cdcd8a11aae0b6a512f0478f")
	t.Logf("b6A block hash: %s", b6A.BlockHash())
	verifyProcessBlock(t, blockChain, b6A, core.ErrMissingTxOut, 5, b5A)

	b6A = nextBlock(b5A)
	b6A.Txs = append(b6A.Txs, createGeneralTx(b5A.Txs[0], 0, 50*core.DuPerBox, userAddr.String(), privKeyMiner, pubKeyMiner))
	b6A.Header.TxsRoot = *CalcTxsHash(b6A.Txs)
	b6A.Header.RootHash.SetString("ad6f0fb9f02e2410869ba1f9403627e5155aa175ae612daab899c32c6c437d2c")
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
	logger.Infof("b4ASplitTx: %v", b4ASplitTx)
	b7ATx := createGeneralTx(b4ASplitTx, 0, 25*core.DuPerBox, userAddr.String(), privKeySplitA, pubKeySplitA)
	b7A.Txs = append(b7A.Txs, b7ATx)
	b7A.Header.TxsRoot = *CalcTxsHash(b7A.Txs)
	b7A.Header.RootHash.SetString("3886134dfe459cf0dc9851c121bea380e2ef799f308d46d1708625cbd17d417e")
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
	b7B.Header.RootHash.SetString("114123f3fd8a170693a1319533d447c812cf1823f548d7f25416b8064a0168ed")
	t.Logf("b7B block hash: %s", b7B.BlockHash())
	verifyProcessBlock(t, blockChain, b7B, core.ErrBlockInSideChain, 7, b7A)
	b8B := nextBlock(b7B)
	b8B.Header.RootHash.SetString("e709f664ec4af99fe2fc24da282eae259452a2cf0d9cd443b7b9809f01b572d4")
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
	b6.Header.RootHash.SetString("4d9f26b6aee6e22a4c1955b8b6bfb945b8f9bd59fb670e94b605533f772e0604")
	t.Logf("b6 block hash: %s", b6.BlockHash())
	verifyProcessBlock(t, blockChain, b6, core.ErrBlockInSideChain, 8, b8B)
	b7 := nextBlock(b6)
	b7.Header.RootHash.SetString("64209a7f9a2b2950eb06c87575be42983bf87944c95279e328edc24042e3e988")
	t.Logf("b7 block hash: %s", b7.BlockHash())
	verifyProcessBlock(t, blockChain, b7, core.ErrBlockInSideChain, 8, b8B)
	b8 := nextBlock(b7)
	b8.Header.RootHash.SetString("157f575985a57702b6327ea9042462a635321a2ab5fb490eaa54f5146c22dcd7")
	t.Logf("b8 block hash: %s", b8.BlockHash())
	verifyProcessBlock(t, blockChain, b8, core.ErrBlockInSideChain, 8, b8B)
	b9 := nextBlock(b8)
	b9.Header.RootHash.SetString("33dfd0f709a7ec504cb135705cae978426f2d5745de8196199ecd6921bfcc1f5")
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
	ensure.Nil(t, blockChain.StoreBlockWithStateInBatch(b1, blockChain.db))

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
	blockChain := NewTestBlockChain()
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
	gasUsed, vmValue, gasPrice, gasLimit, contractBalance, userRecv uint64

	contractAddr *types.AddressContract
}

func genTestChain(t *testing.T, blockChain *BlockChain) *types.Block {
	b0 := getTailBlock(blockChain)
	// >>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
	// extend main chain
	// b0 -> b1
	b1 := nextBlock(b0)
	b1.Header.RootHash.SetString("21f2a68960cdc2eb60910e0c80a8d61aa44ae8ce58e38a7bd13d0d24c7d89341")
	verifyProcessBlock(t, blockChain, b1, nil, 1, b1)
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
	b2.Header.RootHash.SetString("09d69c41cf7131cc6a8c48ce2c47e8e569a85ae38935031a440a0a16ba35f119")
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
	ensure.DeepEqual(t, balance, 2*testBlockSubsidy-userBalance)
	minerBalance = balance
	t.Logf("b1 -> b2 passed, now tail height: %d", blockChain.LongestChainHeight)
	return b2
}

func contractBlockHandle(
	t *testing.T, blockChain *BlockChain, vmTx *types.Transaction, parent *types.Block,
	rootHashStr, utxoRootHashStr string, param *testContractParam, err error,
	internalTxs ...*types.Transaction,
) *types.Block {

	block := nextBlockWithTxs(parent, vmTx)
	block.Header.RootHash.SetString(rootHashStr)
	block.Header.UtxoRoot.SetString(utxoRootHashStr)
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
	expectMinerBalance := minerBalance + testBlockSubsidy + gasCost
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

	return block
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
	contractVout, err := txlogic.MakeContractCreationVout(vmValue, gasLimit, gasPrice, byteCode)
	ensure.Nil(t, err)
	prevHash, _ := b2.Txs[1].TxHash()
	changeValue2 := userBalance - vmValue - gasPrice*gasLimit
	vmTx := types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(prevHash, 0), 0)).
		AppendVout(contractVout).
		AppendVout(txlogic.MakeVout(userAddr.String(), changeValue2))
	signTx(vmTx, privKey, pubKey)
	vmTxHash, _ := vmTx.TxHash()
	t.Logf("vmTx hash: %s", vmTxHash)
	stateDB, err := state.New(&b2.Header.RootHash, nil, blockChain.db)
	ensure.Nil(t, err)
	nonce := stateDB.GetNonce(*userAddr.Hash160())
	t.Logf("user nonce: %d", nonce)
	contractAddr, _ := types.MakeContractAddress(userAddr, nonce)
	vmParam.contractAddr = contractAddr
	t.Logf("contract address: %s", contractAddr)
	gasRefundTx := createGasRefundUtxoTx(userAddr.Hash160(), gasPrice*(gasLimit-gasUsed), nonce+1)
	rootHashStr := "894681fa5a51e9ec2472aec04178d18420aef31184814392e9e560430079f468"
	utxoRootHashStr := "cba64a2ed674666b2bc3079d2744b9b52c2e8bb230b21ae1e14502a98ba3b401"
	b3 := contractBlockHandle(t, blockChain, vmTx, b2, rootHashStr, utxoRootHashStr,
		vmParam, nil, gasRefundTx)
	stateDB, err = state.New(&b3.Header.RootHash, &b3.Header.UtxoRoot, blockChain.db)
	ensure.Nil(t, err)
	nonce = stateDB.GetNonce(*userAddr.Hash160())
	t.Logf("user nonce: %d", nonce)

	gasRefundValue := vmParam.gasPrice * (vmParam.gasLimit - vmParam.gasUsed)
	t.Logf("b2 -> b3 passed, now tail height: %d", blockChain.LongestChainHeight)

	// >>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
	// extend main chain
	// b3 -> b4
	// make call contract tx
	gasUsed, vmValue, gasPrice, gasLimit = uint64(9912), uint64(0), uint64(6), uint64(20000)
	contractBalance := uint64(10000 - 2000) // with draw 2000, construct contract with 10000
	vmParam = &testContractParam{
		// gasUsed, vmValue, gasPrice, gasLimit, contractBalance, userRecv, contractAddr
		gasUsed, vmValue, gasPrice, gasLimit, contractBalance, 2000, contractAddr,
	}
	byteCode, _ = hex.DecodeString(testFaucetCall)
	contractVout, err = txlogic.MakeContractCallVout(contractAddr.String(),
		vmValue, gasLimit, gasPrice, byteCode)
	ensure.Nil(t, err)
	// use internal tx vout
	prevHash, _ = b3.InternalTxs[0].TxHash()
	changeValue3 := gasRefundValue - vmValue - gasPrice*gasLimit
	vmTx = types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(prevHash, 0), 0)).
		AppendVout(contractVout).
		AppendVout(txlogic.MakeVout(userAddr.String(), changeValue3))
	gasRefundTx = createGasRefundUtxoTx(userAddr.Hash160(), gasPrice*(gasLimit-gasUsed), nonce+1)
	op := types.NewOutPoint(types.NormalizeAddressHash(contractAddr.Hash160()), 0)
	contractTx := types.NewTx(0, 0, 0).
		AppendVin(txlogic.MakeContractVin(op, 0)).
		AppendVout(txlogic.MakeVout(userAddr.String(), 2000))
	signTx(vmTx, privKey, pubKey)
	vmTxHash, _ = vmTx.TxHash()
	t.Logf("vmTx hash: %s", vmTxHash)
	rootHashStr = "ad7ab3e72e6dc3c1c6fd708ac178ee714bf72fe9f9862c2d679984eee9fcd284"
	utxoRootHashStr = "e1549fe32c8b508d29da9d170c20c8753b8712183817b64e94004afc4f0b35a6"
	b4 := contractBlockHandle(t, blockChain, vmTx, b3, rootHashStr, utxoRootHashStr,
		vmParam, nil, gasRefundTx, contractTx)
	stateDB, err = state.New(&b4.Header.RootHash, &b4.Header.UtxoRoot, blockChain.db)
	ensure.Nil(t, err)
	nonce = stateDB.GetNonce(*userAddr.Hash160())
	t.Logf("user nonce: %d", nonce)
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
	contractVout, err = txlogic.MakeContractCreationVout(vmValue, gasLimit, gasPrice, byteCode)
	ensure.Nil(t, err)
	prevHash, _ = b3.Txs[1].TxHash()
	changeValue4 := changeValue2 - vmValue - gasPrice*gasLimit
	vmTx = types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(prevHash, 1), 0)).
		AppendVout(contractVout).
		AppendVout(txlogic.MakeVout(userAddr.String(), changeValue4))
	signTx(vmTx, privKey, pubKey)
	rootHashStr = "b98dadc162f1fccfff29efc6faf79eea3c1f305d53e8f18289216b691a783227"
	utxoRootHashStr = "3f45d74900973b7b4703f00ec0097859864374536c27162cea1ddcc865555c29"
	// make refund tx
	contractAddrHash := types.CreateAddress(*userAddr.Hash160(), stateDB.GetNonce(*userAddr.Hash160()))
	refundTx := types.NewTx(0, 0, 0).
		AppendVin(txlogic.MakeContractVin(
			types.NewOutPoint(types.NormalizeAddressHash(&contractAddrHash), 0), 0)).
		AppendVout(txlogic.MakeVout(userAddr.String(), vmValue))
	//
	b5 := contractBlockHandle(t, blockChain, vmTx, b4, rootHashStr, utxoRootHashStr,
		vmParam, nil, refundTx)
	stateDB, err = state.New(&b5.Header.RootHash, &b5.Header.UtxoRoot, blockChain.db)
	ensure.Nil(t, err)
	nonce = stateDB.GetNonce(*userAddr.Hash160())
	t.Logf("user nonce: %d", nonce)
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
	contractVout, err = txlogic.MakeContractCreationVout(vmValue, gasLimit, gasPrice, byteCode)
	ensure.Nil(t, err)
	prevHash, _ = b5.InternalTxs[0].TxHash()
	vmTx = types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(prevHash, 0), 0)).
		AppendVout(contractVout)
	signTx(vmTx, privKey, pubKey)
	b6 := contractBlockHandle(t, blockChain, vmTx, b5, rootHashStr, utxoRootHashStr,
		vmParam, errInsufficientBalanceForGas, refundTx)
	stateDB, err = state.New(&b6.Header.RootHash, &b6.Header.UtxoRoot, blockChain.db)
	ensure.Nil(t, err)
	nonce = stateDB.GetNonce(*userAddr.Hash160())
	t.Logf("user nonce: %d", nonce)
	t.Logf("b5 -> b6 failed, now tail height: %d", blockChain.LongestChainHeight)
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
	contractVout, err := txlogic.MakeContractCreationVout(vmValue, gasLimit, gasPrice, byteCode)
	ensure.Nil(t, err)
	prevHash, _ := b2.Txs[1].TxHash()
	changeValue := userBalance - vmValue - gasPrice*gasLimit
	vmTx := types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(prevHash, 0), 0)).
		AppendVout(contractVout).
		AppendVout(txlogic.MakeVout(userAddr.String(), changeValue))
	signTx(vmTx, privKey, pubKey)
	stateDB, err := state.New(&b2.Header.RootHash, &b2.Header.UtxoRoot, blockChain.db)
	ensure.Nil(t, err)
	nonce := stateDB.GetNonce(*userAddr.Hash160())
	t.Logf("user nonce: %d", nonce)
	contractAddr, _ := types.MakeContractAddress(userAddr, nonce)
	vmParam.contractAddr = contractAddr
	t.Logf("contract address: %s", contractAddr)
	// TODO
	gasRefundTx := createGasRefundUtxoTx(userAddr.Hash160(), gasPrice*(gasLimit-gasUsed), nonce+1)
	rootHashStr := "8b7ffbaad0576ea921c7789135a96c1c1a5c82c7de1c3df11bcc7046a47000ef"
	utxoRootHashStr := "03e42d0ebbba0dc5d6b58ebb691e3ea789920ec6ce39fd48a542fa41d5f73262"
	b3 := contractBlockHandle(t, blockChain, vmTx, b2, rootHashStr, utxoRootHashStr,
		vmParam, nil, gasRefundTx)
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
	contractVout, err = txlogic.MakeContractCallVout(contractAddr.String(),
		vmValue, gasLimit, gasPrice, byteCode)
	ensure.Nil(t, err)
	prevHash, _ = b3.Txs[1].TxHash()
	changeValue = changeValue - vmValue - gasPrice*gasLimit
	vmTx = types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(prevHash, 1), 0)).
		AppendVout(contractVout).
		AppendVout(txlogic.MakeVout(userAddr.String(), changeValue))
	signTx(vmTx, privKey, pubKey)
	stateDB, err = state.New(&b3.Header.RootHash, &b3.Header.UtxoRoot, blockChain.db)
	ensure.Nil(t, err)
	nonce = stateDB.GetNonce(*userAddr.Hash160())
	t.Logf("user nonce: %d", nonce)
	rootHashStr = "346834219547844d55c00b9aba28e41c28ea5283527eb72278b6d68e4013e0e0"
	utxoRootHashStr = "03e42d0ebbba0dc5d6b58ebb691e3ea789920ec6ce39fd48a542fa41d5f73262"
	gasRefundTx = createGasRefundUtxoTx(userAddr.Hash160(), gasPrice*(gasLimit-gasUsed), nonce+1)
	b4 := contractBlockHandle(t, blockChain, vmTx, b3, rootHashStr, utxoRootHashStr,
		vmParam, nil, gasRefundTx)
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
	contractVout, err = txlogic.MakeContractCallVout(contractAddr.String(),
		vmValue, gasLimit, gasPrice, byteCode)
	ensure.Nil(t, err)
	// use internal tx vout
	prevHash, _ = b4.Txs[1].TxHash()
	changeValue = changeValue - vmValue - gasPrice*gasLimit
	vmTx = types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(prevHash, 1), 0)).
		AppendVout(contractVout).
		AppendVout(txlogic.MakeVout(userAddr.String(), changeValue))
	signTx(vmTx, privKey, pubKey)
	stateDB, err = state.New(&b4.Header.RootHash, &b4.Header.UtxoRoot, blockChain.db)
	ensure.Nil(t, err)
	nonce = stateDB.GetNonce(*userAddr.Hash160())
	t.Logf("user nonce: %d", nonce)
	gasRefundTx = createGasRefundUtxoTx(userAddr.Hash160(), gasPrice*(gasLimit-gasUsed), nonce+1)
	rootHashStr = "45367d93158cb1a1eb5c4bc54918df63870e0abbe483411d2bc727281c8a4bbc"
	utxoRootHashStr = "03e42d0ebbba0dc5d6b58ebb691e3ea789920ec6ce39fd48a542fa41d5f73262"
	b5 := contractBlockHandle(t, blockChain, vmTx, b4, rootHashStr, utxoRootHashStr,
		vmParam, nil, gasRefundTx)
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
	contractVout, err = txlogic.MakeContractCallVout(contractAddr.String(),
		vmValue, gasLimit, gasPrice, byteCode)
	ensure.Nil(t, err)
	// use internal tx vout
	prevHash, _ = b5.Txs[1].TxHash()
	changeValue = changeValue - vmValue - gasPrice*gasLimit
	vmTx = types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(prevHash, 1), 0)).
		AppendVout(contractVout).
		AppendVout(txlogic.MakeVout(userAddr.String(), changeValue))
	signTx(vmTx, privKey, pubKey)
	stateDB, err = state.New(&b5.Header.RootHash, &b5.Header.UtxoRoot, blockChain.db)
	ensure.Nil(t, err)
	nonce = stateDB.GetNonce(*userAddr.Hash160())
	t.Logf("user nonce: %d", nonce)
	gasRefundTx = createGasRefundUtxoTx(userAddr.Hash160(), gasPrice*(gasLimit-gasUsed), nonce+1)
	rootHashStr = "9408d3e296b4525cfd5db08cc1ee5ac3f591eadf30e3b316a656d99b9ab836ed"
	utxoRootHashStr = "03e42d0ebbba0dc5d6b58ebb691e3ea789920ec6ce39fd48a542fa41d5f73262"
	b6 := contractBlockHandle(t, blockChain, vmTx, b5, rootHashStr, utxoRootHashStr,
		vmParam, nil, gasRefundTx)
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
	contractVout, err = txlogic.MakeContractCallVout(contractAddr.String(),
		vmValue, gasLimit, gasPrice, byteCode)
	ensure.Nil(t, err)
	// use internal tx vout
	prevHash, _ = b6.Txs[1].TxHash()
	changeValue = changeValue - vmValue - gasPrice*gasLimit
	vmTx = types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(prevHash, 1), 0)).
		AppendVout(contractVout).
		AppendVout(txlogic.MakeVout(userAddr.String(), changeValue))
	signTx(vmTx, privKey, pubKey)
	stateDB, err = state.New(&b6.Header.RootHash, &b6.Header.UtxoRoot, blockChain.db)
	ensure.Nil(t, err)
	nonce = stateDB.GetNonce(*userAddr.Hash160())
	t.Logf("user nonce: %d", nonce)
	gasRefundTx = createGasRefundUtxoTx(userAddr.Hash160(), gasPrice*(gasLimit-gasUsed), nonce+1)
	gasRefundTxHash, _ := gasRefundTx.TxHash()
	t.Logf("refund tx: %s", gasRefundTxHash)
	rootHashStr = "a57df55796e02f3750143c651d6094caec1c33b9e73fd3d13ed57a2e12d094dd"
	utxoRootHashStr = "03e42d0ebbba0dc5d6b58ebb691e3ea789920ec6ce39fd48a542fa41d5f73262"
	contractBlockHandle(t, blockChain, vmTx, b6, rootHashStr, utxoRootHashStr,
		vmParam, nil, gasRefundTx)
	t.Logf("b6 -> b7 passed, now tail height: %d", blockChain.LongestChainHeight)
	// check balances
	// return 0x1e8480 = 2000000, check okay
}
