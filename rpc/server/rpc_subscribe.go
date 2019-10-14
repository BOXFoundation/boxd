// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package rpc

import (
	"container/list"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/BOXFoundation/boxd/boxd/eventbus"
	"github.com/BOXFoundation/boxd/core/types"
	rpcpb "github.com/BOXFoundation/boxd/rpc/pb"
	"github.com/jbenet/goprocess"
	uuid "github.com/satori/go.uuid"
)

func (s *webapiServer) Connect(stream rpcpb.WebApi_ConnectServer) error {

	connuid := uuid.NewV1().String()

	defer func() {
		s.CloseConn(connuid)
		logger.Info("Close the persistent conn.")
	}()

	resultCh := make(chan *rpcpb.ListenedData)
	// register endpoints.
	p := s.proc.Go(func(p goprocess.Process) {
		for {
			select {
			case <-p.Closing():
				logger.Debug("Closing rpc stream")
				return
			default:
			}

			registerReq, err := stream.Recv()
			if err != nil {
				logger.Warn(err)
				break
			}
			if registerReq.Cancel {
				switch registerReq.Type {
				case BlockEp:
					s.unsubscribeBlockEndpoint(connuid)
				case LogEp:
					uid := registerReq.Info.(*rpcpb.RegisterReq_LogsReq).LogsReq.Uid
					s.unsubscribeLogEndpoint(connuid, uid)
				}
				continue
			}

			endpoint, ok := s.endpoints[registerReq.Type]
			if !ok {
				logger.Errorf("Api not supported: %d", registerReq.Type)
				continue
			}

			var ch chan bool
			var uid string
			switch registerReq.Type {
			case BlockEp:
				if _, ch, err = s.subscribeBlockEndpoint(connuid); err != nil {
					logger.Error(err)
					continue
				}
				go s.listenBlocks(endpoint, resultCh, ch)
			case LogEp:
				if uid, ch, err = s.subscribeLogEndpoint(connuid); err != nil {
					logger.Error(err)
					continue
				}
				go s.listenLogs(registerReq.Info.(*rpcpb.RegisterReq_LogsReq), endpoint, resultCh, ch)
				if err := stream.Send(
					&rpcpb.ListenedData{
						Type: RegisterResp, Data: &rpcpb.ListenedData_Info{
							Info: &rpcpb.RegisterDetails{Uid: uid}}}); err != nil {
					logger.Warnf("Webapi send uid data error %v", err)
				}
			default:
				logger.Errorf("Register type not found: %d", registerReq.Type)
			}

		}
	})

	for {
		select {
		case data := <-resultCh:
			if err := stream.Send(data); err != nil {
				logger.Warnf("Webapi send data error %v", err)
			}
		case <-s.proc.Closing():
			return nil
		case <-p.Closing():
			return nil
		}
	}
}

func (s *webapiServer) listenBlocks(
	endpoint Endpoint, resultCh chan *rpcpb.ListenedData, cancelCh <-chan bool,
) {
	logger.Info("start listen new blocks")
	defer func() {
		if err := endpoint.Unsubscribe(); err != nil {
			logger.Error(err)
		}
	}()

	var (
		elm  *list.Element
		exit bool
	)
	for {
		select {
		case <-cancelCh:
			return
		default:
		}

		if s.Closing() {
			logger.Info("receive closing signal, exit ListenAndReadNewBlock ...")
			return
		}
		rwmtx := endpoint.GetEventMutex()
		rwmtx.RLock()
		if endpoint.GetQueue().Len() != 0 {
			elm = endpoint.GetQueue().Front()
			rwmtx.RUnlock()
			break
		}
		rwmtx.RUnlock()
		time.Sleep(100 * time.Millisecond)
	}
	for {
		select {
		case <-cancelCh:
			logger.Info("Unsubscribe Blocks.")
			return
		default:
		}
		// get block
		blockDetail := elm.Value.(*rpcpb.BlockDetail)
		resultCh <- &rpcpb.ListenedData{Type: BlockEp, Data: &rpcpb.ListenedData_Block{Block: blockDetail}}
		logger.Debugf("webapi server sent a block, hash: %s, height: %d",
			blockDetail.Hash, blockDetail.Height)
		if elm, exit = s.moveToNextElem(endpoint, elm); exit {
			return
		}
	}
}

