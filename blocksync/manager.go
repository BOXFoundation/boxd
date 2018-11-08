// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package blocksync

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/BOXFoundation/boxd/boxd/eventbus"
	"github.com/BOXFoundation/boxd/consensus/dpos"
	"github.com/BOXFoundation/boxd/core"
	"github.com/BOXFoundation/boxd/core/chain"
	coreTypes "github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/crypto"
	"github.com/BOXFoundation/boxd/log"
	"github.com/BOXFoundation/boxd/p2p"
	"github.com/BOXFoundation/boxd/util"
	"github.com/jbenet/goprocess"
	peer "github.com/libp2p/go-libp2p-peer"
)

var logger = log.NewLogger("blocksync") // logger

var (
	errNoPeerToSync = errors.New("no peer to sync")
	errCheckFailed  = errors.New("check failed")
	errNoResponding = errors.New("no responding when node is in sync")
	zeroHash        = &crypto.HashType{}
)

type peerStatus int
type syncStatus int
type errFlag int

func (s syncStatus) String() string {
	switch s {
	case freeStatus:
		return "freeStatus"
	case locateStatus:
		return "locateStatus"
	case checkStatus:
		return "checkStatus"
	case blocksStatus:
		return "blocksStatus"
	default:
		return "unknown syncStatus"
	}
}

const (
	maxSyncFailedTimes = 100
	maxCheckPeers      = 2
	syncBlockChunkSize = 16
	locatorSeqPartLen  = 6
	syncTimeout        = 5 * time.Second
	blocksTimeout      = 20 * time.Second
	retryTimes         = 20
	retryInterval      = 2 * time.Second
	maxSyncTries       = 100

	availablePeerStatus peerStatus = iota
	locatePeerStatus
	checkedPeerStatus
	blocksPeerStatus
	errPeerStatus
	timeoutPeerStatus

	freeStatus syncStatus = iota
	locateStatus
	checkStatus
	blocksStatus
	// err falg
	errFlagNoHash errFlag = iota
	errFlagUnmarshal
	errFlagWrongPeerStatus
	errFlagCheckNumTooBig
	errFlagRootHashMismatch
)

type blockCheckInfo struct {
	rootHash *crypto.HashType
	fbh      *FetchBlockHeaders
}

// SyncManager syncs blocks with peers
type SyncManager struct {
	status  syncStatus
	statMtx sync.RWMutex
	// ProcessBlock mutex
	processMtx sync.Mutex
	// peers number that need to check headers
	checkNum int32
	// root hash for check hashes
	checkRootHash *crypto.HashType
	// blocks hash needed to fetch from others peer
	fetchHashes []*crypto.HashType
	// check info for blocks from remote nodes
	peerBlockCheckInfo map[peer.ID][]*blockCheckInfo
	// contain begin hash and length indicate the check hashes
	checkHash *CheckHash
	// peers who local peer has checked to or synchronized to
	stalePeers   map[peer.ID]peerStatus
	blocksSynced int32
	// server started only once
	svrStarted int32

	proc      goprocess.Process
	chain     *chain.BlockChain
	consensus *dpos.Dpos
	p2pNet    p2p.Net

	messageCh         chan p2p.Message
	locateErrCh       chan errFlag
	locateDoneCh      chan struct{}
	checkErrCh        chan struct{}
	checkOkCh         chan struct{}
	syncErrCh         chan struct{}
	blocksDoneCh      chan struct{}
	blocksErrCh       chan FetchBlockHeaders
	blocksProcessedCh chan struct{}
}

func (sm *SyncManager) reset() {
	atomic.StoreInt32(&sm.checkNum, 0)
	sm.checkRootHash = &crypto.HashType{}
	sm.fetchHashes = make([]*crypto.HashType, 0)
	sm.peerBlockCheckInfo = make(map[peer.ID][]*blockCheckInfo)
	sm.checkHash = nil
	sm.blocksSynced = 0
}

