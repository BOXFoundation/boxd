// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package blacklist

import (
	"sync"
	"time"

	"github.com/BOXFoundation/boxd/boxd/eventbus"
	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/log"
	"github.com/BOXFoundation/boxd/p2p"
	lru "github.com/hashicorp/golang-lru"
	"github.com/jbenet/goprocess"
)

var logger = log.NewLogger("blacklist") // logger

// TODO: add into core config
const (
	TxEvidence uint32 = iota
	BlockEvidence

	blackListPeriod    = 3 * time.Second
	blackListThreshold = 100

	txEvidenceMaxSize    = 100
	blockEvidenceMaxSize = 3
	BlMsgChBufferSize    = 5
)

var (
	blackList *BlackList
)

// BlackList represents the black list of public keys
type BlackList struct {
	// checksumIEEE(pubKey) -> struct{}{}
	Details *sync.Map
	// checksumIEEE(pubKey) -> []ch
	evidenceNote *lru.Cache
	SceneCh      chan *Evidence

	bus      eventbus.Bus
	notifiee p2p.Net
	msgCh    chan p2p.Message
	proc     goprocess.Process
}

// Evidence can help bp to restore error scene
type Evidence struct {
	PubKeyChecksum uint32
	Tx             *types.Transaction
	Block          *types.Block
	Type           uint32
	Err            string
	Ts             int64
}

func init() {
	blackList = &BlackList{
		Details: new(sync.Map),
		SceneCh: make(chan *Evidence, 4096),
		msgCh:   make(chan p2p.Message, BlMsgChBufferSize),
	}
	blackList.evidenceNote, _ = lru.New(4096)
}

// Default returns the default BlackList.
func Default() *BlackList {
	return blackList
}

// Run process
func (bl *BlackList) Run(notifiee p2p.Net, bus eventbus.Bus, parent goprocess.Process) {

	bl.bus = bus
	bl.notifiee = notifiee
	bl.subscribeMessageNotifiee()

	bl.proc = parent.Go(func(p goprocess.Process) {
		logger.Info("Start blacklist loop")
		for {
			select {
			case msg := <-bl.msgCh:
				bl.onBlacklistMsg(msg)
			case evidence := <-bl.SceneCh:
				bl.processEvidence(evidence)
			case <-parent.Closing():
				logger.Info("Stopped black list loop.")
				return
			}
		}
	})
}

func (bl *BlackList) processEvidence(evidence *Evidence) {

	// get personal note
	personalNote, ok := bl.evidenceNote.Get(evidence.PubKeyChecksum)
	if !ok {
		newNote := make([]chan *Evidence, 2)
		// store invalid txs
		newNote[0] = make(chan *Evidence, txEvidenceMaxSize+1)
		// store invalid blocks
		newNote[1] = make(chan *Evidence, blockEvidenceMaxSize+1)
		bl.evidenceNote.Add(evidence.PubKeyChecksum, newNote)
		personalNote, _ = bl.evidenceNote.Get(evidence.PubKeyChecksum)
	}

	// get pioneer
	var first *Evidence
	var evidenceCh chan *Evidence
	switch evidence.Type {
	case TxEvidence:
		evidenceCh = personalNote.([]chan *Evidence)[0]
		if len(evidenceCh) >= txEvidenceMaxSize {
			first = <-evidenceCh
		} else {
			evidenceCh <- evidence
			return
		}
	case BlockEvidence:
		evidenceCh = personalNote.([]chan *Evidence)[1]
		if len(evidenceCh) >= blockEvidenceMaxSize {
			first = <-evidenceCh
		} else {
			evidenceCh <- evidence
			return
		}
	default:
		logger.Errorf("invalid evidence type: %v", evidence.Type)
	}
	if first == nil || time.Unix(int64(first.Ts), 0).Add(blackListPeriod).Before(time.Unix(int64(evidence.Ts), 0)) {
		return
	}
	eviPackege := bl.packageEvidences(first, evidence, evidenceCh)
	logger.Errorf("blacklist Broadcast to miners")
	bl.notifiee.BroadcastToMiners(p2p.BlacklistMsg, &BlacklistMsg{evidences: eviPackege})
}

func (bl *BlackList) packageEvidences(first, last *Evidence, evidenceCh chan *Evidence) []*Evidence {
	evidences := []*Evidence{first}
	for len(evidenceCh) != 0 {
		evidences = append(evidences, <-evidenceCh)
	}
	evidences = append(evidences, last)
	return evidences
}

func (bl *BlackList) subscribeMessageNotifiee() {
	bl.notifiee.Subscribe(p2p.NewNotifiee(p2p.BlacklistMsg, bl.msgCh))
}
