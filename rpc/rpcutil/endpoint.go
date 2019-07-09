// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package rpcutil

import (
	"container/list"
	"fmt"
	"sync"

	"github.com/BOXFoundation/boxd/boxd/eventbus"
	"github.com/BOXFoundation/boxd/core/types"
	rpcpb "github.com/BOXFoundation/boxd/rpc/pb"
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

	Subscribe(...string) error
	Unsubscribe(...string) error
}

// BlockEndpoint is an endpoint to push new blocks.
type BlockEndpoint struct {
	queue        *list.List
	subscribeCnt int

	eventMtx *sync.RWMutex
	mtx      sync.Mutex
	bus      eventbus.Bus
}

// NewBlockEndpoint return a new block endpoint.
func NewBlockEndpoint(bus eventbus.Bus) *BlockEndpoint {
	return &BlockEndpoint{
		bus: bus,
	}
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
func (bep *BlockEndpoint) Subscribe(...string) error {
	bep.mtx.Lock()
	defer bep.mtx.Unlock()
	if bep.subscribeCnt == 0 {
		err := bep.bus.SubscribeUniq(eventbus.TopicRPCSendNewBlock, bep.receiveNewBlockMsg)
		if err != nil {
			return err
		}
	}
	bep.subscribeCnt++
	logger.Infof("subscribe new blocks#%d", bep.subscribeCnt)
	return nil
}

// Unsubscribe unsubscribe the topic of new blocks.
func (bep *BlockEndpoint) Unsubscribe(...string) error {
	bep.mtx.Lock()
	defer bep.mtx.Unlock()
	if bep.subscribeCnt == 1 {
		err := bep.bus.Unsubscribe(eventbus.TopicRPCSendNewBlock, bep.receiveNewBlockMsg)
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
	subscribeCnt map[string]uint8

	eventMtx *sync.RWMutex
	mtx      sync.Mutex
	bus      eventbus.Bus
}

// NewLogEndpoint return a new log endpoint.
func NewLogEndpoint(bus eventbus.Bus) *LogEndpoint {
	return &LogEndpoint{
		bus:          bus,
		subscribeCnt: make(map[string]uint8),
	}
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
func (lep *LogEndpoint) Subscribe(addrs ...string) error {
	if len(addrs) == 0 {
		return fmt.Errorf("Need a contract address to subscribe")
	}
	lep.mtx.Lock()
	defer lep.mtx.Unlock()
	for _, addr := range addrs {
		if cnt, ok := lep.subscribeCnt[addr]; !ok || cnt == 0 {
			err := lep.bus.SubscribeUniq(eventbus.TopicRPCSendNewLog+addr, lep.receiveNewLog)
			if err != nil {
				return err
			}
			lep.subscribeCnt[addr] = 1
		} else {
			lep.subscribeCnt[addr]++
		}
	}
	logger.Infof("subscribe new logs#%d", lep.subscribeCnt)
	return nil
}

// Unsubscribe unsubscribe the topic of new logs.
func (lep *LogEndpoint) Unsubscribe(addrs ...string) error {
	if len(addrs) == 0 {
		return fmt.Errorf("Need a contract address to subscribe")
	}
	lep.mtx.Lock()
	defer lep.mtx.Unlock()
	for _, addr := range addrs {
		if cnt, ok := lep.subscribeCnt[addr]; ok && cnt == 1 {
			err := lep.bus.Unsubscribe(eventbus.TopicRPCSendNewLog+addr, lep.receiveNewLog)
			if err != nil {
				return err
			}
			delete(lep.subscribeCnt, addr)
		} else if ok && cnt > 1 {
			lep.subscribeCnt[addr]--
		}
	}
	logger.Infof("unsubscribe new logs#%d", lep.subscribeCnt)
	return nil
}

func (lep *LogEndpoint) receiveNewLog(logs []*types.Log) {
	lep.eventMtx.Lock()
	defer lep.eventMtx.Unlock()

	if lep.GetQueue().Len() == newLogMsgSize {
		lep.queue.Remove(lep.queue.Front())
	}
	lep.queue.PushBack(logs)
}

// ToPbLogs convert []*types.Log to []*rpcpb.Logs_LogDetail.
func ToPbLogs(logs []*types.Log) []*rpcpb.Logs_LogDetail {
	ret := []*rpcpb.Logs_LogDetail{}
	for _, log := range logs {
		topics := [][]byte{}
		for _, topic := range log.Topics {
			topics = append(topics, topic.Bytes())
		}
		ret = append(ret, &rpcpb.Logs_LogDetail{
			Address:     log.Address.Bytes(),
			Topics:      topics,
			Data:        log.Data,
			BlockNumber: log.BlockNumber,
			TxHash:      log.TxHash.Bytes(),
			TxIndex:     log.TxIndex,
			BlockHash:   log.BlockHash.Bytes(),
			Index:       log.Index,
			Removed:     log.Removed,
		})
	}
	return ret
}