func (sm *SyncManager) resetAll() {
	sm.reset()
	sm.setStatus(freeStatus)
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

// NewSyncManager returns new block sync manager.
func NewSyncManager(blockChain *chain.BlockChain, p2pNet p2p.Net, consensus *dpos.Dpos, parent goprocess.Process) *SyncManager {
	return &SyncManager{
		status:       freeStatus,
		chain:        blockChain,
		consensus:    consensus,
		p2pNet:       p2pNet,
		proc:         goprocess.WithParent(parent),
		stalePeers:   make(map[peer.ID]peerStatus),
		messageCh:    make(chan p2p.Message, 512),
		locateErrCh:  make(chan errFlag),
		locateDoneCh: make(chan struct{}),
		checkErrCh:   make(chan struct{}),
		checkOkCh:    make(chan struct{}),
		syncErrCh:    make(chan struct{}),
		blocksDoneCh: make(chan struct{}),
		blocksErrCh:  make(chan FetchBlockHeaders),
		blocksProcessedCh: make(chan struct{},
			chain.MaxBlocksPerSync/syncBlockChunkSize),
	}
}

// Run start sync task and handle sync message
func (sm *SyncManager) Run() {
	// Already started?
	if i := atomic.AddInt32(&sm.svrStarted, 1); i != 1 {
		logger.Infof("SyncManager server has started. no tried %d", i)
		return
	}
	logger.Info("start Run")
	sm.subscribeMessageNotifiee()
	go sm.handleSyncMessage()
}

// StartSync start sync block message from remote peers.
func (sm *SyncManager) StartSync() {
	if sm.getStatus() != freeStatus {
		return
	}
	logger.Info("StartSync")
	sm.consensus.StopMint()
	go sm.startSync()
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
	p2p.UpdateSynced(false)
	// prevent startSync being executed again
	sm.setStatus(locateStatus)
	// sleep 5s to wait for connections to establish
	time.Sleep(5 * time.Second)
	//
	defer func() {
		sm.consensus.RecoverMint()
		sm.resetAll()
		p2p.UpdateSynced(true)
		logger.Info("sync completed and exit!")
	}()

	// timer for locate, check and sync
	timer := time.NewTimer(syncTimeout)
	needMore := false
	tries := 0
out_sync:
	for {
		if !needMore {
			tries++
			if tries == maxSyncTries {
				logger.Warnf("exceed max retry times(%d)", maxSyncTries)
				return
			}
		}
		needMore = false
		// start block sync.
		sm.reset()
		sm.setStatus(locateStatus)
		if err := sm.locateHashes(); err != nil {
			logger.Warn("locateHashes error: ", err)
			time.Sleep(retryInterval)
			continue
		}
		logger.Info("wait locateHashes done")
		timer.Reset(syncTimeout)
	out_locate:
		for {
			select {
			case <-sm.locateDoneCh:
				logger.Info("success to locate, start check")
				timer.Stop()
				break out_locate
			case ef := <-sm.locateErrCh:
				// no hash sent from locate peer, no need to sync
				if ef == errFlagNoHash {
					return
				}
				logger.Infof("SyncManager locate wrong, restart sync")
				continue out_sync
			case <-timer.C:
				logger.Info("timeout for locate and exit sync!")
				continue out_sync
			case <-sm.proc.Closing():
				logger.Info("Quit handle sync msg loop.")
				timer.Stop()
				return
			}
		}

		sm.setStatus(checkStatus)
		if err := sm.checkHashes(); err != nil {
			logger.Info("checkHashes error: ", err)
			continue out_sync
		}
		logger.Info("wait checkHashes done")
		timer.Reset(syncTimeout)
	out_check:
		for {
			select {
			case <-sm.checkOkCh:
				if sm.checkPass() {
					logger.Infof("success to check, start to sync")
					timer.Stop()
					break out_check
				}
			case <-sm.checkErrCh:
				logger.Infof("SyncManager check failed, checkHash: %+v, restart sync",
					sm.checkHash)
				continue out_sync
			case <-timer.C:
				logger.Info("timeout for check and exit sync!")
				continue out_sync
			case <-sm.proc.Closing():
				logger.Info("Quit handle sync msg loop.")
				timer.Stop()
				return
			}
		}

		sm.setStatus(blocksStatus)
		sm.fetchAllBlocks(sm.fetchHashes)
		logger.Infof("wait sync %d blocks done", len(sm.fetchHashes))
		timer.Reset(blocksTimeout)
		for {
			select {
			case <-sm.blocksDoneCh:
				if sm.completed() {
					logger.Infof("complete to sync %d blocks", len(sm.fetchHashes))
					sm.waitAllBlocksProcessed()
					logger.Infof("complete to process %d blocks", len(sm.fetchHashes))
					timer.Stop()
					if sm.moreSync() {
						logger.Info("start next sync ...")
						needMore = true
						continue out_sync
					} else {
						sm.blocksProcessedCh = make(chan struct{},
							chain.MaxBlocksPerSync/syncBlockChunkSize)
						return
					}
				}
			case fbh := <-sm.blocksErrCh:
				logger.Warnf("fetch blocks error, retry for %+v", fbh)
				timer.Stop()
				_, err := sm.fetchRemoteBlocksWithRetry(&fbh, retryTimes, retryInterval)
				if err != nil {
					logger.Warn(err)
					sm.blocksProcessedCh = make(chan struct{},
						chain.MaxBlocksPerSync/syncBlockChunkSize)
					return
				}
				timer.Reset(blocksTimeout)
			case <-timer.C:
				logger.Info("timeout for sync blocks and exit sync!")
				// retry uncompleted FetchBlockHeaders
				for _, infos := range sm.peerBlockCheckInfo {
					for _, v := range infos {
						if v == nil {
							continue
						}
						_, err := sm.fetchRemoteBlocksWithRetry(v.fbh, retryTimes,
							retryInterval)
						if err != nil {
							logger.Warn(err)
							sm.blocksProcessedCh = make(chan struct{},
								chain.MaxBlocksPerSync/syncBlockChunkSize)
							return
						}
					}
				}
				timer.Reset(blocksTimeout)
			case <-sm.proc.Closing():
				logger.Info("Quit handle sync msg loop.")
				timer.Stop()
				return
			}
		}
	}
}

func (sm *SyncManager) locateHashes() error {
	// get block locator hashes
	hashes, err := sm.getLatestBlockLocator()
	if err != nil {
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
	sm.checkHash = newCheckHash(sm.fetchHashes[0], uint32(len(sm.fetchHashes)))
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

func (sm *SyncManager) fetchAllBlocks(hashes []*crypto.HashType) error {
	total := uint32(len(hashes))
	for i := uint32(0); i < total; i += syncBlockChunkSize {
		var length uint32
		if i+syncBlockChunkSize <= total {
			length = syncBlockChunkSize
		} else {
			length = total - i
		}
		idx := i / syncBlockChunkSize
		fbh := newFetchBlockHeaders(idx, hashes[i], length)
		pid, err := sm.fetchRemoteBlocksWithRetry(fbh, retryTimes, retryInterval)
		if err != nil {
			panic(fmt.Errorf("fetchRemoteBlocks(%v) exceed max retry times(%d)",
				fbh, retryTimes))
		}
		merkleRoot := util.BuildMerkleRoot(hashes[i : i+length])
		rootHash := merkleRoot[len(merkleRoot)-1]
		sm.peerBlockCheckInfo[pid] = append(sm.peerBlockCheckInfo[pid],
			&blockCheckInfo{fbh: fbh, rootHash: rootHash})
	}
	return nil
}

func (sm *SyncManager) fetchRemoteBlocksWithRetry(fbh *FetchBlockHeaders,
	times int, interval time.Duration) (peer.ID, error) {
	for i := 0; i < times; i++ {
		if pid, err := sm.fetchRemoteBlocks(fbh); err == nil {
			return pid, nil
		}
		time.Sleep(interval)
	}
	return peer.ID(""), fmt.Errorf("fetchRemoteBlocks(%v) exceed max retry "+
		"times(%d)", fbh, times)
}

func (sm *SyncManager) fetchRemoteBlocks(fbh *FetchBlockHeaders) (peer.ID, error) {
	pid, err := sm.pickOnePeer()
	if err != nil {
		return peer.ID(""), fmt.Errorf("select peer to sync blocks error: %s", err)
	}
	logger.Infof("send message[0x%X] body:%+v to peer %s", p2p.BlockChunkRequest,
		fbh, pid.Pretty())
	return pid, sm.p2pNet.SendMessageToPeer(p2p.BlockChunkRequest, fbh, pid)
}

func (sm *SyncManager) onLocateRequest(msg p2p.Message) error {
	sm.chain.Bus().Publish(eventbus.TopicConnEvent, msg.From(), eventbus.SyncMsgEvent)

	// not to been sync when the node is in sync status
	if sm.getStatus() != freeStatus {
		return errNoResponding
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
		sm.locateErrCh <- errFlagUnmarshal
		return err
	}
	if len(sh.Hashes) == 0 {
		logger.Infof("onLocateResponse receive no Header from peer[%s], "+
			"try another peer to sync", msg.From().Pretty())
		sm.locateErrCh <- errFlagNoHash
		return nil
	}
	logger.Infof("onLocateResponse receive %d hashes", len(sh.Hashes))
	// get headers hashes needed to sync
	hashes, err := sm.rmOverlap(sh.Hashes)
	if err != nil {
		logger.Infof("rmOverlap error: %s", err)
		return err
	}
	logger.Infof("onLocateResponse get syncHeaders %d hashes", len(hashes))
	sm.fetchHashes = hashes
	merkleRoot := util.BuildMerkleRoot(hashes)
	sm.checkRootHash = merkleRoot[len(merkleRoot)-1]
	sm.locateDoneCh <- struct{}{}
	return nil
}

func (sm *SyncManager) onCheckRequest(msg p2p.Message) error {
	sm.chain.Bus().Publish(eventbus.TopicConnEvent, msg.From(), eventbus.SyncMsgEvent)
	// not to been sync when the node is in sync status
	if sm.getStatus() != freeStatus {
		return errNoResponding
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
	sm.chain.Bus().Publish(eventbus.TopicConnEvent, msg.From(), eventbus.SyncMsgEvent)
	// not to been sync when the node is in sync status
	if sm.getStatus() != freeStatus {
		return errNoResponding
	}
	//
	sb := newSyncBlocks(0)
	defer func() {
		logger.Infof("send message[0x%X] %d blocks to peer %s",
			p2p.LocateCheckResponse, len(sb.Blocks), msg.From().Pretty())
		err = sm.p2pNet.SendMessageToPeer(p2p.BlockChunkResponse, sb, msg.From())
	}()
	// parse response
	fbh := newFetchBlockHeaders(0, nil, 0)
	err = fbh.Unmarshal(msg.Body())
	if err != nil {
		logger.Infof("onBlocksRequest error: %s", err)
		return
	}
	// fetch blocks from local main chain
	blocks, err := sm.chain.FetchNBlockAfterSpecificHash(*fbh.BeginHash, fbh.Length)
	if err != nil {
		logger.Infof("onBlocksRequest fetchLocalBlocks FetchBlockHeaders: %+v"+
			" error: %s", fbh, err)
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
	//if !sm.isPeerStatusFor(locatePeerStatus, msg.From()) {
	if sm.isPeerStatusFor(availablePeerStatus, pid) {
		sm.syncErrCh <- struct{}{}
		return fmt.Errorf("receive BlockChunkResponse from non-sync peer[%s]",
			pid.Pretty())
	}
	// parse response
	sb := new(SyncBlocks)
	if err := sb.Unmarshal(msg.Body()); err != nil {
		sm.syncErrCh <- struct{}{}
		return err
	}
	// check blocks merkle root hash
	if fbh, ok := sm.checkBlocksAndClearInfo(sb, pid); !ok {
		if fbh != nil {
			sm.blocksErrCh <- *fbh
		}
		return fmt.Errorf("onBlocksResponse check failed from peer: %s, "+
			"SyncBlocks: %+v", pid.Pretty(), sb)
	}
	// process blocks
	go func() {
		for _, b := range sb.Blocks {
			sm.processMtx.Lock()
			_, _, err := sm.chain.ProcessBlock(b, false, false)
			sm.processMtx.Unlock()
			if err != nil {
				if err == core.ErrBlockExists || err == core.ErrOrphanBlockExists {
					continue
				} else {
					panic(err)
				}
			}
		}
		sm.blocksProcessedCh <- struct{}{}
	}()
	count := atomic.AddInt32(&sm.blocksSynced, int32(len(sb.Blocks)))
	logger.Infof("has sync %d/(%d) blocks, current peer[%s]",
		count, len(sm.fetchHashes), pid.Pretty())
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

func heightLocator(height uint32) []uint32 {
	var h uint32
	heights := make([]uint32, 0)
	// get sequential portion
	for i := uint32(0); i < locatorSeqPartLen; i++ {
		h = height - i
		heights = append(heights, h)
		if h == 0 {
			return heights
		}
	}
	// get skip portion
	for step := uint32(1); h > step+1; step = step * 2 {
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
		_, err := sm.chain.LoadBlockByHash(*h)
		if err == core.ErrBlockIsNil {
			return locateHashes[i:], nil
		} else if err != nil {
			return nil, err
		}
	}
	return nil, errors.New("no header needed to sync")
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
	return atomic.LoadInt32(&sm.blocksSynced) == int32(len(sm.fetchHashes))
}

func (sm *SyncManager) waitAllBlocksProcessed() {
	for i := 0; i < len(sm.fetchHashes); i += syncBlockChunkSize {
		<-sm.blocksProcessedCh
	}
}

func (sm *SyncManager) moreSync() bool {
	return len(sm.fetchHashes) == chain.MaxBlocksPerSync
}

func (sm *SyncManager) checkBlocksAndClearInfo(sb *SyncBlocks, pid peer.ID) (
	*FetchBlockHeaders, bool) {
	checkInfos, ok := sm.peerBlockCheckInfo[pid]
	if !ok {
		return nil, false
	}
	rootHash := *merkleRootHashForBlocks(sb.Blocks)
	for i, v := range checkInfos {
		if v != nil && v.fbh.Idx == sb.Idx {
			if rootHash != *v.rootHash {
				return v.fbh, false
			}
			checkInfos[i] = nil
			return nil, true
		}
	}
	// remove checkInfos pointed to pid if all elements in checkInfos is nil
	for i, v := range checkInfos {
		if v != nil {
			break
		}
		if i == len(checkInfos)-1 {
			delete(sm.peerBlockCheckInfo, pid)
		}
	}
	return nil, false
}

func merkleRootHashForBlocks(blocks []*coreTypes.Block) *crypto.HashType {
	hashes := make([]*crypto.HashType, 0, len(blocks))
	for _, b := range blocks {
		hashes = append(hashes, b.BlockHash())
	}
	merkleRoot := util.BuildMerkleRoot(hashes)
	return merkleRoot[len(merkleRoot)-1]
}
