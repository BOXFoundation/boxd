// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package bpos

import (
	"math/big"
	"testing"
	"time"

	"github.com/BOXFoundation/boxd/boxd/eventbus"
	"github.com/BOXFoundation/boxd/core/chain"
	"github.com/BOXFoundation/boxd/core/txlogic"
	"github.com/BOXFoundation/boxd/core/txpool"
	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/crypto"
	"github.com/BOXFoundation/boxd/p2p"
	"github.com/BOXFoundation/boxd/script"
	_ "github.com/BOXFoundation/boxd/storage/memdb"
	"github.com/facebookgo/ensure"
)

type DummyBpos struct {
	bpos         *Bpos
	isBookkeeper bool
}

var (
	cfg = &Config{
		Keypath:    "../../keyfile/key7.keystore",
		EnableMint: true,
		Passphrase: "1",
	}

	cfgBookkeeper = &Config{
		Keypath:    "../../keyfile/key1.keystore",
		EnableMint: true,
		Passphrase: "1",
	}

	bpos           = NewDummyBpos(cfg)
	bposBookkeeper = NewDummyBpos(cfgBookkeeper)
	bus            = eventbus.New()

	privKey, pubKey, _ = crypto.NewKeyPair()
	addr, _            = types.NewAddressFromPubKey(pubKey)
	scriptAddr         = addr.Hash()
	scriptPubKey       = script.PayToPubKeyHashScript(scriptAddr)
)

func NewDummyBpos(cfg *Config) *DummyBpos {

	blockchain := chain.NewTestBlockChain()
	txPool := txpool.NewTransactionPool(blockchain.Proc(), p2p.NewDummyPeer(), blockchain, bus)
	bpos, _ := NewBpos(txPool.Proc(), blockchain, txPool, p2p.NewDummyPeer(), cfg)
	blockchain.Setup(bpos, nil)
	bpos.Setup()
	isBookkeeper := bpos.IsBookkeeper()
	bftService, _ := NewBftService(bpos)
	bpos.bftservice = bftService
	return &DummyBpos{
		bpos:         bpos,
		isBookkeeper: isBookkeeper,
	}
}

func TestDpos_ValidateMiner(t *testing.T) {
	ensure.DeepEqual(t, bpos.isBookkeeper, false)
	ensure.DeepEqual(t, bposBookkeeper.isBookkeeper, true)
}

func TestDpos_verifyBookkeeper(t *testing.T) {

	timestamp := int64(1541824620)
	dynasty, err := bpos.bpos.fetchDynastyByHeight(bpos.bpos.chain.LongestChainHeight)
	ensure.Nil(t, err)
	// check bookkeeper in right time but wrong bookkeeper.
	result := bpos.bpos.verifyBookkeeper(timestamp, dynasty.delegates)
	ensure.DeepEqual(t, result, ErrNotMyTurnToProduce)

	// check bookkeeper with right bookkeeper and right time.
	result = bposBookkeeper.bpos.verifyBookkeeper(timestamp, dynasty.delegates)
	ensure.Nil(t, result)
}

func TestDpos_run(t *testing.T) {

	bposBookkeeper.bpos.StopMint()
	result := bposBookkeeper.bpos.run(time.Now().Unix())
	ensure.DeepEqual(t, result, ErrNoLegalPowerToProduce)

	bposBookkeeper.bpos.RecoverMint()

	timestamp := int64(1541824625)
	result = bposBookkeeper.bpos.run(timestamp)
	ensure.DeepEqual(t, result, ErrNotMyTurnToProduce)

	timestamp1 := int64(1541824620)
	result = bposBookkeeper.bpos.run(timestamp1)
	ensure.Nil(t, result)

}

// func TestDpos_FindProposerWithTimeStamp(t *testing.T) {
// 	hash, err := bposBookkeeper.bpos.context.periodContext.FindProposerWithTimeStamp(1541824620)
// 	ensure.Nil(t, err)
// 	ensure.DeepEqual(t, *hash, bposBookkeeper.bpos.context.periodContext.period[0].addr)
// }

func TestDpos_signBlock(t *testing.T) {

	ensure.DeepEqual(t, bposBookkeeper.isBookkeeper, true)
	block := &chain.GenesisBlock

	block.Header.TimeStamp = 1541824626
	err := bposBookkeeper.bpos.signBlock(block)
	ensure.Nil(t, err)
	ok, err := bposBookkeeper.bpos.verifySign(block)
	ensure.DeepEqual(t, ok, false)

	block.Header.TimeStamp = 1541824620
	err = bposBookkeeper.bpos.signBlock(block)
	ensure.Nil(t, err)
	ok, err = bposBookkeeper.bpos.verifySign(block)
	ensure.Nil(t, err)
	ensure.DeepEqual(t, ok, true)
}

// func TestDpos_LoadPeriodContext(t *testing.T) {

// 	result, err := bposBookkeeper.bpos.LoadPeriodContext()
// 	ensure.Nil(t, err)

// 	initperiod, err := InitPeriodContext()
// 	ensure.Nil(t, err)

