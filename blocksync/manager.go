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

	"github.com/BOXFoundation/boxd/consensus/dpos"
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
	syncBlockChunkSize = 64
	locatorSeqPartLen  = 6
	syncTimeout        = 5 * time.Second
	blocksTimeout      = 10 * time.Second
	retryTimes         = 10
	retryInterval      = 1 * time.Second
	maxSyncTries       = 20
)

const (
	availablePeerStatus peerStatus = iota
	locatePeerStatus
	locateDonePeerStatus
	checkedPeerStatus
	checkedDonePeerStatus
	blocksPeerStatus
	blocksDonePeerStatus
	errPeerStatus
)

const (
	freeStatus syncStatus = iota
	locateStatus
	checkStatus
	blocksStatus
)

// err falg
const (
	errFlagNoHash errFlag = iota
	errFlagInSync
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
	stalePeers   *sync.Map
	blocksSynced int32
	// server started only once
	svrStarted int32
	// last the end located hash received that been remove by rmOverlap method
	lastLocatedHash *crypto.HashType

	proc      goprocess.Process
	chain     *chain.BlockChain
	consensus *dpos.Dpos
	p2pNet    p2p.Net

	messageCh         chan p2p.Message
	locateErrCh       chan errFlag
	locateDoneCh      chan struct{}
	checkErrCh        chan errFlag
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
	sm.stalePeers = new(sync.Map)
	sm.drainAllSyncChan()
}

