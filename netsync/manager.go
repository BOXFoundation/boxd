// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package netsync

import (
	"errors"
	"fmt"

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

	locatePeerStatus peerStatus = iota
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
	stalePeers map[peer.ID]peerStatus

	proc     goprocess.Process
	chain    *chain.BlockChain
	notifiee p2p.Net
	peer     *p2p.BoxPeer

	messageCh             chan p2p.Message
	locateForkPointDoneCh chan bool
	syncBlockChunkDoneCh  chan bool
	//alertCh               chan error
}

func (sm *SyncManager) resetStatus() {
	sm.status = freeStatus
	sm.checkNum = 0
	sm.checkRootHash = &crypto.HashType{}
	sm.fetchHashes = make([]*crypto.HashType, 0)
	sm.checkHash = nil
}

func (sm *SyncManager) resetAll() {
	sm.resetStatus()
	sm.stalePeers = make(map[peer.ID]peerStatus)
}

// NewSyncManager return new block sync manager.
func NewSyncManager(blockChain *chain.BlockChain, notifiee p2p.Net,
	boxPeer *p2p.BoxPeer, parent goprocess.Process) *SyncManager {
	return &SyncManager{
		peer:       boxPeer,
		chain:      blockChain,
		notifiee:   notifiee,
		messageCh:  make(chan p2p.Message, 512),
		proc:       goprocess.WithParent(parent),
		stalePeers: make(map[peer.ID]peerStatus),
	}
}

// Run start sync task and handle sync message
func (sm *SyncManager) Run() {
	sm.subscribeMessageNotifiee()
	go sm.handleSyncMessage()
}

// StartSync start sync block message from remote peers.
func (sm *SyncManager) StartSync() {
	go sm.startSync()
}

func (sm *SyncManager) subscribeMessageNotifiee() {
	sm.notifiee.Subscribe(p2p.NewNotifiee(p2p.LocateForkPointRequest, sm.messageCh))
	sm.notifiee.Subscribe(p2p.NewNotifiee(p2p.LocateForkPointResponse, sm.messageCh))
	sm.notifiee.Subscribe(p2p.NewNotifiee(p2p.LocateCheckRequest, sm.messageCh))
	sm.notifiee.Subscribe(p2p.NewNotifiee(p2p.LocateCheckResponse, sm.messageCh))
	sm.notifiee.Subscribe(p2p.NewNotifiee(p2p.BlockChunkRequest, sm.messageCh))
	sm.notifiee.Subscribe(p2p.NewNotifiee(p2p.BlockChunkResponse, sm.messageCh))
}

func (sm *SyncManager) handleSyncMessage() {
	for {
		select {
		case msg := <-sm.messageCh:
			switch msg.Code() {
			case p2p.LocateForkPointRequest:
				sm.onLocateForkPointRequest(msg)
			case p2p.LocateForkPointResponse:
				sm.onLocateForkPointResponse(msg)
			case p2p.LocateCheckRequest:
				sm.onLocateCheckRequest(msg)
			case p2p.LocateCheckResponse:
				sm.onLocateCheckResponse(msg)
			case p2p.BlockChunkRequest:
				sm.onBlockChunkRequest(msg)
			case p2p.BlockChunkResponse:
				sm.onBlockChunkResponse(msg)
			default:
				logger.Warn("Failed to handle sync msg, unknow msg code")
			}
		case <-sm.proc.Closing():
			logger.Info("Quit handle sync msg loop.")
			return
		}
	}
}

func (sm *SyncManager) startSync() {
	for {
		// start block sync.
		sm.locateForkPointRequest()

	Sync_Locate_Fork_Point:
		for {
			select {
			case <-sm.locateForkPointDoneCh:
				break Sync_Locate_Fork_Point
			}
		}

	Sync_Block_Chunk:
		for {
			select {
			case <-sm.syncBlockChunkDoneCh:
				break Sync_Block_Chunk
			}
		}

	}

}

