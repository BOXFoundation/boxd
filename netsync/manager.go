// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package netsync

import (
	"errors"
	"fmt"
	"time"

	"github.com/BOXFoundation/boxd/consensus"
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

var (
	logger          = log.NewLogger("sync") // logger
	errNoPeerToSync = errors.New("no peer to sync")
	errCheckFailed  = errors.New("check failed")
	zeroHash        = &crypto.HashType{}
)

type peerStatus int
type syncStatus int

const (
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
	checkFailedStatus
	syncBlockStatus
)

// SyncManager use to manage blocks and main chains to stay in sync
type SyncManager struct {
	status syncStatus
	// peers number that need to check headers
	checkNum int
	// root hash for check hashes
	checkRootHash *crypto.HashType
	// blocks hash needed to fetch from others peer
	fetchHashes []*crypto.HashType
	// contain begin hash and length indicate the check hashes
	checkHash *CheckHash
	// peers who local peer has checked to or synchronized to
	stalePeers    map[peer.ID]peerStatus
	hasSyncBlocks int

	proc      goprocess.Process
	chain     *chain.BlockChain
	consensus consensus.Consensus
	p2pNet    p2p.Net

	messageCh     chan p2p.Message
	locateWrongCh chan struct{}
	locateDoneCh  chan struct{}
	checkWrongCh  chan struct{}
	checkDoneCh   chan struct{}
	syncWrongCh   chan struct{}
	syncDoneCh    chan struct{}
}

func (sm *SyncManager) resetStatus() {
	sm.status = freeStatus
	sm.checkNum = 0
	sm.checkRootHash = &crypto.HashType{}
	sm.fetchHashes = make([]*crypto.HashType, 0)
	sm.checkHash = nil
	sm.hasSyncBlocks = 0
}

func (sm *SyncManager) resetAll() {
	sm.resetStatus()
	sm.stalePeers = make(map[peer.ID]peerStatus)
}

// NewSyncManager return new block sync manager.
func NewSyncManager(blockChain *chain.BlockChain, p2pNet p2p.Net,
	consensus consensus.Consensus, parent goprocess.Process) *SyncManager {
	return &SyncManager{
		chain:         blockChain,
		consensus:     consensus,
		p2pNet:        p2pNet,
		proc:          goprocess.WithParent(parent),
		stalePeers:    make(map[peer.ID]peerStatus),
		messageCh:     make(chan p2p.Message, 512),
		locateWrongCh: make(chan struct{}),
		locateDoneCh:  make(chan struct{}),
		checkWrongCh:  make(chan struct{}),
		checkDoneCh:   make(chan struct{}),
		syncWrongCh:   make(chan struct{}),
		syncDoneCh:    make(chan struct{}),
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
out_sync:
	for {
		// start block sync.
		sm.resetStatus()
		if err := sm.locateHashes(); err != nil {
			logger.Warn("locateHashes error: ", err)
			//select {
			//case <-sm.proc.Closing():
			//	logger.Info("Quit startSync loop.")
			//	return
			//}
			time.Sleep(100 * time.Millisecond)
			continue
		}

		logger.Info("wait locateHashes done")
	out_locate:
		for {
			select {
			case <-sm.locateDoneCh:
				logger.Info("success to locate, start check")
				break out_locate
			case <-sm.locateWrongCh:
				logger.Infof("SyncManager locate wrong, restart sync")
				continue out_sync
			case <-sm.proc.Closing():
				logger.Info("Quit handle sync msg loop.")
				return
			}
		}

		if err := sm.checkHashes(); err != nil {
			logger.Info("checkHashes error: ", err)
			continue out_sync
		}
		logger.Info("wait checkHashes done")
	out_check:
		for {
			select {
			case <-sm.checkDoneCh:
				logger.Infof("success to check, start to sync")
				break out_check
			case <-sm.checkWrongCh:
				logger.Infof("SyncManager check failed, checkHash: %+v, restart sync",
					sm.checkHash)
				continue out_sync
			case <-sm.proc.Closing():
				logger.Info("Quit handle sync msg loop.")
				return
			}
		}

		sm.fetchBlocks(sm.fetchHashes)
		logger.Infof("wait sync %d blocks done", len(sm.fetchHashes))
		for {
			select {
			case <-sm.syncDoneCh:
				logger.Infof("success to sync %d blocks", len(sm.fetchHashes))
				sm.resetAll()
				sm.consensus.RecoverMint()
				return
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
		logger.Infof("getLatestBlockLocator error: ", err)
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

func (sm *SyncManager) onLocateRequest(msg p2p.Message) error {
	// parse response
	lh := new(LocateHeaders)
	if err := lh.Unmarshal(msg.Body()); err != nil {
		return err
	}
	//
	hashes, err := sm.chain.LocateForkPointAndFetchHeaders(lh.Hashes)
	if err != nil {
		logger.Infof("onLocateRequest fetch headers error: %s", err)
		return err
	}
	// send SyncHeaders hashes to active end
	sh := newSyncHeaders(hashes...)
	logger.Infof("send message[0x%X] (%d hashes) to peer %s",
		p2p.LocateForkPointResponse, len(hashes), msg.From().Pretty())
	return sm.p2pNet.SendMessageToPeer(p2p.LocateForkPointResponse, sh, msg.From())
}

func (sm *SyncManager) onLocateResponse(msg p2p.Message) error {
	//if sm.isPeerStatusFor(locatePeerStatus, msg.From()) {
	if sm.isPeerStatusFor(availablePeerStatus, msg.From()) {
		return fmt.Errorf("receive LocateForkPointResponse from non-sync peer[%s]",
			msg.From().Pretty())
	}
	// parse response
	sh := new(SyncHeaders)
	if err := sh.Unmarshal(msg.Body()); err != nil {
		logger.Infof("onLocateResponse unmarshal msg.Body[%+v] error: %s", msg.Body(), err)
		sm.locateWrongCh <- struct{}{}
		return err
	}
	if len(sh.Hashes) == 0 {
		logger.Infof("onLocateResponse receive no Header from peer[%s], "+
			"try another peer to sync", msg.From().Pretty())
		sm.locateWrongCh <- struct{}{}
		return nil
	}
	// get headers hashes needed to sync
	hashes, err := sm.getSyncHeaders(sh.Hashes)
	if err != nil {
		logger.Infof("getSyncHeaders error: %s", err)
		return err
	}
	logger.Debugf("getSyncHeaders hashes[%d]", len(hashes))
	sm.fetchHashes = hashes
	merkleRoot := util.BuildMerkleRoot(hashes)
	sm.checkRootHash = merkleRoot[len(merkleRoot)-1]
	sm.locateDoneCh <- struct{}{}
	return nil
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
	length := len(sm.fetchHashes)
	sm.checkHash = newCheckHash(sm.fetchHashes[0], length)
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

func (sm *SyncManager) onCheckRequest(msg p2p.Message) error {
	//if sm.isPeerStatusFor(checkedPeerStatus, msg.From()) {
	if sm.isPeerStatusFor(availablePeerStatus, msg.From()) {
		sm.syncWrongCh <- struct{}{}
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
	//if sm.isPeerStatusFor(checkPeerStatus, msg.From()) {
	if sm.isPeerStatusFor(availablePeerStatus, msg.From()) {
		sm.checkWrongCh <- struct{}{}
		return fmt.Errorf("receive LocateCheckResponse from non-sync peer[%s]",
			msg.From().Pretty())
	}
	sm.checkNum++
	if sm.checkNum > maxCheckPeers {
		sm.checkWrongCh <- struct{}{}
		return fmt.Errorf("receive too many LocateCheckResponse from check peer[%s]",
			msg.From().Pretty())
	}
	// parse response
	sch := new(SyncCheckHash)
	if err := sch.Unmarshal(msg.Body()); err != nil {
		sm.checkWrongCh <- struct{}{}
		return err
	}
	// check rootHash
	if *sm.checkRootHash != *sch.RootHash {
		logger.Warnf("check RootHash mismatch from peer[%s], recv: %v, want: %v, msg body: %+v",
			msg.From().Pretty(), sch.RootHash, sm.checkRootHash, msg.Body())
		sm.checkWrongCh <- struct{}{}
		return nil
	}
	logger.Infof("success to check %d times", sm.checkNum)
	if sm.checkNum < maxCheckPeers {
		return nil
	}
	sm.checkDoneCh <- struct{}{}
	return nil
}

func (sm *SyncManager) onBlocksRequest(msg p2p.Message) error {
	// parse response
	fbh := newFetchBlockHeaders()
	if err := fbh.Unmarshal(msg.Body()); err != nil {
		return err
	}
	// fetch blocks from local main chain
	blocks, err := sm.fetchLocalBlocks(fbh.Hashes)
	if err != nil {
		return fmt.Errorf("fetchLocalBlocks error: %s", err)
	}
	//
	sb := newSyncBlocks(blocks...)
	return sm.p2pNet.SendMessageToPeer(p2p.BlockChunkResponse, sb, msg.From())
}

func (sm *SyncManager) onBlocksResponse(msg p2p.Message) error {
	//if sm.isPeerStatusFor(locatePeerStatus, msg.From()) {
	if sm.isPeerStatusFor(availablePeerStatus, msg.From()) {
		sm.syncWrongCh <- struct{}{}
		return fmt.Errorf("receive BlockChunkResponse from non-sync peer[%s]",
			msg.From().Pretty())
	}
	// parse response
	sb := new(SyncBlocks)
	if err := sb.Unmarshal(msg.Body()); err != nil {
		sm.syncWrongCh <- struct{}{}
		return err
	}
	// process blocks
	for _, b := range sb.Blocks {
		sm.chain.ProcessBlock(b, false)
	}
	sm.hasSyncBlocks += len(sb.Blocks)
	if sm.hasSyncBlocks == len(sm.fetchHashes) {
		sm.syncDoneCh <- struct{}{}
		logger.Infof("SyncManager has sync %d(done) blocks, current peer[%s]",
			len(sm.fetchHashes), msg.From().Pretty())
	} else {
		logger.Infof("SyncManager has sync %d/(%d) blocks, current peer[%s]",
			sm.hasSyncBlocks, len(sm.fetchHashes), msg.From().Pretty())
	}
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
		return nil, nil
	}
	heights := heightLocator(tailHeight)
	for _, h := range heights {
		b, err := sm.chain.LoadBlockByHeight(h)
		if err != nil {
			return nil, err
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
	for step := int32(1); h-step >= 0; step = step * 2 {
		h -= step + 1
		heights = append(heights, h)
	}
	if h > 0 {
		heights = append(heights, 0)
	}
	return heights
}

// getSyncHeaders remove overlapped headers between locateHashes and local chain
func (sm *SyncManager) getSyncHeaders(locateHashes []*crypto.HashType) (
	[]*crypto.HashType, error) {
	start := 0
	for i := len(locateHashes) - 1; i >= 0; i-- {
		_, err := sm.chain.LoadBlockByHash(*locateHashes[i])
		if err == core.ErrEmptyProtoMessage {
			return locateHashes[start:], nil
		} else if err != nil {
			return nil, err
		}
		start++
	}
	return nil, errors.New("no header needed to sync")
}

func (sm *SyncManager) fetchBlocks(hashes []*crypto.HashType) error {
	for i := 0; i < len(hashes); i += syncBlockChunkSize {
		pid, err := sm.pickOnePeer()
		if err != nil {
			logger.Infof("select peer to sync error: %s", err)
			// TODO: need to strict error handling method
			//if err == errNoPeerToSync {
			//	sm.syncWrongCh <- struct{}{}
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

func (sm *SyncManager) fetchLocalBlocks(hashes []*crypto.HashType) ([]*coreTypes.Block, error) {
	blocks := make([]*coreTypes.Block, 0, len(hashes))
	for _, h := range hashes {
		b, err := sm.chain.LoadBlockByHash(*h)
		if err != nil {
			return nil, err
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
