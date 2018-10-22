// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package netsync

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/BOXFoundation/boxd/consensus"
	"github.com/BOXFoundation/boxd/core/chain"
	coreTypes "github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/crypto"
	"github.com/BOXFoundation/boxd/log"
	"github.com/BOXFoundation/boxd/p2p"
	"github.com/BOXFoundation/boxd/util"
	"github.com/jbenet/goprocess"
	peer "github.com/libp2p/go-libp2p-peer"
)

var (
	logger          = log.NewLogger("sync") // logger
	errNoPeerToSync = errors.New("no peer to sync")
	errCheckFailed  = errors.New("check failed")
	zeroHash        = &crypto.HashType{}
)

type peerStatus int
type syncStatus int

func (s syncStatus) String() string {
	switch s {
	case freeStatus:
		return "freeStatus"
	case locateStatus:
		return "locateStatus"
	case checkStatus:
		return "blockStatus"
	case blockStatus:
		return "blockStatus"
	default:
		return "unknown syncStatus"
	}
}

const (
	maxSyncFailedTimes = 100
	maxCheckPeers      = 2
	syncBlockChunkSize = 10
	beginSeqHashes     = 6

	availablePeerStatus peerStatus = iota
	locatePeerStatus
	checkedPeerStatus
	syncPeerStatus
	errPeerStatus
	timeoutPeerStatus

	freeStatus syncStatus = iota
	locateStatus
	checkStatus
	blockStatus
)

// SyncManager use to manage blocks and main chains to stay in sync
type SyncManager struct {
	status  syncStatus
	statMtx sync.RWMutex
	// peers number that need to check headers
	checkNum int32
	// root hash for check hashes
	checkRootHash *crypto.HashType
	// blocks hash needed to fetch from others peer
	fetchHashes []*crypto.HashType
	// contain begin hash and length indicate the check hashes
	checkHash *CheckHash
	// peers who local peer has checked to or synchronized to
	stalePeers    map[peer.ID]peerStatus
	hasSyncBlocks int32

	proc      goprocess.Process
	chain     *chain.BlockChain
	consensus consensus.Consensus
	p2pNet    p2p.Net

	messageCh    chan p2p.Message
	locateErrCh  chan struct{}
	locateDoneCh chan struct{}
	checkErrCh   chan struct{}
	checkOkCh    chan struct{}
	syncErrCh    chan struct{}
	blocksDoneCh chan struct{}
}

func (sm *SyncManager) reset() {
	sm.setStatus(freeStatus)
	atomic.StoreInt32(&sm.checkNum, 0)
	sm.checkRootHash = &crypto.HashType{}
	sm.fetchHashes = make([]*crypto.HashType, 0)
	sm.checkHash = nil
	sm.hasSyncBlocks = 0
}

func (sm *SyncManager) resetAll() {
	sm.reset()
	sm.stalePeers = make(map[peer.ID]peerStatus)
}

func (sm *SyncManager) setStatus(stat syncStatus) {
	sm.statMtx.Lock()
	sm.status = stat
	sm.statMtx.Unlock()
}

func (sm *SyncManager) getStatus() syncStatus {
	sm.statMtx.RLock()
	defer sm.statMtx.RUnlock()
	return sm.status
}

// NewSyncManager return new block sync manager.
func NewSyncManager(blockChain *chain.BlockChain, p2pNet p2p.Net,
	consensus consensus.Consensus, parent goprocess.Process) *SyncManager {
	return &SyncManager{
		status:       freeStatus,
		chain:        blockChain,
		consensus:    consensus,
		p2pNet:       p2pNet,
		proc:         goprocess.WithParent(parent),
		stalePeers:   make(map[peer.ID]peerStatus),
		messageCh:    make(chan p2p.Message, 512),
		locateErrCh:  make(chan struct{}),
		locateDoneCh: make(chan struct{}),
		checkErrCh:   make(chan struct{}),
		checkOkCh:    make(chan struct{}),
		syncErrCh:    make(chan struct{}),
		blocksDoneCh: make(chan struct{}),
	}
}

// Run start sync task and handle sync message
func (sm *SyncManager) Run() {
	logger.Info("start Run")
	sm.subscribeMessageNotifiee()
	go sm.handleSyncMessage()
}

