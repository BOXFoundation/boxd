// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package netsync

import (
	"errors"
	"fmt"
	"math/rand"
	"time"

	"github.com/BOXFoundation/boxd/core/chain"
	"github.com/BOXFoundation/boxd/crypto"
	"github.com/BOXFoundation/boxd/log"
	"github.com/BOXFoundation/boxd/p2p"
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
	status        syncStatus
	checkNum      int
	checkRootHash *crypto.HashType
	fetchHashes   []*crypto.HashType
	checkHash     *CheckHash
	stalePeers    map[peer.ID]peerStatus

	proc     goprocess.Process
	chain    *chain.BlockChain
	notifiee p2p.Net

	messageCh             chan p2p.Message
	locateForkPointDoneCh chan bool
	syncBlockChunkDoneCh  chan bool
	//alertCh               chan error
}

func (sm *SyncManager) resetStatus() {
	sm.status = freeStatus
	sm.checkNum = 0
	sm.checkRootHash = zeroHash
	sm.fetchHashes = make([]*crypto.HashType, 0)
	sm.checkHash = nil
}

func (sm *SyncManager) resetAll() {
	sm.resetStatus()
	sm.stalePeers = make(map[peer.ID]peerStatus)
}

// NewSyncManager return new block sync manager.
func NewSyncManager(blockChain *chain.BlockChain, notifiee p2p.Net,
	parent goprocess.Process) *SyncManager {
	return &SyncManager{
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
	sm.notifiee.Subscribe(p2p.NewNotifiee(p2p.BlockChunkRequest, sm.messageCh))
	sm.notifiee.Subscribe(p2p.NewNotifiee(p2p.BlockChunkResponse, sm.messageCh))
}

func (sm *SyncManager) handleSyncMessage() {
	for {
		select {

		case message := <-sm.messageCh:
			switch message.Code() {
			case p2p.LocateForkPointRequest:
				sm.onLocateForkPointRequest(message)
			case p2p.LocateForkPointResponse:
				sm.onLocateForkPointResponse(message)
			case p2p.LocateCheckRequest:
				sm.onLocateCheckRequest(message)
			case p2p.LocateCheckResponse:
				sm.onLocateCheckResponse(message)
			case p2p.BlockChunkRequest:
				sm.onBlockChunkRequest(message)
			case p2p.BlockChunkResponse:
				sm.onBlockChunkResponse(message)
			default:
				logger.Warn("Failed to handle sync message, unknow message code")
			}
		case <-sm.proc.Closing():
			logger.Info("Quit handle sync message loop.")
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
	hashes := sm.getLatestBlockLocator()
	lh := newLocateHeaders(hashes...)
	// select one peer to sync
	pid, err := sm.selectPeer()
	if err == errNoPeerToSync {
		return
	}
	sm.stalePeers[pid] = locatePeerStatus
	sm.notifiee.SendMessageToPeer(p2p.LocateForkPointRequest, lh, pid)
}

func (sm *SyncManager) onLocateForkPointRequest(message p2p.Message) {

}

func (sm *SyncManager) onLocateForkPointResponse(message p2p.Message) error {
	if sm.isLocatePeer(message.From()) {
		return fmt.Errorf("receive LocateForkPointResponse from non-sync peer[%s]",
			message.From())
	}
	// parse response
	sh := new(SyncHeaders)
	if err := sh.Unmarshal(message.Body()); err != nil {
		return err
	}
	if len(sh.Hashes) == 0 {
		sm.locateForkPointRequest()
		logger.Infof("onLocateForkPointResponse receive no Header from peer[%s], "+
			"try another peer to sync", message.From())
		return nil
	}
	// get headers hashes needed to sync
	hashes := sm.getSyncHeaders(sh.Hashes)
	if len(hashes) == 0 {
		return nil
	}
	sm.fetchHashes = hashes
	sm.checkRootHash = sm.calcRootHash(hashes)
	// start CheckHash process
	// select peers to check
	peers := make([]peer.ID, 0, maxCheckPeers)
	for i := 0; i < maxCheckPeers; i++ {
		pid, err := sm.selectPeer()
		if err != nil {
			return err
		}
		peers = append(peers, pid)
	}
	// send message to checking peer
	sm.checkHash = newCheckHash(hashes[0], len(hashes))
	for _, pid := range peers {
		sm.stalePeers[pid] = checkedPeerStatus
		sm.notifiee.SendMessageToPeer(p2p.LocateCheckRequest, sm.checkHash, pid)
	}
	return nil
}

func (sm *SyncManager) onLocateCheckRequest(message p2p.Message) {

}

func (sm *SyncManager) onLocateCheckResponse(message p2p.Message) error {
	if sm.isCheckPeer(message.From()) {
		return fmt.Errorf("receive LocateCheckResponse from non-sync peer[%s]",
			message.From())
	}
	sm.checkNum++
	if sm.checkNum > maxCheckPeers {
		return fmt.Errorf("receive too many LocateCheckResponse from sync peer[%s]",
			message.From())
	}
	// parse response
	sch := new(SyncCheckHash)
	if err := sch.Unmarshal(message.Body()); err != nil {
		return err
	}
	// check rootHash
	if *sm.checkRootHash != *sch.RootHash {
		logger.Infof("onLocateCheckResponse failed from peer[%s]", message.From())
		sm.resetStatus()
		sm.locateForkPointRequest()
		return nil
	}
	if sm.checkNum < maxCheckPeers {
		pid, err := sm.selectPeer()
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

func (sm *SyncManager) onBlockChunkRequest(message p2p.Message) {

}

func (sm *SyncManager) onBlockChunkResponse(message p2p.Message) {

}

func (sm *SyncManager) isCheckPeer(id peer.ID) bool {
	s, ok := sm.stalePeers[id]
	return ok && s == checkedPeerStatus
}

func (sm *SyncManager) isLocatePeer(id peer.ID) bool {
	s, ok := sm.stalePeers[id]
	return ok && s == locatePeerStatus
}

// TODO: need to implement a method to improve sync effience and not to sync
// melicious peers' blocks
func (sm *SyncManager) selectPeer() (peer.ID, error) {
	var peers []peer.ID
	var id peer.ID
	//peers = sm.notifiee.GetPeers()
	if len(peers) == 0 {
		return peer.ID(""), errNoPeerToSync
	}
	rand.Seed(time.Now().UnixNano())
	id = peers[rand.Intn(len(peers))]
	return id, nil
}

// getLatestBlockLocator returns a block locator that comprises hashes
// sequences
func (sm *SyncManager) getLatestBlockLocator() []*crypto.HashType {

	return nil
}

// getSyncHeaders remove overlapped headers between locateHashes and local chain
func (sm *SyncManager) getSyncHeaders(locateHashes []*crypto.HashType) []*crypto.HashType {

	return nil
}

func (sm *SyncManager) calcRootHash(hashes []*crypto.HashType) *crypto.HashType {
	return nil
}

func (sm *SyncManager) fetchBlocks(hashes []*crypto.HashType) error {
	for i := 0; i < len(hashes); i += syncBlockChunkSize {
		pid, err := sm.selectPeer()
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
