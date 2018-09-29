// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package dpos

import (
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
}

// NewDpos new a dpos implement.
func NewDpos(chain *core.BlockChain, net p2p.Net, parent goprocess.Process, cfg *Config) *Dpos {
	return &Dpos{
		chain: chain,
		net:   net,
		proc:  goprocess.WithParent(parent),
		cfg:   cfg,
	}
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

	logger.Info("My turn to mint a block")
	dpos.mintBlock()
}

func (dpos *Dpos) mintBlock() {
	// serializedPubKey, err := hex.DecodeString(dpos.cfg.Pubkey)
	// if err != nil {
	// 	panic("invalid hex in source file: " + dpos.cfg.Pubkey)
	// }
	// logger.Info("pubkey ", serializedPubKey)
	pubkey := []byte{
		0xe3, 0x4c, 0xce, 0x70, 0xc8, 0x63, 0x73, 0x27, 0x3e, 0xfc,
		0xc5, 0x4c, 0xe7, 0xd2, 0xa4, 0x91, 0xbb, 0x4a, 0x0e, 0x84}
	addr, err := types.NewAddressPubKeyHash(pubkey, 0x00)
	if err != nil {
		panic("invalid public key in test source")
	}

	tail, _ := dpos.chain.LoadTailBlock()
	logger.Info("current tail block: ", tail.Hash, tail.Height)

	block := types.NewBlock(tail)
	dpos.chain.PackTxs(block, addr)
	// block.setMiner()
	dpos.chain.ProcessBlock(block, true)
}