// StartSync start sync block message from remote peers.
func (sm *SyncManager) StartSync() {
	logger.Info("StartSync")
	if sm.getStatus() != freeStatus {
		return
	}
	sm.consensus.StopMint()
	go sm.startSync()
}

func (sm *SyncManager) subscribeMessageNotifiee() {
	sm.p2pNet.Subscribe(p2p.NewNotifiee(p2p.LocateForkPointRequest, sm.messageCh))
	sm.p2pNet.Subscribe(p2p.NewNotifiee(p2p.LocateForkPointResponse, sm.messageCh))
	sm.p2pNet.Subscribe(p2p.NewNotifiee(p2p.LocateCheckRequest, sm.messageCh))
	sm.p2pNet.Subscribe(p2p.NewNotifiee(p2p.LocateCheckResponse, sm.messageCh))
	sm.p2pNet.Subscribe(p2p.NewNotifiee(p2p.BlockChunkRequest, sm.messageCh))
	sm.p2pNet.Subscribe(p2p.NewNotifiee(p2p.BlockChunkResponse, sm.messageCh))
}

func (sm *SyncManager) handleSyncMessage() {
	var err error
	for {
		select {
		case msg := <-sm.messageCh:
			logger.Infof("Receive msg[0x%X] from peer %s", msg.Code(), msg.From().Pretty())
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
				logger.Warnf("handleSyncMessage[0x%X] error: %s", msg.Code(), err)
			}
		case <-sm.proc.Closing():
			logger.Info("Quit handle sync msg loop.")
			return
		}
	}
}

func (sm *SyncManager) startSync() {
	// sleep 2s to wait connections established
	time.Sleep(5 * time.Second)
	//
	defer func() {
		sm.consensus.RecoverMint()
		sm.resetAll()
		logger.Info("sync completed and exit!")
	}()

	retries := 0
out_sync:
	for {
		retries++
		if retries == 100 {
			logger.Warn("exceed max retry times(200)")
			return
		}
		// start block sync.
		sm.reset()
		sm.setStatus(locateStatus)
		if err := sm.locateHashes(); err != nil {
			logger.Warn("locateHashes error: ", err)
			time.Sleep(3 * time.Second)
			continue
		}

		logger.Info("wait locateHashes done")
	out_locate:
		for {
			select {
			case <-sm.locateDoneCh:
				logger.Info("success to locate, start check")
				break out_locate
			case <-sm.locateErrCh:
				logger.Infof("SyncManager locate wrong, restart sync")
				continue out_sync
			case <-sm.proc.Closing():
				logger.Info("Quit handle sync msg loop.")
				return
			}
		}

		sm.setStatus(checkStatus)
		if err := sm.checkHashes(); err != nil {
			logger.Info("checkHashes error: ", err)
			continue out_sync
		}
		logger.Info("wait checkHashes done")
	out_check:
		for {
			select {
			case <-sm.checkOkCh:
				if sm.checkPass() {
					logger.Infof("success to check, start to sync")
					break out_check
				}
			case <-sm.checkErrCh:
				logger.Infof("SyncManager check failed, checkHash: %+v, restart sync",
					sm.checkHash)
				continue out_sync
			case <-sm.proc.Closing():
				logger.Info("Quit handle sync msg loop.")
				return
			}
		}

		sm.setStatus(blockStatus)
		sm.fetchBlocks(sm.fetchHashes)
		logger.Infof("wait sync %d blocks done", len(sm.fetchHashes))
		for {
			select {
			case <-sm.blocksDoneCh:
				if sm.completed() {
					logger.Infof("complete to sync %d blocks", len(sm.fetchHashes))
					if sm.moreSync() {
						logger.Info("start next sync ...")
						continue out_sync
					} else {
						return
					}
				}
			case <-sm.proc.Closing():
				logger.Info("Quit handle sync msg loop.")
				return
			}
		}
	}
}

func (sm *SyncManager) locateHashes() error {
	// get block locator hashes
	hashes, err := sm.getLatestBlockLocator()
	if err != nil {
		logger.Infof("getLatestBlockLocator error: %s", err)
		return err
	}
	logger.Infof("locateHashes get lastestBlockLocator %d hashes", len(hashes))
	lh := newLocateHeaders(hashes...)
	// select one peer to sync
	pid, err := sm.pickOnePeer()
	if err == errNoPeerToSync {
		return err
	}
	sm.stalePeers[pid] = locatePeerStatus
	logger.Infof("send message[0x%X] (%d hashes) to peer %s",
		p2p.LocateForkPointRequest, len(hashes), pid.Pretty())
	return sm.p2pNet.SendMessageToPeer(p2p.LocateForkPointRequest, lh, pid)
}

