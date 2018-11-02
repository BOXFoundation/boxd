// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package dpos

import (
	"testing"
	"time"

	"github.com/BOXFoundation/boxd/core/chain"
	"github.com/BOXFoundation/boxd/core/txpool"
	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/p2p"
	_ "github.com/BOXFoundation/boxd/storage/memdb"
	"github.com/facebookgo/ensure"
)

var (
	cfg = &Config{
		Keypath:    "../../keyfile/key4.keystore",
		EnableMint: true,
		Passphrase: "zaq12wsx",
	}

	cfgMiner = &Config{
		Keypath:    "../../keyfile/key.keystore",
		EnableMint: true,
		Passphrase: "zaq12wsx",
	}
)

func NewDummyDpos(cfg *Config) (*Dpos, error) {

	blockchain := chain.NewTestBlockChain()
	txPool := txpool.NewTransactionPool(blockchain.Proc(), p2p.NewDummyPeer(), blockchain)
	dpos, err := NewDpos(txPool.Proc(), blockchain, txPool, p2p.NewDummyPeer(), cfg)
	blockchain.Setup(dpos, nil)
	dpos.Setup()
	return dpos, err
}

func TestDpos_ValidateMiner(t *testing.T) {

	dpos1, err := NewDummyDpos(cfg)
	ensure.Nil(t, err)
	result := dpos1.ValidateMiner()
	ensure.DeepEqual(t, result, false)

	dpos2, err := NewDummyDpos(cfgMiner)
	ensure.Nil(t, err)
	result = dpos2.ValidateMiner()
	ensure.DeepEqual(t, result, true)
}

func TestDpos_checkMiner(t *testing.T) {

	timestamp1 := int64(1541077395)
	timestamp2 := int64(1541077396)

	dpos1, err := NewDummyDpos(cfg)
	ensure.Nil(t, err)

	// check miner in right time but wrong miner.
	result := dpos1.checkMiner(timestamp1)
	ensure.DeepEqual(t, result, ErrNotMyTurnToMint)

	// check miner in wrong time.
	result = dpos1.checkMiner(timestamp2)
	ensure.DeepEqual(t, result, ErrWrongTimeToMint)

	dpos2, err := NewDummyDpos(cfgMiner)
	ensure.Nil(t, err)

	// check miner with right miner but wrong time.
	result = dpos2.checkMiner(timestamp2)
	ensure.DeepEqual(t, result, ErrWrongTimeToMint)

	// check miner with right miner and right time.
	result = dpos2.checkMiner(timestamp1)
	ensure.Nil(t, err)
}
func TestDpos_mint(t *testing.T) {

	dpos, err := NewDummyDpos(cfgMiner)
	ensure.Nil(t, err)

	dpos.StopMint()
	result := dpos.mint(time.Now().Unix())
	ensure.DeepEqual(t, result, ErrNoLegalPowerToMint)

	isMiner := dpos.ValidateMiner()
	ensure.DeepEqual(t, isMiner, true)

	dpos.RecoverMint()
	timestamp1 := int64(1541077396)
	result = dpos.mint(timestamp1)
	ensure.DeepEqual(t, result, ErrWrongTimeToMint)

	timestamp2 := int64(1541077400)
	result = dpos.mint(timestamp2)
	ensure.DeepEqual(t, result, ErrNotMyTurnToMint)

	timestamp3 := int64(1541077395)
	result = dpos.mint(timestamp3)
	ensure.Nil(t, result)

}

func TestDpos_signBlock(t *testing.T) {
	dpos, err := NewDummyDpos(cfgMiner)
	ensure.Nil(t, err)
	isMiner := dpos.ValidateMiner()
	ensure.DeepEqual(t, isMiner, true)
	block := &chain.GenesisBlock

	block.Header.TimeStamp = 1541077396
	err = dpos.signBlock(block)
	ensure.Nil(t, err)
	ok, err := dpos.VerifySign(block)
	ensure.DeepEqual(t, err, ErrWrongTimeToMint)
	ensure.DeepEqual(t, ok, false)

	block.Header.TimeStamp = 1541077395
	err = dpos.signBlock(block)
	ensure.Nil(t, err)
	ok, err = dpos.VerifySign(block)
	ensure.Nil(t, err)
	ensure.DeepEqual(t, ok, true)
}

func TestDpos_LoadPeriodContext(t *testing.T) {

	dpos, err := NewDummyDpos(cfgMiner)
	ensure.Nil(t, err)

	result, err := dpos.LoadPeriodContext()
	ensure.Nil(t, err)

	initperiod, err := InitPeriodContext()
	ensure.Nil(t, err)

	ensure.DeepEqual(t, result.period, initperiod.period)

	dpos.context.periodContext.period = append(dpos.context.periodContext.period, &Period{
		addr:   types.AddressHash{156, 17, 133, 165, 197, 233, 252, 84, 97, 40, 8, 151, 126, 232, 245, 72, 178, 37, 141, 49},
		peerID: "xxxxxxx",
	})
	err = dpos.StorePeriodContext()
	ensure.Nil(t, err)
	result, err = dpos.LoadPeriodContext()
	ensure.Nil(t, err)

	ensure.DeepEqual(t, result.period, dpos.context.periodContext.period)

}