func (sm *SyncManager) locateForkPointRequest() {
	// get block locator hashes
	hashes, err := sm.getLatestBlockLocator()
	if err != nil {
		logger.Warn("locateForkPointRequest error: ", err)
		return
	}
	lh := newLocateHeaders(hashes...)
	// select one peer to sync
	pid, err := sm.pickOnePeer()
	if err == errNoPeerToSync {
		logger.Warn("locateForkPointRequest error: ", err)
		return
	}
	sm.stalePeers[pid] = locatePeerStatus
	sm.notifiee.SendMessageToPeer(p2p.LocateForkPointRequest, lh, pid)
}

func (sm *SyncManager) onLocateForkPointRequest(msg p2p.Message) error {
	// parse response
	lh := new(LocateHeaders)
	if err := lh.Unmarshal(msg.Body()); err != nil {
		return err
	}
	//
	hashes, err := sm.chain.LocateForkPointAndFetchHeaders(lh.Hashes)
	if err != nil {
		logger.Infof("onLocateForkPointRequest fetch headers error: %s", err)
	}
	// send SyncHeaders hashes to active end
	sh := newSyncHeaders(hashes...)
	sm.notifiee.SendMessageToPeer(p2p.LocateForkPointRequest, sh, msg.From())
	return err
}

func (sm *SyncManager) onLocateForkPointResponse(msg p2p.Message) error {
	if sm.isLocatePeer(msg.From()) {
		return fmt.Errorf("receive LocateForkPointResponse from non-sync peer[%s]",
			msg.From())
	}
	// parse response
	sh := new(SyncHeaders)
	if err := sh.Unmarshal(msg.Body()); err != nil {
		return err
	}
	if len(sh.Hashes) == 0 {
		sm.resetStatus()
		sm.locateForkPointRequest()
		logger.Infof("onLocateForkPointResponse receive no Header from peer[%s], "+
			"try another peer to sync", msg.From())
		return nil
	}
	// get headers hashes needed to sync
	hashes := sm.getSyncHeaders(sh.Hashes)
	sm.fetchHashes = hashes
	merkleRoot := util.BuildMerkleRoot(hashes)
	sm.checkRootHash = merkleRoot[len(merkleRoot)-1]
	// start CheckHash process
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
	sm.checkHash = newCheckHash(hashes[0], len(hashes))
	for _, pid := range peers {
		sm.stalePeers[pid] = checkedPeerStatus
		go sm.notifiee.SendMessageToPeer(p2p.LocateCheckRequest, sm.checkHash, pid)
	}
	return nil
}

func (sm *SyncManager) onLocateCheckRequest(msg p2p.Message) error {
	// parse response
	ch := new(CheckHash)
	if err := ch.Unmarshal(msg.Body()); err != nil {
		return err
	}
	//
	hash, err := sm.chain.CalcRootHashForNBlocks(*ch.BeginHash, ch.Length)
	if err != nil {
		logger.Infof("onLocateCheckRequest calc root hash for %+v error: %s",
			ch, err)
	}
	// if hashes == nil {}
	sh := newSyncCheckHash(hash)
	sm.notifiee.SendMessageToPeer(p2p.LocateForkPointRequest, sh, msg.From())
	return err
}

func (sm *SyncManager) onLocateCheckResponse(msg p2p.Message) error {
	if sm.isCheckPeer(msg.From()) {
		return fmt.Errorf("receive LocateCheckResponse from non-sync peer[%s]",
			msg.From())
	}
	sm.checkNum++
	if sm.checkNum > maxCheckPeers {
		return fmt.Errorf("receive too many LocateCheckResponse from sync peer[%s]",
			msg.From())
	}
	// parse response
	sch := new(SyncCheckHash)
	if err := sch.Unmarshal(msg.Body()); err != nil {
		return err
	}
	// check rootHash
	if *sm.checkRootHash != *sch.RootHash {
		logger.Infof("onLocateCheckResponse failed from peer[%s]", msg.From())
		sm.resetStatus()
		sm.locateForkPointRequest()
		return nil
	}
	if sm.checkNum < maxCheckPeers {
		pid, err := sm.pickOnePeer()
		if err != nil {
			logger.Infof("onLocateCheckResponse select peer to resync error: %s", err)
			sm.resetStatus()
			sm.locateForkPointRequest()
			return nil
		}
		sm.stalePeers[pid] = checkedPeerStatus
		sm.notifiee.SendMessageToPeer(p2p.LocateCheckRequest, sm.checkHash, pid)
		return nil
	}
	return sm.fetchBlocks(sm.fetchHashes)
}