func (sm *SyncManager) checkHashes() error {
	// select peers to check
	peers := make([]peer.ID, 0, maxCheckPeers)
	for i := 0; i < maxCheckPeers; i++ {
		pid, err := sm.pickOnePeer()
		if err != nil {
			return err
		}
		peers = append(peers, pid)
	}
	// send msg to checking peers
	sm.checkHash = newCheckHash(sm.fetchHashes[0], len(sm.fetchHashes))
	for _, pid := range peers {
		sm.stalePeers[pid] = checkedPeerStatus
		logger.Infof("send message[0x%X] body[%+v] to peer %s",
			p2p.LocateCheckRequest, sm.checkHash, pid.Pretty())
		err := sm.p2pNet.SendMessageToPeer(p2p.LocateCheckRequest, sm.checkHash, pid)
		if err != nil {
			return err
		}
	}
	return nil
}

func (sm *SyncManager) fetchBlocks(hashes []*crypto.HashType) error {
	for i := 0; i < len(hashes); i += syncBlockChunkSize {
		pid, err := sm.pickOnePeer()
		if err != nil {
			logger.Infof("select peer to sync error: %s", err)
			// TODO: need to strict error handling method
			//if err == errNoPeerToSync {
			//	sm.syncErrCh <- struct{}{}
			//	break
			//}
			i -= syncBlockChunkSize
			continue
		}
		var fbh *FetchBlockHeaders
		if i+syncBlockChunkSize <= len(hashes) {
			fbh = newFetchBlockHeaders(hashes[i : i+syncBlockChunkSize]...)
		} else {
			fbh = newFetchBlockHeaders(hashes[i:]...)
		}
		logger.Infof("send message[0x%X] %d hashes to peer %s",
			p2p.BlockChunkRequest, len(fbh.Hashes), pid.Pretty())
		err = sm.p2pNet.SendMessageToPeer(p2p.BlockChunkRequest, fbh, pid)
		if err != nil {
			logger.Warn("SendMessageToPeer BlockChunkRequest error: ", err)
			i -= syncBlockChunkSize
			continue
		}
	}
	return nil
}

func (sm *SyncManager) onLocateRequest(msg p2p.Message) error {
	// parse response
	lh := new(LocateHeaders)
	if err := lh.Unmarshal(msg.Body()); err != nil {
		return err
	}
	//
	hashes, err := sm.chain.LocateForkPointAndFetchHeaders(lh.Hashes)
	if err != nil {
		logger.Infof("onLocateRequest fetch headers error: %s, hashes: %+v",
			err, lh.Hashes)
		return err
	}
	logger.Debugf("onLocateRequest send %d hashes", len(hashes))
	// send SyncHeaders hashes to active end
	sh := newSyncHeaders(hashes...)
	logger.Infof("send message[0x%X] (%d hashes) to peer %s",
		p2p.LocateForkPointResponse, len(hashes), msg.From().Pretty())
	return sm.p2pNet.SendMessageToPeer(p2p.LocateForkPointResponse, sh, msg.From())
}

func (sm *SyncManager) onLocateResponse(msg p2p.Message) error {
	if sm.getStatus() != locateStatus {
		return fmt.Errorf("onLocateResponse return since now status is %s",
			sm.getStatus())
	}
	//if !sm.isPeerStatusFor(locatePeerStatus, msg.From()) {
	if sm.isPeerStatusFor(availablePeerStatus, msg.From()) {
		return fmt.Errorf("receive LocateForkPointResponse from non-sync peer[%s]",
			msg.From().Pretty())
	}
	// parse response
	sh := new(SyncHeaders)
	if err := sh.Unmarshal(msg.Body()); err != nil {
		logger.Infof("onLocateResponse unmarshal msg.Body[%+v] error: %s",
			msg.Body(), err)
		sm.locateErrCh <- struct{}{}
		return err
	}
	if len(sh.Hashes) == 0 {
		logger.Infof("onLocateResponse receive no Header from peer[%s], "+
			"try another peer to sync", msg.From().Pretty())
		sm.locateErrCh <- struct{}{}
		return nil
	}
	logger.Debugf("onLocateResponse receive %d hashes", len(sh.Hashes))
	// get headers hashes needed to sync
	hashes, err := sm.rmOverlap(sh.Hashes)
	if err != nil {
		logger.Infof("rmOverlap error: %s", err)
		return err
	}
	logger.Debugf("onLocateResponse get syncHeaders %d hashes", len(hashes))
	sm.fetchHashes = hashes
	merkleRoot := util.BuildMerkleRoot(hashes)
	sm.checkRootHash = merkleRoot[len(merkleRoot)-1]
	sm.locateDoneCh <- struct{}{}
	return nil
}

