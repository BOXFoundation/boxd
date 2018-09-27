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
	index  = 0
)

func init() {
}

// Dpos define dpos struct
type Dpos struct {
	chain *core.BlockChain
	net   *p2p.Net
	proc  goprocess.Process
}

// NewDpos new a dpos implement.
func NewDpos(chain *core.BlockChain, net *p2p.Net, parent goprocess.Process) *Dpos {
	return &Dpos{
		chain: chain,
		net:   net,
		proc:  goprocess.WithParent(parent),
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
	now := time.Now().UnixNano()
	if int(now%3) != index {
		return
	}

	logger.Info("My turn to mint a block")
	dpos.mintBlock()
}

func (dpos *Dpos) mintBlock() {
	tail := dpos.chain.TailBlock()
	block := types.NewBlock(tail)
	dpos.chain.PackTxs(block)
}
