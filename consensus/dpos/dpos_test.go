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
	dpos    *Dpos
	isMiner bool
}

var (
	cfg = &Config{
		Keypath:    "../../keyfile/key7.keystore",
		EnableMint: true,
		Passphrase: "1",
	}

	cfgMiner = &Config{
		Keypath:    "../../keyfile/key1.keystore",
		EnableMint: true,
		Passphrase: "1",
	}

	dpos      = NewDummyDpos(cfg)
	dposMiner = NewDummyDpos(cfgMiner)
	bus       = eventbus.New()
)

func NewDummyDpos(cfg *Config) *DummyDpos {

	blockchain := chain.NewTestBlockChain()
	txPool := txpool.NewTransactionPool(blockchain.Proc(), p2p.NewDummyPeer(), blockchain, bus)
	dpos, _ := NewDpos(txPool.Proc(), blockchain, txPool, p2p.NewDummyPeer(), cfg)
	blockchain.Setup(dpos, nil)
	dpos.Setup()
	isMiner := dpos.ValidateMiner()
	return &DummyDpos{
		dpos:    dpos,
		isMiner: isMiner,
	}
}

func TestDpos_ValidateMiner(t *testing.T) {
	ensure.DeepEqual(t, dpos.isMiner, false)
	ensure.DeepEqual(t, dposMiner.isMiner, true)
}

func TestDpos_checkMiner(t *testing.T) {

	timestamp1 := int64(1541824620)
	timestamp2 := int64(1541824621)

	// check miner in right time but wrong miner.
	result := dpos.dpos.checkMiner(timestamp1)
	ensure.DeepEqual(t, result, ErrNotMyTurnToMint)

	// check miner in wrong time.
	result = dpos.dpos.checkMiner(timestamp2)
	ensure.DeepEqual(t, result, ErrWrongTimeToMint)

	// check miner with right miner but wrong time.
	result = dposMiner.dpos.checkMiner(timestamp2)
	ensure.DeepEqual(t, result, ErrWrongTimeToMint)

	// check miner with right miner and right time.
	result = dposMiner.dpos.checkMiner(timestamp1)
	ensure.Nil(t, result)
}
func TestDpos_mint(t *testing.T) {

	dposMiner.dpos.StopMint()
	result := dposMiner.dpos.mint(time.Now().Unix())
	ensure.DeepEqual(t, result, ErrNoLegalPowerToMint)

	dposMiner.dpos.RecoverMint()
	timestamp1 := int64(1541824621)
	result = dposMiner.dpos.mint(timestamp1)
	ensure.DeepEqual(t, result, ErrWrongTimeToMint)

	timestamp2 := int64(1541824625)
	result = dposMiner.dpos.mint(timestamp2)
	ensure.DeepEqual(t, result, ErrNotMyTurnToMint)

	timestamp3 := int64(1541824620)
	result = dposMiner.dpos.mint(timestamp3)
	ensure.Nil(t, result)

}

func TestDpos_FindMinerWithTimeStamp(t *testing.T) {
	hash, err := dposMiner.dpos.context.periodContext.FindMinerWithTimeStamp(1541824620)
	ensure.Nil(t, err)
	ensure.DeepEqual(t, *hash, dposMiner.dpos.context.periodContext.period[0].addr)
}

func TestDpos_signBlock(t *testing.T) {

	ensure.DeepEqual(t, dposMiner.isMiner, true)
	block := &chain.GenesisBlock

	block.Header.TimeStamp = 1541824621
	err := dposMiner.dpos.signBlock(block)
	ensure.Nil(t, err)
	ok, err := dposMiner.dpos.VerifySign(block)
	ensure.DeepEqual(t, err, ErrWrongTimeToMint)
	ensure.DeepEqual(t, ok, false)

	block.Header.TimeStamp = 1541824620
	err = dposMiner.dpos.signBlock(block)
	ensure.Nil(t, err)
	ok, err = dposMiner.dpos.VerifySign(block)
	ensure.Nil(t, err)
	ensure.DeepEqual(t, ok, true)
}

func TestDpos_LoadPeriodContext(t *testing.T) {

	result, err := dposMiner.dpos.LoadPeriodContext()
	ensure.Nil(t, err)

	initperiod, err := InitPeriodContext()
	ensure.Nil(t, err)

	ensure.DeepEqual(t, result.period, initperiod.period)

	dposMiner.dpos.context.periodContext.period = append(dposMiner.dpos.context.periodContext.period, &Period{
		addr:   types.AddressHash{156, 17, 133, 165, 197, 233, 252, 84, 97, 40, 8, 151, 126, 232, 245, 72, 178, 37, 141, 49},
		peerID: "xxxxxxx",
	})
	err = dposMiner.dpos.StorePeriodContext()
	ensure.Nil(t, err)
	result, err = dposMiner.dpos.LoadPeriodContext()
	ensure.Nil(t, err)

	ensure.DeepEqual(t, result.period, dposMiner.dpos.context.periodContext.period)

}