// 	ensure.DeepEqual(t, result.period, initperiod.period)

// 	bposBookkeeper.bpos.context.periodContext.period = append(bposBookkeeper.bpos.context.periodContext.period, &Period{
// 		addr:   types.AddressHash{156, 17, 133, 165, 197, 233, 252, 84, 97, 40, 8, 151, 126, 232, 245, 72, 178, 37, 141, 49},
// 		peerID: "",
// 	})
// 	err = bposBookkeeper.bpos.StorePeriodContext()
// 	ensure.Nil(t, err)
// 	result, err = bposBookkeeper.bpos.LoadPeriodContext()
// 	ensure.Nil(t, err)

// 	ensure.DeepEqual(t, result.period, bposBookkeeper.bpos.context.periodContext.period)

// }

func TestSortPendingTxs(t *testing.T) {
	var pendingTxs []*types.TxWrap
	tx0, _ := chain.CreateCoinbaseTx(addr.Hash(), 0)
	tx1, _ := chain.CreateCoinbaseTx(addr.Hash(), 1)
	tx2, _ := chain.CreateCoinbaseTx(addr.Hash(), 2)
	txwrap0 := createTxWrap(tx0, 100)
	txwrap1 := createTxWrap(tx1, 200)
	txwrap2 := createTxWrap(tx2, 300)

	txwrap20 := createTxWrap(txwrap2.Tx, 200)
	txwrap21 := createTxWrap(txwrap20.Tx, 500)

	txwrap10 := createTxWrap(txwrap1.Tx, 600)
	txwrap11 := createTxWrap(txwrap10.Tx, 900)

	pendingTxs = []*types.TxWrap{txwrap0, txwrap1, txwrap2, txwrap20, txwrap21, txwrap10, txwrap11}

	sortedTxs, err := bpos.bpos.sortPendingTxs(pendingTxs)
	ensure.Nil(t, err)
	var sortedAddTime []int64
	for _, v := range sortedTxs {
		sortedAddTime = append(sortedAddTime, v.AddedTimestamp)
	}
	ensure.DeepEqual(t, sortedAddTime, []int64{100, 200, 300, 200, 600, 500, 900})

}

func TestCalcScore(t *testing.T) {
	bpos.bpos.chain.LongestChainHeight = 1000
	bpos.bpos.context.dynastySwitchThreshold = big.NewInt(10000)
	bpos.bpos.context.dynasty = &Dynasty{}
	bpos.bpos.context.dynasty.delegates = []Delegate{Delegate{}, Delegate{}, Delegate{}, Delegate{}, Delegate{}, Delegate{}}
	bpos.bpos.context.nextDelegates = []Delegate{Delegate{}, Delegate{}, Delegate{}, Delegate{}, Delegate{}, Delegate{}, Delegate{}, Delegate{}, Delegate{}, Delegate{}}
	type args struct {
		totalVotes int64
		delegate   Delegate
	}
	tests := []struct {
		name  string
		args  args
		score int64
	}{
		{
			"test1",
			args{
				10000000 * 1e8,
				Delegate{
					*types.NewAddressHash([]byte{0x1}),
					"a13wr23d42e3e3ti",
					big.NewInt(1800000 * 1e8),
					big.NewInt(1800000 * 1e8),
					big.NewInt(0),
					big.NewInt(1),
					true,
				},
			},
			4337414,
		},
		{
			"test2",
			args{
				10000000 * 1e8,
				Delegate{
					*types.NewAddressHash([]byte{0x2}),
					"a13wr23d42eed43ew",
					big.NewInt(180000 * 1e8),
					big.NewInt(1800000 * 1e8),
					big.NewInt(0),
					big.NewInt(1),
					true,
				},
			},
			2871578,
		},
		{
			"test3",
			args{
				10000000 * 1e8,
				Delegate{
					*types.NewAddressHash([]byte{0x2}),
					"a13wr23d42eed43ew",
					big.NewInt(18000000 * 1e8),
					big.NewInt(1800000 * 1e8),
					big.NewInt(0),
					big.NewInt(10),
					true,
				},
			},
			8364012,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := bpos.bpos.calcScore(tt.args.totalVotes, tt.args.delegate)
			ensure.Nil(t, err)
			if got.Int64() != tt.score {
				t.Errorf("calcScore() = %v, want %v", got, tt.score)
			}
		})
	}
}

func createTxWrap(tx *types.Transaction, addTime int64) *types.TxWrap {
	txchild := createTx(tx, addr.Hash160())
	return &types.TxWrap{
		Tx:             txchild,
		AddedTimestamp: addTime,
		IsScriptValid:  true,
	}
}

func createTx(parentTx *types.Transaction, address *types.AddressHash) *types.Transaction {
	vIn := txlogic.MakeVinForTest(parentTx, 0)
	txOut := txlogic.MakeVout(address, 1)
	vOut := []*types.TxOut{txOut}
	tx := &types.Transaction{
		Vin:  vIn,
		Vout: vOut,
	}
	return tx
}
