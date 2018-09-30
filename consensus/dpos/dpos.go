// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package dpos

import (
	"encoding/hex"
	"time"

	"github.com/BOXFoundation/Quicksilver/core"
	"github.com/BOXFoundation/Quicksilver/core/types"
	"github.com/BOXFoundation/Quicksilver/log"
	"github.com/BOXFoundation/Quicksilver/p2p"
	"github.com/jbenet/goprocess"
)

var (
	logger = log.NewLogger("dpos") // logger
)

func init() {
}

// Config defines the configurations of dpos
type Config struct {
	Index  int    `mapstructure:"index"`
	Pubkey string `mapstructure:"pubkey"`
}

// Dpos define dpos struct
type Dpos struct {
	chain *core.BlockChain
	net   p2p.Net
	proc  goprocess.Process
	cfg   *Config
	miner types.Address
}

// NewDpos new a dpos implement.
func NewDpos(chain *core.BlockChain, net p2p.Net, parent goprocess.Process, cfg *Config) *Dpos {

	dpos := &Dpos{
		chain: chain,
		net:   net,
		proc:  goprocess.WithParent(parent),
		cfg:   cfg,
	}

	pubkey, err := hex.DecodeString(dpos.cfg.Pubkey)
	if err != nil {
		panic("invalid hex in source file: " + dpos.cfg.Pubkey)
	}
	addr, err := types.NewAddressPubKeyHash(pubkey, 0x00)
	if err != nil {
		panic("invalid public key in test source")
	}
	logger.Info("miner addr: ", addr.String())
	dpos.miner = addr
	return dpos
}

// Run start dpos
func (dpos *Dpos) Run() {
	logger.Info("Dpos run")
	go dpos.loop()
}

// Stop dpos
func (dpos *Dpos) Stop() {

}

func (dpos *Dpos) loop() {
	logger.Info("Start block mint")
	timeChan := time.NewTicker(time.Second).C
	for {
		select {
		case <-timeChan:
			dpos.mint()
		case <-dpos.proc.Closing():
			logger.Info("Stopped Dpos Mining.")
			return
		}
	}
}

func (dpos *Dpos) mint() {
	now := time.Now().Unix()
	if int(now%10) != dpos.cfg.Index {
		return
	}

	logger.Infof("My turn to mint a block, time: %d", now)
	dpos.mintBlock()
}

func (dpos *Dpos) mintBlock() {
	tail, _ := dpos.chain.LoadTailBlock()
	logger.Infof("current tail block: %s height: %d", *tail.Hash, tail.Height)

	block := types.NewBlock(tail)
	dpos.chain.PackTxs(block, dpos.miner)
	// block.setMiner()
	dpos.chain.ProcessBlock(block, true)
}
