// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package blocksync

import (
	"fmt"
	"math"
	"sync/atomic"

	"github.com/BOXFoundation/boxd/boxd/eventbus"
	"github.com/BOXFoundation/boxd/core"
	"github.com/BOXFoundation/boxd/p2p"
	"github.com/BOXFoundation/boxd/util"
)

// Run start sync task and handle sync message
func (sm *SyncManager) Run() {
	// Already started?
	if i := atomic.AddInt32(&sm.svrStarted, 1); i != 1 {
		logger.Infof("SyncManager server has started. no tried %d", i)
		return
	}
	logger.Info("Succeed to start sync service.")
	sm.subscribeMessageNotifiee()
	go sm.handleSyncMessage()
}

func (sm *SyncManager) subscribeMessageNotifiee() {
	sm.p2pNet.Subscribe(p2p.NewNotifiee(p2p.LocateForkPointRequest, p2p.Repeatable, sm.messageCh))
	sm.p2pNet.Subscribe(p2p.NewNotifiee(p2p.LocateForkPointResponse, p2p.Repeatable, sm.messageCh))
	sm.p2pNet.Subscribe(p2p.NewNotifiee(p2p.LocateCheckRequest, p2p.Repeatable, sm.messageCh))
	sm.p2pNet.Subscribe(p2p.NewNotifiee(p2p.LocateCheckResponse, p2p.Repeatable, sm.messageCh))
	sm.p2pNet.Subscribe(p2p.NewNotifiee(p2p.BlockChunkRequest, p2p.Repeatable, sm.messageCh))
	sm.p2pNet.Subscribe(p2p.NewNotifiee(p2p.BlockChunkResponse, p2p.Repeatable, sm.messageCh))
}

func (sm *SyncManager) handleSyncMessage() {
	var err error
	for {
		select {
		case msg := <-sm.messageCh:
			logger.Debugf("Receive msg[0x%X] from peer %s", msg.Code(), msg.From().Pretty())
			switch msg.Code() {
			case p2p.LocateForkPointRequest:
				err = sm.onLocateRequest(msg)
			case p2p.LocateForkPointResponse:
				err = sm.onLocateResponse(msg)
			case p2p.LocateCheckRequest:
				err = sm.onCheckRequest(msg)
			case p2p.LocateCheckResponse:
				err = sm.onCheckResponse(msg)
			case p2p.BlockChunkRequest:
				err = sm.onBlocksRequest(msg)
			case p2p.BlockChunkResponse:
				err = sm.onBlocksResponse(msg)
			default:
				logger.Warn("Failed to handle sync msg, unknow msg code")
			}
			if err != nil {
				logger.Warnf("Failed to handle SyncMessage[0x%X]. Err: %v", msg.Code(), err)
			}
		case <-sm.proc.Closing():
			logger.Info("Quit handle sync msg loop.")
			return
		}
	}
}

func (sm *SyncManager) onLocateRequest(msg p2p.Message) error {
	sm.chain.Bus().Publish(eventbus.TopicConnEvent, msg.From(), eventbus.SyncMsgEvent)

	// not to been sync when the node is in sync status
	if sm.getStatus() != freeStatus {
		logger.Infof("send message[0x%X] zeroHash to peer %s for in sync status",
			p2p.LocateForkPointResponse, msg.From().Pretty())
		return sm.p2pNet.SendMessageToPeer(p2p.LocateForkPointResponse,
			newSyncHeaders(zeroHash), msg.From())
	}
	// parse response
	lh := new(LocateHeaders)
	if err := lh.Unmarshal(msg.Body()); err != nil {
		return err
	}
	//
	hashes, err := sm.chain.LocateForkPointAndFetchHeaders(lh.Hashes)
	if err != nil {
		logger.Warnf("onLocateRequest fetch headers error: %s, hashes: %+v",
			err, lh.Hashes)
		return err
	}
	logger.Infof("onLocateRequest send %d hashes", len(hashes))
	// send SyncHeaders hashes to active end
	sh := newSyncHeaders(hashes...)
	logger.Infof("send message[0x%X] (%d hashes) to peer %s",
		p2p.LocateForkPointResponse, len(hashes), msg.From().Pretty())
	return sm.p2pNet.SendMessageToPeer(p2p.LocateForkPointResponse, sh, msg.From())
}

