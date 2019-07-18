// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package rpcutil

import (
	"container/list"
	"encoding/hex"
	"sync"

	"github.com/BOXFoundation/boxd/boxd/eventbus"
	"github.com/BOXFoundation/boxd/core/types"
	rpcpb "github.com/BOXFoundation/boxd/rpc/pb"
)

// Key to endpoints.
const (
	BlockEp uint32 = iota
	LogEp

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
func (bep *BlockEndpoint) Subscribe() error {
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
func (bep *BlockEndpoint) Unsubscribe() error {
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
	subscribeCnt uint8

	eventMtx *sync.RWMutex
	mtx      sync.Mutex
	bus      eventbus.Bus
}

// NewLogEndpoint return a new log endpoint.
func NewLogEndpoint(bus eventbus.Bus) *LogEndpoint {
	return &LogEndpoint{
		bus:   bus,
		queue: list.New(),
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
func (lep *LogEndpoint) Subscribe() error {
	lep.mtx.Lock()
	defer lep.mtx.Unlock()
	if lep.subscribeCnt == 0 {
		err := lep.bus.SubscribeUniq(eventbus.TopicRPCSendNewLog, lep.receiveNewLog)
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
		err := lep.bus.Unsubscribe(eventbus.TopicRPCSendNewLog, lep.receiveNewLog)
		if err != nil {
			return err
		}
	}
	lep.subscribeCnt--
	logger.Infof("unsubscribe new logs#%d", lep.subscribeCnt)
	return nil
}

func (lep *LogEndpoint) receiveNewLog(logs map[string][]*types.Log) {
	lep.eventMtx.Lock()
	defer lep.eventMtx.Unlock()

	queue := lep.GetQueue()
	if queue.Len() == newLogMsgSize {
		queue.Remove(queue.Front())
	}
	queue.PushBack(logs)
}

// ToPbLogs convert []*types.Log to []*rpcpb.Logs_LogDetail.
func ToPbLogs(logs []*types.Log) []*rpcpb.LogDetail {
	ret := []*rpcpb.LogDetail{}
	for _, log := range logs {
		topics := []string{}
		for _, topic := range log.Topics {
			topics = append(topics, hex.EncodeToString(topic.Bytes()))
		}
		contractAddr, err := types.NewContractAddressFromHash(log.Address.Bytes()[:])
		if err != nil {
			logger.Errorf("ToPbLogs failed, Err: %v", err)
			return nil
		}
		ret = append(ret, &rpcpb.LogDetail{
			Address:     contractAddr.String(),
			Topics:      topics,
			Data:        hex.EncodeToString(log.Data),
			BlockNumber: log.BlockNumber,
			TxHash:      hex.EncodeToString(log.TxHash.Bytes()),
			TxIndex:     log.TxIndex,
			BlockHash:   hex.EncodeToString(log.BlockHash.Bytes()),
			Index:       log.Index,
			Removed:     log.Removed,
		})
	}
	return ret
}
