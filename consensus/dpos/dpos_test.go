// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package dpos

import (
	"testing"
	"time"

	"github.com/BOXFoundation/boxd/boxd/eventbus"
	"github.com/BOXFoundation/boxd/core/chain"
	"github.com/BOXFoundation/boxd/core/txpool"
	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/p2p"
	_ "github.com/BOXFoundation/boxd/storage/memdb"
	"github.com/facebookgo/ensure"
)

type DummyDpos struct {
	dpos         *Dpos
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

	dpos           = NewDummyDpos(cfg)
	dposBookkeeper = NewDummyDpos(cfgBookkeeper)
	bus            = eventbus.New()
)

func NewDummyDpos(cfg *Config) *DummyDpos {

	blockchain := chain.NewTestBlockChain()
	txPool := txpool.NewTransactionPool(blockchain.Proc(), p2p.NewDummyPeer(), blockchain, bus)
	dpos, _ := NewDpos(txPool.Proc(), blockchain, txPool, p2p.NewDummyPeer(), cfg)
	blockchain.Setup(dpos, nil)
	dpos.Setup()
	isBookkeeper := dpos.IsBookkeeper()
	bftService, _ := NewBftService(dpos)
	dpos.bftservice = bftService
	return &DummyDpos{
		dpos:         dpos,
		isBookkeeper: isBookkeeper,
	}
}

func TestDpos_ValidateMiner(t *testing.T) {
	ensure.DeepEqual(t, dpos.isBookkeeper, false)
	ensure.DeepEqual(t, dposBookkeeper.isBookkeeper, true)
}

func TestDpos_verifyBookkeeper(t *testing.T) {

	timestamp := int64(1541824620)

	// check bookkeeper in right time but wrong bookkeeper.
	result := dpos.dpos.verifyBookkeeper(timestamp)
	ensure.DeepEqual(t, result, ErrNotMyTurnToProduce)

	// check bookkeeper with right bookkeeper and right time.
	result = dposBookkeeper.dpos.verifyBookkeeper(timestamp)
	ensure.Nil(t, result)
}

func TestDpos_run(t *testing.T) {

	dposBookkeeper.dpos.StopMint()
	result := dposBookkeeper.dpos.run(time.Now().Unix())
	ensure.DeepEqual(t, result, ErrNoLegalPowerToProduce)

	dposBookkeeper.dpos.RecoverMint()

	timestamp := int64(1541824625)
	result = dposBookkeeper.dpos.run(timestamp)
	ensure.DeepEqual(t, result, ErrNotMyTurnToProduce)

	timestamp1 := int64(1541824620)
	result = dposBookkeeper.dpos.run(timestamp1)
	ensure.Nil(t, result)

}

func TestDpos_FindProposerWithTimeStamp(t *testing.T) {
	hash, err := dposBookkeeper.dpos.context.periodContext.FindProposerWithTimeStamp(1541824620)
	ensure.Nil(t, err)
	ensure.DeepEqual(t, *hash, dposBookkeeper.dpos.context.periodContext.period[0].addr)
}

func TestDpos_signBlock(t *testing.T) {

	ensure.DeepEqual(t, dposBookkeeper.isBookkeeper, true)
	block := &chain.GenesisBlock

	block.Header.TimeStamp = 1541824626
	err := dposBookkeeper.dpos.signBlock(block)
	ensure.Nil(t, err)
	ok, err := dposBookkeeper.dpos.VerifySign(block)
	ensure.DeepEqual(t, ok, false)

	block.Header.TimeStamp = 1541824620
	err = dposBookkeeper.dpos.signBlock(block)
	ensure.Nil(t, err)
	ok, err = dposBookkeeper.dpos.VerifySign(block)
	ensure.Nil(t, err)
	ensure.DeepEqual(t, ok, true)
}

func TestDpos_LoadPeriodContext(t *testing.T) {

	result, err := dposBookkeeper.dpos.LoadPeriodContext()
	ensure.Nil(t, err)

	initperiod, err := InitPeriodContext()
	ensure.Nil(t, err)

	ensure.DeepEqual(t, result.period, initperiod.period)

	dposBookkeeper.dpos.context.periodContext.period = append(dposBookkeeper.dpos.context.periodContext.period, &Period{
		addr:   types.AddressHash{156, 17, 133, 165, 197, 233, 252, 84, 97, 40, 8, 151, 126, 232, 245, 72, 178, 37, 141, 49},
		peerID: "",
	})
	err = dposBookkeeper.dpos.StorePeriodContext()
	ensure.Nil(t, err)
	result, err = dposBookkeeper.dpos.LoadPeriodContext()
	ensure.Nil(t, err)

	ensure.DeepEqual(t, result.period, dposBookkeeper.dpos.context.periodContext.period)

}