func (sm *SyncManager) onLocateResponse(msg p2p.Message) error {
	if sm.getStatus() != locateStatus {
		return fmt.Errorf("onLocateResponse returns since now status is %s",
			sm.getStatus())
	}
	pid := msg.From()
	if !sm.verifyPeerStatus(locatePeerStatus, pid) {
		sm.stalePeers.Store(pid, errPeerStatus)
		tryPushErrFlagChan(sm.locateErrCh, errFlagWrongPeerStatus)
		return fmt.Errorf("receive LocateForkPointResponse from non-sync peer[%s]",
			pid.Pretty())
	}
	// parse response
	sh := new(SyncHeaders)
	if err := sh.Unmarshal(msg.Body()); err != nil ||
		(len(sh.Hashes) == 1 && *sh.Hashes[0] == *zeroHash) {
		logger.Infof("onLocateResponse unmarshal msg.Body[%+v] error: %v",
			msg.Body(), err)
		sm.stalePeers.Store(pid, errPeerStatus)
		tryPushErrFlagChan(sm.locateErrCh, errFlagUnmarshal)
		return err
	}
	if len(sh.Hashes) == 0 {
		logger.Infof("onLocateResponse receive no Header from peer[%s], "+
			"try another peer to sync", pid.Pretty())
		sm.stalePeers.Store(pid, errPeerStatus)
		tryPushErrFlagChan(sm.locateErrCh, errFlagNoHash)
		return nil
	}
	logger.Infof("onLocateResponse receive %d hashes", len(sh.Hashes))
	// get headers hashes needed to sync
	hashes := sm.rmOverlap(sh.Hashes)
	if hashes == nil {
		tryPushErrFlagChan(sm.locateErrCh, errFlagNoHash)
		return nil
	}
	logger.Infof("onLocateResponse get syncHeaders %d hashes", len(hashes))
	sm.fetchHashes = hashes
	merkleRoot := util.BuildMerkleRoot(hashes)
	sm.checkRootHash = merkleRoot[len(merkleRoot)-1]
	sm.stalePeers.Store(pid, locateDonePeerStatus)
	tryPushEmptyChan(sm.locateDoneCh)
	return nil
}

func (sm *SyncManager) onCheckRequest(msg p2p.Message) error {
	sm.chain.Bus().Publish(eventbus.TopicConnEvent, msg.From(), eventbus.SyncMsgEvent)
	// not to been sync when the node is in sync status
	if sm.getStatus() != freeStatus {
		logger.Infof("send message[0x%X] zeroHash to peer %s",
			p2p.LocateCheckResponse, msg.From().Pretty())
		return sm.p2pNet.SendMessageToPeer(p2p.LocateCheckResponse,
			newSyncCheckHash(zeroHash), msg.From())
	}
	// parse response
	ch := new(CheckHash)
	if err := ch.Unmarshal(msg.Body()); err != nil {
		return err
	}
	logger.Infof("receive CheckHash: %+v", ch)
	//
	hash, err := sm.chain.CalcRootHashForNBlocks(*ch.BeginHash, ch.Length)
	if err != nil {
		logger.Warnf("onCheckRequest calc root hash for %+v error: %s", ch, err)
	}
	sh := newSyncCheckHash(hash)
	logger.Infof("send message[0x%X] body[%+v] to peer %s", p2p.LocateCheckResponse,
		sh, msg.From().Pretty())
	return sm.p2pNet.SendMessageToPeer(p2p.LocateCheckResponse, sh, msg.From())
}

func (sm *SyncManager) onCheckResponse(msg p2p.Message) error {
	if sm.getStatus() != checkStatus {
		return fmt.Errorf("onCheckResponse returns since now status is %s",
			sm.getStatus())
	}
	pid := msg.From()
	if !sm.verifyPeerStatus(checkedPeerStatus, pid) &&
		!sm.verifyPeerStatus(checkedDonePeerStatus, pid) {
		sm.stalePeers.Store(pid, errPeerStatus)
		tryPushEmptyChan(sm.checkErrCh)
		return fmt.Errorf("receive LocateCheckResponse from non-sync peer[%s]",
			pid.Pretty())
	}
	checkNum := atomic.AddInt32(&sm.checkNum, 1)
	if checkNum > int32(maxCheckPeers) {
		sm.stalePeers.Store(pid, errPeerStatus)
		tryPushEmptyChan(sm.checkErrCh)
		return fmt.Errorf("receive too many LocateCheckResponse from check peer[%s]",
			pid.Pretty())
	}
	// parse response
	sch := new(SyncCheckHash)
	if err := sch.Unmarshal(msg.Body()); err != nil {
		sm.stalePeers.Store(pid, errPeerStatus)
		tryPushEmptyChan(sm.checkErrCh)
		return err
	}
	// check rootHash
	if *sm.checkRootHash != *sch.RootHash {
		logger.Warnf("check RootHash mismatch from peer[%s], recv: %v, want: %v",
			pid.Pretty(), sch.RootHash, sm.checkRootHash)
		sm.stalePeers.Store(pid, errPeerStatus)
		tryPushEmptyChan(sm.checkErrCh)
		return nil
	}
	sm.stalePeers.Store(pid, checkedDonePeerStatus)
	tryPushEmptyChan(sm.checkOkCh)
	logger.Infof("success to check %d times", checkNum)
	return nil
}

