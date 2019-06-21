// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package rpcutil

import (
	"container/list"
	"sync"

	"github.com/BOXFoundation/boxd/boxd/eventbus"
	"github.com/BOXFoundation/boxd/core/types"
)

// Key to endpoints.
const (
	BlockEp = "block_endpoint"
	LogEp   = "log_endpoint"

	newBlockMsgSize = 60
	newLogMsgSize   = 100
)

// Endpoint is an interface for websocket endpoint.
type Endpoint interface {
	GetQueue() *list.List
	GetEventMutex() *sync.RWMutex

	Subscribe() error
	Unsubscribe() error
}

// BlockEndpoint is an endpoint to push new blocks.
type BlockEndpoint struct {
	queue        *list.List
	subscribeCnt int

	eventMtx *sync.RWMutex
	mtx      sync.Mutex
	Bus      eventbus.Bus
}

// GetQueue return blocks caching list.
func (bep *BlockEndpoint) GetQueue() *list.List {
	if bep.queue == nil {
		bep.queue = list.New()
	}
	return bep.queue
}

// GetEventMutex return eventMtx.
func (bep *BlockEndpoint) GetEventMutex() *sync.RWMutex {
	if bep.eventMtx == nil {
		bep.eventMtx = new(sync.RWMutex)
	}
	return bep.eventMtx
}

// Subscribe subscribe the topic of new blocks.
func (bep *BlockEndpoint) Subscribe() error {
	bep.mtx.Lock()
	defer bep.mtx.Unlock()
	if bep.subscribeCnt == 0 {
		err := bep.Bus.SubscribeUniq(eventbus.TopicRPCSendNewBlock, bep.receiveNewBlockMsg)
		if err != nil {
			return err
		}
	}
	bep.subscribeCnt++
	logger.Infof("subscribe new blocks#%d", bep.subscribeCnt)
	return nil
}

// Unsubscribe unsubscribe the topic of new blocks.
func (bep *BlockEndpoint) Unsubscribe() error {
	bep.mtx.Lock()
	defer bep.mtx.Unlock()
	if bep.subscribeCnt == 1 {
		err := bep.Bus.Unsubscribe(eventbus.TopicRPCSendNewBlock, bep.receiveNewBlockMsg)
		if err != nil {
			return err
		}
	}
	bep.subscribeCnt--
	logger.Infof("unsubscribe new blocks#%d", bep.subscribeCnt)
	return nil
}

func (bep *BlockEndpoint) receiveNewBlockMsg(block *types.Block) {
	bep.eventMtx.Lock()
	defer bep.eventMtx.Unlock()

	if bep.GetQueue().Len() == newBlockMsgSize {
		bep.queue.Remove(bep.queue.Front())
	}
	// detail block
	logger.Debugf("webapiServer receives a block, hash: %s, height: %d",
		block.BlockHash(), block.Header.Height)

	// push
	bep.queue.PushBack(block)
}

// LogEndpoint is an endpoint to push logs.
type LogEndpoint struct {
	queue        *list.List
	subscribeCnt int

	eventMtx *sync.RWMutex
	mtx      sync.Mutex
	Bus      eventbus.Bus
}

// GetQueue return logs caching list.
func (lep *LogEndpoint) GetQueue() *list.List {
	if lep.queue == nil {
		lep.queue = list.New()
	}
	return lep.queue
}

// GetEventMutex return eventMtx.
func (lep *LogEndpoint) GetEventMutex() *sync.RWMutex {
	if lep.eventMtx == nil {
		lep.eventMtx = new(sync.RWMutex)
	}
	return lep.eventMtx
}

// Subscribe subscribe the topic of new logs.
func (lep *LogEndpoint) Subscribe() error {
	lep.mtx.Lock()
	defer lep.mtx.Unlock()
	if lep.subscribeCnt == 0 {
		err := lep.Bus.SubscribeUniq(eventbus.TopicRPCSendNewLog, lep.receiveNewLog)
		if err != nil {
			return err
		}
	}
	lep.subscribeCnt++
	logger.Infof("subscribe new logs#%d", lep.subscribeCnt)
	return nil
}

// Unsubscribe unsubscribe the topic of new logs.
func (lep *LogEndpoint) Unsubscribe() error {
	lep.mtx.Lock()
	defer lep.mtx.Unlock()
	if lep.subscribeCnt == 1 {
		err := lep.Bus.Unsubscribe(eventbus.TopicRPCSendNewLog, lep.receiveNewLog)
		if err != nil {
			return err
		}
	}
	lep.subscribeCnt--
	logger.Infof("unsubscribe new logs#%d", lep.subscribeCnt)
	return nil
}

func (lep *LogEndpoint) receiveNewLog(logs []*types.Log) {
	lep.eventMtx.Lock()
	defer lep.eventMtx.Unlock()

	for _, log := range logs {
		if lep.GetQueue().Len() == newLogMsgSize {
			lep.queue.Remove(lep.queue.Front())
		}
		lep.queue.PushBack(log)
	}
}