func (s *webapiServer) listenLogs(
	logsreq *rpcpb.RegisterReq_LogsReq, endpoint Endpoint,
	resultCh chan *rpcpb.ListenedData, cancelCh <-chan bool,
) error {

	if logsreq == nil {
		return fmt.Errorf("Invalid params")
	}
	req := logsreq.LogsReq

	logger.Info("start listen new logs")
	defer func() {
		if err := endpoint.Unsubscribe(); err != nil {
			logger.Error(err)
		}
	}()
	var (
		elm  *list.Element
		exit bool
	)
	for {
		select {
		case <-cancelCh:
			return nil
		default:
		}

		if s.Closing() {
			logger.Info("receive closing signal, exit ListenAndReadNewLog ...")
			return nil
		}
		rwmtx := endpoint.GetEventMutex()
		rwmtx.RLock()
		if endpoint.GetQueue().Len() != 0 {
			elm = endpoint.GetQueue().Front()
			rwmtx.RUnlock()
			break
		}
		rwmtx.RUnlock()
		time.Sleep(100 * time.Millisecond)
	}

	for {
		select {
		case <-cancelCh:
			return nil
		default:
		}
		// get logs
		logs := elm.Value.(map[string][]*types.Log)

		filteredLogs := []*types.Log{}
		topicslist := [][][]byte{[][]byte{}}
		for _, addr := range req.Addresses {
			if l, ok := logs[addr]; ok {
				filteredLogs = append(filteredLogs, l...)
			}
			contractAddress, err := types.NewContractAddress(addr)
			if err != nil {
				logger.Error(err)
				return err
			}
			topicslist[0] = append(topicslist[0], contractAddress.Hash())
		}

		for i, topiclist := range req.Topics {
			topicslist = append(topicslist, [][]byte{})
			for _, topic := range topiclist.Topics {
				t, err := hex.DecodeString(topic)
				if err != nil {
					logger.Error(err)
					return err
				}
				topicslist[i+1] = append(topicslist[i+1], t)
			}
		}

		filteredLogs, err := s.ChainBlockReader.FilterLogs(filteredLogs, topicslist)
		if err != nil {
			logger.Error(err)
			return err
		}

		resultLogs := ToPbLogs(filteredLogs)
		if len(resultLogs) != 0 {
			resultCh <- &rpcpb.ListenedData{
				Type: LogEp,
				Data: &rpcpb.ListenedData_Logs{Logs: &rpcpb.LogDetails{Logs: resultLogs}}}
			logger.Debugf("webapi server sent a log, data: %v", resultLogs)
		}
		if elm, exit = s.moveToNextElem(endpoint, elm); exit {
			return nil
		}
	}
}

func (s *webapiServer) moveToNextElem(
	endpoint Endpoint, elm *list.Element,
) (elmA *list.Element, exit bool) {
	// move to next element
	for {
		if s.Closing() {
			logger.Info("receive closing signal, exit ListenAndReadNewBlock ...")
			return nil, true
		}
		rwmtx := endpoint.GetEventMutex()
		rwmtx.RLock()
		if next := elm.Next(); next != nil {
			elmA = next
		} else if elm.Prev() == nil {
			// if this element is removed, move to the front of list
			elmA = endpoint.GetQueue().Front()
		}
		// occur when elm is the front
		if elmA != nil && elm != elmA {
			rwmtx.RUnlock()
			return elmA, false
		}
		rwmtx.RUnlock()
		time.Sleep(100 * time.Millisecond)
	}
}

func (s *webapiServer) subscribeBlockEndpoint(connuid string) (string, chan bool, error) {
	uid, ch, err := s.subscribe([]string{connuid, "blocks"}...)
	if err != nil {
		return "", nil, err
	}
	err = s.endpoints[BlockEp].Subscribe()
	if err != nil {
		return "", nil, err
	}
	return uid, ch, nil
}

func (s *webapiServer) subscribeLogEndpoint(connuid string) (string, chan bool, error) {
	uid, ch, err := s.subscribe(connuid)
	if err != nil {
		return "", nil, err
	}
	err = s.endpoints[LogEp].Subscribe()
	if err != nil {
		return "", nil, err
	}
	return uid, ch, nil
}

func (s *webapiServer) unsubscribeBlockEndpoint(connuid string) {
	s.Unsubscribe(connuid, "blocks")
}

func (s *webapiServer) unsubscribeLogEndpoint(connuid, uid string) {
	s.Unsubscribe(connuid, uid)
}

// Key to endpoints.
const (
	BlockEp uint32 = iota
	LogEp
	RegisterResp

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

	chainReader ChainBlockReader
	txReader    TxPoolReader
}

// NewBlockEndpoint return a new block endpoint.
func NewBlockEndpoint(
	bus eventbus.Bus, chainReader ChainBlockReader, txReader TxPoolReader,
) *BlockEndpoint {
	return &BlockEndpoint{
		bus:         bus,
		chainReader: chainReader,
		txReader:    txReader,
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
	logger.Warnf("subscribe new blocks#%d", bep.subscribeCnt)
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
	logger.Warnf("unsubscribe new blocks#%d", bep.subscribeCnt)
	return nil
}

func (bep *BlockEndpoint) receiveNewBlockMsg(block *types.Block) {
	bep.eventMtx.Lock()
	defer bep.eventMtx.Unlock()

	if bep.GetQueue().Len() == newBlockMsgSize {
		bep.queue.Remove(bep.queue.Front())
	}
	logger.Debugf("webapiServer receives a block, hash: %s, height: %d",
		block.BlockHash(), block.Header.Height)
	// detail block
	blockDetail, err := detailBlock(block, bep.chainReader, bep.txReader, true)
	if err != nil {
		logger.Warnf("detail block %s height %d error: %s",
			block.BlockHash(), block.Header.Height, err)
		return
	}
	_, n, err := bep.chainReader.ReadBlockFromDB(block.BlockHash())
	if err != nil {
		logger.Warn(err)
	}
	blockDetail.Size_ = uint32(n)
	// push
	bep.queue.PushBack(blockDetail)
	logger.Debugf("webapi server sent a block, hash: %s, height: %d",
		blockDetail.Hash, blockDetail.Height)
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
	logger.Warnf("subscribe new logs#%d", lep.subscribeCnt)
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
	logger.Warnf("unsubscribe new logs#%d", lep.subscribeCnt)
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
