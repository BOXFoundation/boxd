// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package walletdata

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/BOXFoundation/boxd/core/types"

	"github.com/BOXFoundation/boxd/log"

	"github.com/jbenet/goprocess"
)

const queryInterval = 2

var logger = log.NewLogger("walletdata")

// BlockCatcher catch up the block generation of server node using grpc call
type BlockCatcher interface {
	Run() error
	Stop() error
	RegisterListener(AddrProcessor)
}

// BlockFetcher is an interface to obtaining block info
type BlockFetcher interface {
	Height() (uint32, error)
	HashForHeight(uint32) (string, error)
	Block(string) (*types.Block, error)
}

// NewBlockCatcher creates a block catcher with implementation
func NewBlockCatcher(parent goprocess.Process, fetcher BlockFetcher) BlockCatcher {
	return &catcherImpl{
		currentHeight: 0,
		fetcher:       fetcher,
		blocks:        &sync.Map{},
		parent:        goprocess.WithParent(parent),
		syncMutex:     &sync.Mutex{},
		processor:     []AddrProcessor{},
	}
}

type catcherImpl struct {
	currentHeight uint32
	parent        goprocess.Process
	fetcher       BlockFetcher
	blocks        *sync.Map
	syncMutex     *sync.Mutex
	processor     []AddrProcessor
}

func (ci *catcherImpl) RegisterListener(p AddrProcessor) {
	ci.processor = append(ci.processor, p)
}

var _ BlockCatcher = (*catcherImpl)(nil)

func (ci *catcherImpl) Run() error {
	ci.parent.Go(ci.observer)
	return nil
}

func (ci *catcherImpl) Stop() error {
	ci.parent.Closing()
	return nil
}

func (ci *catcherImpl) observer(p goprocess.Process) {
	t := time.NewTicker(queryInterval * time.Second)
	defer t.Stop()
	for {
		select {
		case <-t.C:
			ci.sync()
		case <-p.Closing():
			fmt.Println("Observer closing")
			return
		}
	}
}

func (ci *catcherImpl) sync() {
	ci.syncMutex.Lock()
	height, hash := ci.getCurrentBlock()
	if height > ci.currentHeight {
		err := ci.catchup(height)
		if err != nil {
			ci.reorg(height)
		}
	} else if height == ci.currentHeight && strings.Compare(ci.getStoredBlockHash(height), hash) != 0 {
		ci.reorg(height)
	}
	ci.syncMutex.Unlock()
}

func (ci *catcherImpl) reorg(serverHeight uint32) {
	ci.revert(serverHeight)
	ci.catchup(serverHeight)
}

func (ci *catcherImpl) getStoredBlockHash(height uint32) string {
	b, ok := ci.blocks.Load(height)
	if ok {
		return b.(string)
	}
	return ""
}

func (ci *catcherImpl) revert(serverHeight uint32) error {
	var start, end uint32
	if ci.currentHeight < serverHeight {
		end = ci.currentHeight
	} else {
		end = serverHeight
	}
	parent := (start + end) / 2
	for {
		if parent == 0 || parent == start {
			break
		}
		hash, err := ci.fetcher.HashForHeight(parent)
		if err != nil {
			return err
		}
		if strings.Compare(hash, ci.getStoredBlockHash(parent)) != 0 {
			end = parent
		} else {
			start = parent
		}
	}
	for i := ci.currentHeight; i > parent; i-- {
		// TODO: impl block fetch or ignore block info when revert
		ci.revertBlock(i, nil)
	}
	return nil
}

func (ci *catcherImpl) catchup(targetHeight uint32) error {
	previousBlockHash := ci.getStoredBlockHash(ci.currentHeight)
	for i := ci.currentHeight + 1; i <= targetHeight; i++ {
		blockHash, err := ci.fetcher.HashForHeight(i)
		if err != nil {
			return err
		}
		block, err := ci.fetcher.Block(blockHash)
		if err != nil {
			return err
		}
		if previousBlockHash != "" && strings.Compare(block.Header.PrevBlockHash.String(), previousBlockHash) != 0 {
			return fmt.Errorf("invalid block previous hash")
		}
		ci.acceptBlock(block.Height, block)
		previousBlockHash = block.Hash.String()
	}
	return nil
}

func (ci *catcherImpl) acceptBlock(height uint32, block *types.Block) {
	ci.currentHeight = height
	ci.blocks.Store(height, block.Hash.String())
	for _, tx := range block.Txs {
		for _, p := range ci.processor {
			p.ApplyTransaction(tx, height)
		}
	}
}

func (ci *catcherImpl) revertBlock(height uint32, block *types.Block) {
	ci.blocks.Delete(height)
	ci.currentHeight = height - 1
	for _, p := range ci.processor {
		p.RevertHeight(height)
	}
}

func (ci *catcherImpl) getCurrentBlock() (uint32, string) {
	count, err := ci.fetcher.Height()
	if err != nil {
		logger.Error(err)
	}
	blockHash, err := ci.fetcher.HashForHeight(count)
	if err != nil {
		logger.Error(err)
	}
	return count, blockHash
}
