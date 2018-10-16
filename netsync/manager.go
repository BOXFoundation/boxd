// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package netsync

import (
	"github.com/BOXFoundation/boxd/core/chain"
	"github.com/BOXFoundation/boxd/log"
	"github.com/BOXFoundation/boxd/p2p"
	"github.com/jbenet/goprocess"
)

var logger = log.NewLogger("sync") // logger

// SyncManager use to manage blocks and main chains to stay in sync
type SyncManager struct {
	chain                 *chain.BlockChain
	notifiee              p2p.Net
	messageCh             chan p2p.Message
	proc                  goprocess.Process
	locateForkPointDoneCh chan bool
	syncBlockChunkDoneCh  chan bool
}

// NewSyncManager return new block sync manager.
func NewSyncManager(blockChain *chain.BlockChain, notifiee p2p.Net, parent goprocess.Process) *SyncManager {
	return &SyncManager{
		chain:     blockChain,
		notifiee:  notifiee,
		messageCh: make(chan p2p.Message, 512),
		proc:      goprocess.WithParent(parent),
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

}

func (sm *SyncManager) onLocateForkPointRequest(message p2p.Message) {

}

func (sm *SyncManager) onLocateForkPointResponse(message p2p.Message) {

}

func (sm *SyncManager) onBlockChunkRequest(message p2p.Message) {

}

func (sm *SyncManager) onBlockChunkResponse(message p2p.Message) {

}