func (sm *SyncManager) onCheckRequest(msg p2p.Message) error {
	//if sm.isPeerStatusFor(checkedPeerStatus, msg.From()) {
	if sm.isPeerStatusFor(availablePeerStatus, msg.From()) {
		sm.syncErrCh <- struct{}{}
		return fmt.Errorf("receive BlockChunkResponse from non-check peer[%s]",
			msg.From().Pretty())
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
		logger.Infof("onCheckRequest calc root hash for %+v error: %s", ch, err)
	}
	// if hashes == nil {}
	sh := newSyncCheckHash(hash)
	logger.Infof("send message[0x%X] body[%+v] to peer %s", p2p.LocateCheckResponse,
		sh, msg.From().Pretty())
	return sm.p2pNet.SendMessageToPeer(p2p.LocateCheckResponse, sh, msg.From())
}

func (sm *SyncManager) onCheckResponse(msg p2p.Message) error {
	if sm.getStatus() != checkStatus {
		return fmt.Errorf("onCheckResponse return since now status is %s",
			sm.getStatus())
	}
	//if !sm.isPeerStatusFor(checkPeerStatus, msg.From()) {
	if sm.isPeerStatusFor(availablePeerStatus, msg.From()) {
		sm.checkErrCh <- struct{}{}
		return fmt.Errorf("receive LocateCheckResponse from non-sync peer[%s]",
			msg.From().Pretty())
	}
	checkNum := atomic.AddInt32(&sm.checkNum, 1)
	if checkNum > int32(maxCheckPeers) {
		sm.checkErrCh <- struct{}{}
		return fmt.Errorf("receive too many LocateCheckResponse from check peer[%s]",
			msg.From().Pretty())
	}
	// parse response
	sch := new(SyncCheckHash)
	if err := sch.Unmarshal(msg.Body()); err != nil {
		sm.checkErrCh <- struct{}{}
		return err
	}
	// check rootHash
	if *sm.checkRootHash != *sch.RootHash {
		logger.Warnf("check RootHash mismatch from peer[%s], recv: %v, want: %v",
			msg.From().Pretty(), sch.RootHash, sm.checkRootHash)
		sm.checkErrCh <- struct{}{}
		return nil
	}
	sm.checkOkCh <- struct{}{}
	logger.Infof("success to check %d times", checkNum)
	return nil
}

func (sm *SyncManager) onBlocksRequest(msg p2p.Message) (err error) {
	sb := newSyncBlocks()
	defer func() {
		logger.Infof("send message[0x%X] %d blocks to peer %s",
			p2p.LocateCheckResponse, len(sb.Blocks), msg.From().Pretty())
		err = sm.p2pNet.SendMessageToPeer(p2p.BlockChunkResponse, sb, msg.From())
	}()
	// parse response
	fbh := newFetchBlockHeaders()
	err = fbh.Unmarshal(msg.Body())
	if err != nil {
		logger.Infof("onBlocksRequest error: %s", err)
	}
	// fetch blocks from local main chain
	blocks, err := sm.fetchLocalBlocks(fbh.Hashes)
	if err != nil {
		logger.Infof("onBlocksRequest fetchLocalBlocks error: %s", err)
	}
	sb = newSyncBlocks(blocks...)
	return
}