func (sm *SyncManager) onBlocksRequest(msg p2p.Message) (err error) {
	sm.chain.Bus().Publish(eventbus.TopicConnEvent, msg.From(), eventbus.SyncMsgEvent)
	// not to been sync when the node is in sync status
	if sm.getStatus() != freeStatus {
		logger.Infof("send message[0x%X] maxUint32 Idx blocks body to peer %s",
			p2p.BlockChunkResponse, msg.From().Pretty())
		return sm.p2pNet.SendMessageToPeer(p2p.BlockChunkResponse,
			newSyncBlocks(math.MaxUint32), msg.From())
	}
	//
	sb := newSyncBlocks(0)
	defer func() {
		logger.Infof("send message[0x%X] %d blocks to peer %s",
			p2p.BlockChunkResponse, len(sb.Blocks), msg.From().Pretty())
		err = sm.p2pNet.SendMessageToPeer(p2p.BlockChunkResponse, sb, msg.From())
	}()
	// parse response
	fbh := newFetchBlockHeaders(0, nil, 0)
	err = fbh.Unmarshal(msg.Body())
	if err != nil {
		logger.Warnf("[onBlocksRequest]Failed to unmarshal blockHeaders. Err: %v", err)
		return
	}
	// fetch blocks from local main chain
	blocks, err := sm.chain.FetchNBlockAfterSpecificHash(*fbh.BeginHash, fbh.Length)
	if err != nil {
		logger.Warnf("[onBlocksRequest]Failed to fetch blocks after specific hash. BlockHeaders: %+v, Err: %v", fbh, err)
		return
	}
	sb = newSyncBlocks(fbh.Idx, blocks...)
	return
}

func (sm *SyncManager) onBlocksResponse(msg p2p.Message) error {
	if sm.getStatus() != blocksStatus {
		return fmt.Errorf("onBlocksResponse returns since now status is %s",
			sm.getStatus())
	}
	pid := msg.From()
	if !sm.verifyPeerStatus(blocksPeerStatus, pid) &&
		!sm.verifyPeerStatus(blocksDonePeerStatus, pid) {
		sm.stalePeers.Store(pid, errPeerStatus)
		tryPushEmptyChan(sm.syncErrCh)
		return fmt.Errorf("receive BlockChunkResponse from non-sync peer[%s]",
			pid.Pretty())
	}
	// parse response
	sb := new(SyncBlocks)
	if err := sb.Unmarshal(msg.Body()); err != nil || len(sb.Blocks) == 0 ||
		sb.Idx == math.MaxUint32 {
		sm.stalePeers.Store(pid, errPeerStatus)
		tryPushEmptyChan(sm.syncErrCh)
		return fmt.Errorf("Failed to unmarshal syncblocks. Err: %v or receive no blocks(%d) "+
			"or msg.From is in sync(idx: %d)", err, len(sb.Blocks), sb.Idx)
	}
	count := atomic.AddInt32(&sm.blocksSynced, int32(len(sb.Blocks)))
	logger.Infof("has sync %d/%d blocks, current peer[%s]",
		count, len(sm.fetchHashes), pid.Pretty())
	maxChanLen := (len(sm.fetchHashes) + syncBlockChunkSize - 1) / syncBlockChunkSize
	// check blocks merkle root hash
	if fbh, ok := sm.checkBlocksAndClearInfo(sb, pid); !ok {
		sm.stalePeers.Store(pid, errPeerStatus)
		if fbh != nil {
			if len(sm.blocksErrCh) == maxChanLen {
				logger.Warnf("blocksErrCh overflow for len: %d", len(sm.blocksErrCh))
				return nil
			}
			tryPushBlockHeadersChan(sm.blocksErrCh, *fbh)
		}
		return fmt.Errorf("onBlocksResponse check failed from peer: %s, "+
			"SyncBlocks: %+v", pid.Pretty(), sb)
	}
	sm.stalePeers.Store(pid, blocksDonePeerStatus)
	if len(sm.blocksDoneCh) == maxChanLen {
		sm.stalePeers.Store(pid, errPeerStatus)
		logger.Warnf("blocksDownCh overflow for len: %d", len(sm.blocksDoneCh))
		return nil
	}
	tryPushEmptyChan(sm.blocksDoneCh)
	// process blocks
	go func() {
		for _, b := range sb.Blocks {
			err := sm.chain.ProcessBlock(b, false, false)
			if err != nil {
				if err == core.ErrBlockExists || err == core.ErrOrphanBlockExists {
					continue
				} else {
					panic(err)
				}
			}
		}
		tryPushEmptyChan(sm.blocksProcessedCh)
	}()
	return nil
}