// NewSyncManager returns new block sync manager.
func NewSyncManager(blockChain *chain.BlockChain, p2pNet p2p.Net,
	consensus *dpos.Dpos, parent goprocess.Process) *SyncManager {
	return &SyncManager{
		status:       freeStatus,
		chain:        blockChain,
		consensus:    consensus,
		p2pNet:       p2pNet,
		proc:         goprocess.WithParent(parent),
		stalePeers:   new(sync.Map),
		messageCh:    make(chan p2p.Message, 512),
		locateErrCh:  make(chan errFlag),
		locateDoneCh: make(chan struct{}),
		checkErrCh:   make(chan errFlag),
		checkOkCh:    make(chan struct{}, maxCheckPeers),
		syncErrCh:    make(chan struct{}),
		blocksDoneCh: make(chan struct{},
			chain.MaxBlocksPerSync/syncBlockChunkSize),
		blocksErrCh: make(chan FetchBlockHeaders,
			chain.MaxBlocksPerSync/syncBlockChunkSize),
		blocksProcessedCh: make(chan struct{},
			chain.MaxBlocksPerSync/syncBlockChunkSize),
	}
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

// ActiveLightSync active light sync from remote peer.
func (sm *SyncManager) ActiveLightSync(pid peer.ID) error {
	if sm.getStatus() != freeStatus {
		return errors.New("Peer is in sync")
	}
	sm.consensus.StopMint()
	hashes, err := sm.getLatestBlockLocator()
	if err != nil {
		return err
	}
	logger.Infof("Active light sync. remote peerID: %s", pid.Pretty())
	data := newLocateHeaders(hashes...)

	return sm.p2pNet.SendMessageToPeer(p2p.LightSyncRequest, data, pid)
}

func (sm *SyncManager) startSync() {
	p2p.UpdateSynced(false)
	// prevent startSync being executed again
	sm.setStatus(locateStatus)
	// sleep 1s to wait for connections to establish
	time.Sleep(time.Second)
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
		// locate header to sync
		sm.reset()
		sm.drainAllSyncChan()
		sm.setStatus(locateStatus)
		if err := sm.locateHashes(); err != nil {
			logger.Warn("locateHashes error: ", err)
			time.Sleep(retryInterval)
			continue
		}
		logger.Info("wait locateHashes done")
		drainTimer(timer.C)
		timer.Reset(syncTimeout)
	out_locate:
		for {
			select {
			case <-sm.locateDoneCh:
				logger.Info("success to locate, start check")
				cleanStopTimer(timer)
				break out_locate
			case ef := <-sm.locateErrCh:
				// no hash sent from locate peer, no need to sync
				if ef == errFlagNoHash && sm.lastLocatedHash == nil {
					return
				}
				logger.Infof("SyncManager locate error %d, restart sync", ef)
				continue out_sync
			case <-timer.C:
				logger.Info("timeout for locate and restart sync!")
				sm.setTimeoutPeersErrStatus(locatePeerStatus)
				continue out_sync
			case <-sm.proc.Closing():
				logger.Info("Quit handle sync msg loop.")
				cleanStopTimer(timer)
				return
			}
		}
		sm.lastLocatedHash = nil

		// find other Peers to check header
		sm.setStatus(checkStatus)
		sm.drainCheckChan()
		if err := sm.checkHashes(maxCheckPeers); err != nil {
			logger.Info("checkHashes error: ", err)
			continue out_sync
		}
		logger.Info("wait checkHashes done")
		drainTimer(timer.C)
		timer.Reset(syncTimeout)
	out_check:
		for {
			select {
			case <-sm.checkOkCh:
				if sm.checkPass() {
					logger.Infof("success to check, start to sync")
					cleanStopTimer(timer)
					break out_check
				}
			case ef := <-sm.checkErrCh:
				if ef != errFlagRootHashMismatch {
					if err := sm.checkHashes(1); err != nil {
						logger.Info("retry checkHashes error: ", err)
						continue out_sync
					}
					continue out_check
				}
				logger.Infof("SyncManager check failed, checkHash: %+v, restart sync",
					sm.checkHash)
				continue out_sync
			case <-timer.C:
				logger.Info("timeout for check and restart sync!")
				sm.setTimeoutPeersErrStatus(checkedPeerStatus)
				continue out_sync
			case <-sm.proc.Closing():
				logger.Info("Quit handle sync msg loop.")
				cleanStopTimer(timer)
				return
			}
		}

		// sync blocks
		sm.setStatus(blocksStatus)
		sm.drainBlocksChan()
		if err := sm.fetchAllBlocks(sm.fetchHashes); err != nil {
			logger.Warn(err)
			sm.blocksProcessedCh = make(chan struct{},
				chain.MaxBlocksPerSync/syncBlockChunkSize)
			return
		}
		logger.Infof("wait sync %d blocks done", len(sm.fetchHashes))
		drainTimer(timer.C)
		timer.Reset(blocksTimeout)
		for {
			select {
			case <-sm.blocksDoneCh:
				if sm.completed() {
					logger.Infof("complete to sync %d blocks", len(sm.fetchHashes))
					sm.waitAllBlocksProcessed()
					logger.Infof("complete to process %d blocks", len(sm.fetchHashes))
					cleanStopTimer(timer)
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
			case <-sm.syncErrCh:
				logger.Infof("sync blocks error")
			case fbh := <-sm.blocksErrCh:
				logger.Warnf("fetch blocks error, retry for %+v", fbh)
				cleanStopTimer(timer)
				_, err := sm.fetchRemoteBlocksWithRetry(&fbh, retryTimes, retryInterval)
				if err != nil {
					logger.Warn(err)
					sm.blocksProcessedCh = make(chan struct{},
						chain.MaxBlocksPerSync/syncBlockChunkSize)
					return
				}
				drainTimer(timer.C)
				timer.Reset(blocksTimeout)
			case <-timer.C:
				logger.Info("timeout for sync some blocks and retry these blocks' sync!")
				sm.setTimeoutPeersErrStatus(blocksPeerStatus)
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
				cleanStopTimer(timer)
				return
			}
		}
	}
}

func (sm *SyncManager) locateHashes() error {
	// get block locator hashes
	var (
		hashes []*crypto.HashType
		err    error
	)
	if sm.lastLocatedHash != nil {
		hashes = []*crypto.HashType{sm.lastLocatedHash}
	} else {
		hashes, err = sm.getLatestBlockLocator()
		if err != nil {
			return err
		}
	}
	logger.Infof("locateHashes get lastestBlockLocator %d hashes", len(hashes))
	lh := newLocateHeaders(hashes...)
	// select one peer to sync
	pid, err := sm.pickOnePeer(locateStatus)
	if err == errNoPeerToSync {
		return err
	}
	sm.stalePeers.Store(pid, locatePeerStatus)
	logger.Infof("send message[0x%X] (begin: %s, len %d) to peer %s",
		p2p.LocateForkPointRequest, hashes[len(hashes)-1], len(hashes), pid.Pretty())
	return sm.p2pNet.SendMessageToPeer(p2p.LocateForkPointRequest, lh, pid)
}

func (sm *SyncManager) checkHashes(checkTimes int) error {
	// select peers to check
	peers := make([]peer.ID, 0, maxCheckPeers)
	for i := 0; i < checkTimes; i++ {
		pid, err := sm.pickOnePeer(checkStatus)
		if err != nil {
			return err
		}
		sm.stalePeers.Store(pid, checkedPeerStatus)
		peers = append(peers, pid)
	}
	// send msg to checking peers
	// sm.fetachHashes can not be nil
	sm.checkHash = newCheckHash(sm.fetchHashes[0], uint32(len(sm.fetchHashes)))
	for _, pid := range peers {
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
			return err
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
	pid, err := sm.pickOnePeer(blocksStatus)
	if err != nil {
		return peer.ID(""), fmt.Errorf("select peer to sync blocks error: %s", err)
	}
	logger.Infof("send message[0x%X] body:%+v to peer %s", p2p.BlockChunkRequest,
		fbh, pid.Pretty())
	sm.stalePeers.Store(pid, blocksPeerStatus)
	return pid, sm.p2pNet.SendMessageToPeer(p2p.BlockChunkRequest, fbh, pid)
}

func (sm *SyncManager) verifyPeerStatus(status peerStatus, id peer.ID) bool {
	s, ok := sm.stalePeers.Load(id)
	return ok && (s != nil && s.(peerStatus) == status)
}

// getLatestBlockLocator returns a block locator that comprises @beginSeqHashes
// and log2(height-@beginSeqHashes) entries to geneses for the skip portion.
func (sm *SyncManager) getLatestBlockLocator() ([]*crypto.HashType, error) {
	hashes := make([]*crypto.HashType, 0)
	tailHeight := sm.chain.TailBlock().Header.Height
	if tailHeight == 0 {
		return []*crypto.HashType{sm.chain.Genesis().BlockHash()}, nil
	}
	heights := heightLocator(tailHeight)
	for _, h := range heights {
		b, err := sm.chain.LoadBlockByHeight(h)
		if err != nil {
			return nil, fmt.Errorf("LoadBlockByHeight error: %s, h: %d, heights: %v",
				err, h, heights)
		}
		hashes = append(hashes, b.BlockHash())
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
func (sm *SyncManager) rmOverlap(locateHashes []*crypto.HashType) []*crypto.HashType {
	for i, h := range locateHashes {
		if block, _ := chain.LoadBlockByHash(*h, sm.chain.DB()); block == nil {
			return locateHashes[i:]
		}
	}
	return nil
}

func (sm *SyncManager) pickOnePeer(syncStatus syncStatus) (peer.ID, error) {
	ids := make([]peer.ID, 0)
	var preferedID peer.ID
	sm.stalePeers.Range(func(k, v interface{}) bool {
		// if now is in syncStatus, prefer to select locate or check peers
		if syncStatus == blocksStatus &&
			(v.(peerStatus) == locateDonePeerStatus ||
				v.(peerStatus) == checkedDonePeerStatus) {
			synced, existed := sm.p2pNet.PeerSynced(k.(peer.ID))
			if existed && synced {
				preferedID = k.(peer.ID)
				return false
			}
		}
		ids = append(ids, k.(peer.ID))
		return true
	})
	if preferedID != peer.ID("") {
		return preferedID, nil
	}
	var pid peer.ID
	var syncIds []peer.ID
	for {
		pid = sm.p2pNet.PickOnePeer(ids...)
		if pid == peer.ID("") {
			break
		}
		synced, _ := sm.p2pNet.PeerSynced(pid)
		if synced {
			return pid, nil
		}
		syncIds = append(syncIds, pid)
		ids = append(ids, pid)
	}
	// select a peer that have sync with this peer when no other peers to sync
	ids = make([]peer.ID, 0)
	sm.stalePeers.Range(func(k, v interface{}) bool {
		if (v != nil && v.(peerStatus) == errPeerStatus) ||
			util.InArray(k.(peer.ID), syncIds) {
			ids = append(ids, k.(peer.ID))
		}
		return true
	})
	for {
		pid = sm.p2pNet.PickOnePeer(ids...)
		if pid == peer.ID("") {
			return pid, errNoPeerToSync
		}
		synced, _ := sm.p2pNet.PeerSynced(pid)
		if synced {
			return pid, nil
		}
		ids = append(ids, pid)
	}
}

func (sm *SyncManager) setTimeoutPeersErrStatus(status peerStatus) {
	sm.stalePeers.Range(func(k, v interface{}) bool {
		if v != nil && v.(peerStatus) == status {
			sm.stalePeers.Store(k, errPeerStatus)
		}
		return true
	})
}

func (sm *SyncManager) checkPass() bool {
	return atomic.LoadInt32(&sm.checkNum) >= int32(maxCheckPeers)
}

func (sm *SyncManager) completed() bool {
	return atomic.LoadInt32(&sm.blocksSynced) >= int32(len(sm.fetchHashes))
}

func (sm *SyncManager) waitAllBlocksProcessed() {
	for i := 0; i < len(sm.fetchHashes); i += syncBlockChunkSize {
		<-sm.blocksProcessedCh
	}
}

func (sm *SyncManager) moreSync() bool {
	return len(sm.fetchHashes) >= chain.MaxBlocksPerSync
}

func (sm *SyncManager) checkBlocksAndClearInfo(sb *SyncBlocks, pid peer.ID) (
	*FetchBlockHeaders, bool) {
	checkInfos, ok := sm.peerBlockCheckInfo[pid]
	if !ok {
		return nil, false
	}
	ok = false
	rootHash := *zeroHash
	if len(sb.Blocks) > 0 {
		rootHash = *merkleRootHashForBlocks(sb.Blocks)
	}
	for i, v := range checkInfos {
		if v != nil && v.fbh.Idx == sb.Idx {
			if rootHash != *v.rootHash {
				return v.fbh, false
			}
			checkInfos[i] = nil
			ok = true
			break
		}
	}
	// remove checkInfos pointed to pid if all elements in checkInfos is nil
	for _, v := range checkInfos {
		if v != nil {
			return nil, ok
		}
	}
	delete(sm.peerBlockCheckInfo, pid)
	return nil, ok
}

func merkleRootHashForBlocks(blocks []*coreTypes.Block) *crypto.HashType {
	hashes := make([]*crypto.HashType, 0, len(blocks))
	for _, b := range blocks {
		hashes = append(hashes, b.BlockHash())
	}
	merkleRoot := util.BuildMerkleRoot(hashes)
	return merkleRoot[len(merkleRoot)-1]
}

func drainTimer(ch <-chan time.Time) {
	select {
	case <-ch:
	default:
	}
}

func cleanStopTimer(t *time.Timer) {
	t.Stop()
	drainTimer(t.C)
}

func (sm *SyncManager) drainAllSyncChan() {
	sm.drainLocateChan()
	sm.drainCheckChan()
	sm.drainBlocksChan()
}

func (sm *SyncManager) drainLocateChan() {
	tryPopErrFlagChan(sm.locateErrCh)
	tryPopEmptyChan(sm.locateDoneCh)
}

func (sm *SyncManager) drainCheckChan() {
	tryPopErrFlagChan(sm.checkErrCh)
	for {
		if !tryPopEmptyChan(sm.checkOkCh) {
			break
		}
	}
}

func (sm *SyncManager) drainBlocksChan() {
	for {
		if !tryPopEmptyChan(sm.blocksDoneCh) {
			break
		}
	}
	for {
		if !tryPopBlockHeadersChan(sm.blocksErrCh) {
			break
		}
	}
	for {
		if !tryPopEmptyChan(sm.blocksProcessedCh) {
			break
		}
	}
}

func tryPopEmptyChan(ch <-chan struct{}) bool {
	select {
	case <-ch:
		logger.Info("pop a struct{}{} from chan struct{}")
		return true
	default:
		return false
	}
}

func tryPushEmptyChan(ch chan<- struct{}) bool {
	select {
	case ch <- struct{}{}:
		return true
	default:
		logger.Info("cannot push a struct{}{} to chan struct{}")
		return false
	}
}

func tryPopErrFlagChan(ch <-chan errFlag) bool {
	select {
	case v := <-ch:
		logger.Warnf("pop value: %v from channel", v)
		return true
	default:
		return false
	}
}

func tryPushErrFlagChan(ch chan<- errFlag, v errFlag) bool {
	select {
	case ch <- v:
		return true
	default:
		logger.Info("cannot push %v to chan errFag", v)
		return false
	}
}

func tryPopBlockHeadersChan(ch <-chan FetchBlockHeaders) bool {
	select {
	case v := <-ch:
		logger.Warnf("pop value: %v from channel", v)
		return true
	default:
		return false
	}
}

func tryPushBlockHeadersChan(ch chan<- FetchBlockHeaders, v FetchBlockHeaders) bool {
	select {
	case ch <- v:
		return true
	default:
		logger.Info("cannot push %v to chan FetchBlockHeaders", v)
		return false
	}
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