func (sm *SyncManager) onBlocksResponse(msg p2p.Message) error {
	if sm.getStatus() != blockStatus {
		return fmt.Errorf("onBlocksResponse return since now status is %s",
			sm.getStatus())
	}
	//if !sm.isPeerStatusFor(locatePeerStatus, msg.From()) {
	if sm.isPeerStatusFor(availablePeerStatus, msg.From()) {
		sm.syncErrCh <- struct{}{}
		return fmt.Errorf("receive BlockChunkResponse from non-sync peer[%s]",
			msg.From().Pretty())
	}
	// parse response
	sb := new(SyncBlocks)
	if err := sb.Unmarshal(msg.Body()); err != nil {
		sm.syncErrCh <- struct{}{}
		return err
	}
	// process blocks
	for _, b := range sb.Blocks {
		sm.chain.ProcessBlock(b, false)
	}
	count := atomic.AddInt32(&sm.hasSyncBlocks, int32(len(sb.Blocks)))
	logger.Infof("has sync %d/(%d) blocks, current peer[%s]",
		count, len(sm.fetchHashes), msg.From().Pretty())
	sm.blocksDoneCh <- struct{}{}
	return nil
}

func (sm *SyncManager) isPeerStatusFor(status peerStatus, id peer.ID) bool {
	s, ok := sm.stalePeers[id]
	return ok && s == status
}

// getLatestBlockLocator returns a block locator that comprises @beginSeqHashes
// and log2(height-@beginSeqHashes) entries to geneses for the skip portion.
func (sm *SyncManager) getLatestBlockLocator() ([]*crypto.HashType, error) {
	hashes := make([]*crypto.HashType, 0)
	tailHeight := sm.chain.TailBlock().Height
	if tailHeight == 0 {
		return []*crypto.HashType{&chain.GenesisHash}, nil
	}
	heights := heightLocator(tailHeight)
	for _, h := range heights {
		b, err := sm.chain.LoadBlockByHeight(h)
		if err != nil {
			return nil, fmt.Errorf("LoadBlockByHeight error: %s, h: %d, heights: %v",
				err, h, heights)
		}
		hashes = append(hashes, b.Hash)
	}
	return hashes, nil
}

func heightLocator(height int32) []int32 {
	var h int32
	heights := make([]int32, 0)
	if height < 0 {
		return heights
	}
	// get sequence headers portion
	for i := int32(0); i < beginSeqHashes; i++ {
		h = height - i
		if h < 0 {
			return heights
		}
		heights = append(heights, h)
	}
	// get skip portion
	for step := int32(1); h-step-1 > 0; step = step * 2 {
		h -= step + 1
		heights = append(heights, h)
	}
	if h > 0 {
		heights = append(heights, 0)
	}
	return heights
}

// rmOverlap remove overlapped headers between locateHashes and local chain
func (sm *SyncManager) rmOverlap(locateHashes []*crypto.HashType) (
	[]*crypto.HashType, error) {
	for i, h := range locateHashes {
		b, err := sm.chain.LoadBlockByHash(*h)
		if err != nil {
			return nil, err
		}
		if b == nil {
			return locateHashes[i:], nil
		}
	}
	return nil, errors.New("no header needed to sync")
}

func (sm *SyncManager) fetchLocalBlocks(hashes []*crypto.HashType) ([]*coreTypes.Block, error) {
	blocks := make([]*coreTypes.Block, 0, len(hashes))
	for _, h := range hashes {
		b, err := sm.chain.LoadBlockByHash(*h)
		if err != nil {
			return nil, err
		}
		if b == nil {
			return nil, fmt.Errorf("fetchLocalBlocks] block not existed for hash: %+v", h)
		}
		blocks = append(blocks, b)
	}
	return blocks, nil
}

func (sm *SyncManager) pickOnePeer() (peer.ID, error) {
	ids := make([]peer.ID, 0, len(sm.stalePeers))
	for k := range sm.stalePeers {
		ids = append(ids, k)
	}
	//pid := sm.p2pNet.PickOnePeer(ids...)
	pid := sm.p2pNet.PickOnePeer()
	if pid == peer.ID("") {
		return pid, errNoPeerToSync
	}
	return pid, nil
}

func (sm *SyncManager) checkPass() bool {
	return atomic.LoadInt32(&sm.checkNum) == int32(maxCheckPeers)
}

func (sm *SyncManager) completed() bool {
	return atomic.LoadInt32(&sm.hasSyncBlocks) == int32(len(sm.fetchHashes))
}

func (sm *SyncManager) moreSync() bool {
	return !sm.completed() && len(sm.fetchHashes) == chain.MaxBlockHeaderCountInSyncTask
}
