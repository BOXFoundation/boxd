// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package chain

import (
	"sync"
	"time"

	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/log"
	"github.com/BOXFoundation/boxd/p2p"
	lru "github.com/hashicorp/golang-lru"
	"github.com/jbenet/goprocess"
)

var logger = log.NewLogger("blacklist") // logger

// TODO: add into core config
const (
	blackListPeriod    = 3 * time.Second
	blackListThreshold = 100

	txEvidenceMaxSize    = 100
	blockEvidenceMaxSize = 5
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
	notifiee     p2p.Net
	proc         goprocess.Process
}

// Evidence can help bp to restore error scene
type Evidence struct {
	PubKeyChecksum uint32
	Scene          interface{}
	Err            error
	Ts             time.Time
}

func init() {
	blackList = &BlackList{
		Details: new(sync.Map),
		SceneCh: make(chan *Evidence, 4096),
	}
	blackList.evidenceNote, _ = lru.New(4096)
}

// Default returns the default BlackList.
func Default() *BlackList {
	return blackList
}

// Run process
func (bl *BlackList) Run(notifiee p2p.Net, parent goprocess.Process) {

	bl.notifiee = notifiee
	bl.proc = parent.Go(func(p goprocess.Process) {
		logger.Info("Start blacklist loop")
		for {
			select {
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
	switch _type := evidence.Scene.(type) {
	case *types.Transaction:
		evidenceCh = personalNote.([]chan *Evidence)[0]
		if len(evidenceCh) >= txEvidenceMaxSize {
			first = <-evidenceCh
		} else {
			evidenceCh <- evidence
			return
		}
	case *types.Block:
		evidenceCh = personalNote.([]chan *Evidence)[1]
		if len(evidenceCh) >= blockEvidenceMaxSize {
			first = <-evidenceCh
		} else {
			evidenceCh <- evidence
			return
		}
	default:
		logger.Errorf("invalid evidence type: %v", _type)
	}
	if first == nil || first.Ts.Add(blackListPeriod).Before(evidence.Ts) {
		return
	}
	eviPackege := bl.packageEvidences(first, evidence, evidenceCh)
	bl.notifiee.BroadcastToMiners(p2p.BlacklistRequest, eviPackege)
}

func (bl *BlackList) packageEvidences(first, last *Evidence, evidenceCh chan *Evidence) []*Evidence {
	evidences := []*Evidence{first}
	for len(evidenceCh) != 0 {
		evidences = append(evidences, <-evidenceCh)
	}
	evidences = append(evidences, last)
	return evidences
}