func (sm *SyncManager) onBlockChunkRequest(msg p2p.Message) error {
	// parse response
	fbh := newFetchBlockHeaders()
	if err := fbh.Unmarshal(msg.Body()); err != nil {
		return err
	}
	// fetch blocks from local main chain
	blocks, err := sm.fetchLocalBlocks(fbh.Hashes)
	if err != nil {
		logger.Infof("onBlockChunkRequest fetchLocalBloc for peer[%s] error: %s",
			msg.From(), err)
	}
	//
	sb := newSyncBlocks(blocks...)
	sm.notifiee.SendMessageToPeer(p2p.BlockChunkResponse, sb, msg.From())
	return nil
}

func (sm *SyncManager) onBlockChunkResponse(msg p2p.Message) error {
	if sm.isSyncPeer(msg.From()) {
		return fmt.Errorf("receive BlockChunkResponse from non-sync peer[%s]",
			msg.From())
	}
	// parse response
	sb := new(SyncBlocks)
	if err := sb.Unmarshal(msg.Body()); err != nil {
		return err
	}
	// process blocks
	for _, b := range sb.Blocks {
		sm.chain.ProcessBlock(b, false)
	}
	return nil
}

func (sm *SyncManager) isCheckPeer(id peer.ID) bool {
	s, ok := sm.stalePeers[id]
	return ok && s == checkedPeerStatus
}

func (sm *SyncManager) isLocatePeer(id peer.ID) bool {
	s, ok := sm.stalePeers[id]
	return ok && s == locatePeerStatus
}

func (sm *SyncManager) isSyncPeer(id peer.ID) bool {
	s, ok := sm.stalePeers[id]
	return ok && s == syncPeerStatus
}

// getLatestBlockLocator returns a block locator that comprises @beginSeqHashes
// and log2(height-@beginSeqHashes) entries to geneses for the skip portion.
func (sm *SyncManager) getLatestBlockLocator() ([]*crypto.HashType, error) {
	hashes := make([]*crypto.HashType, 0)
	tailHeight := sm.chain.TailBlock().Height
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
func (sm *SyncManager) getSyncHeaders(locateHashes []*crypto.HashType) []*crypto.HashType {
	start := 0
	for _, h := range locateHashes {
		b, err := sm.chain.LoadBlockByHash(*h)
		if err != nil {
			logger.Infof("SyncManager LoadBlockByHeight error: %s", err)
			return nil
		}
		if h == b.Hash {
			start++
		} else {
			break
		}
	}
	return locateHashes[start:]
}

func (sm *SyncManager) fetchBlocks(hashes []*crypto.HashType) error {
	for i := 0; i < len(hashes); i += syncBlockChunkSize {
		pid, err := sm.pickOnePeer()
		if err != nil {
			logger.Infof("onLocateCheckResponse select peer to resync error: %s", err)
			// TODO: need to strict error handling method
			if err == errNoPeerToSync {
				sm.resetStatus()
				sm.locateForkPointRequest()
				break
			}
			i -= syncBlockChunkSize
			continue
		}
		var fbh *FetchBlocksHeaders
		if i+syncBlockChunkSize <= len(hashes) {
			fbh = newFetchBlockHeaders(hashes[i : i+syncBlockChunkSize]...)
		} else {
			fbh = newFetchBlockHeaders(hashes[i:]...)
		}
		sm.notifiee.SendMessageToPeer(p2p.BlockChunkRequest, fbh, pid)
	}
	return nil
}

func (sm *SyncManager) fetchLocalBlocks(hashes []*crypto.HashType) ([]*coreTypes.Block, error) {
	blocks := make([]*coreTypes.Block, 0, len(hashes))
	for _, h := range hashes {
		b, err := sm.chain.LoadBlockByHash(*h)
		if err != nil {
			logger.Infof("SyncManager fetchLocalBlocks error: %s", err)
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
	pid := sm.peer.PickOnePeer(ids...)
	if pid == peer.ID("") {
		return pid, errNoPeerToSync
	}
	return pid, nil
}
