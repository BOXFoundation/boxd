// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package p2p

import (
	"sort"
	"sync"
	"time"

	"github.com/BOXFoundation/boxd/boxd/eventbus"
	"github.com/BOXFoundation/boxd/p2p/pscore"
	"github.com/jbenet/goprocess"
	peer "github.com/libp2p/go-libp2p-peer"
)

// peerConnScore is used for peer.Gc to score the conn
type peerConnScore struct {
	score int64
	conn  *Conn
}

// ScoreManager is an object to maitian all scores of peers
type ScoreManager struct {
	scores *sync.Map
	bus    eventbus.Bus
	peer   *BoxPeer
	Mutex  sync.Mutex
	proc   goprocess.Process
}

// NewScoreManager returns new ScoreManager.
func NewScoreManager(parent goprocess.Process, bus eventbus.Bus, boxPeer *BoxPeer) *ScoreManager {
	scoreMgr := new(ScoreManager)
	scoreMgr.scores = new(sync.Map)
	scoreMgr.bus = bus
	scoreMgr.peer = boxPeer

	scoreMgr.bus.Subscribe(eventbus.TopicConnEvent, scoreMgr.record)
	scoreMgr.run(parent)

	return scoreMgr
}

func (sm *ScoreManager) run(parent goprocess.Process) {
	sm.proc = parent.Go(func(p goprocess.Process) {
		loopTicker := time.NewTicker(pscore.ConnCleanupLoopInterval)
		defer loopTicker.Stop()
		for {
			select {
			case <-loopTicker.C:
				sm.clearUp()
			case <-p.Closing():
				logger.Info("Quit score manager loop.")
				return
			}
		}
	})
}

func (sm *ScoreManager) record(pid peer.ID, event eventbus.BusEvent) {
	peerScore, _ := sm.scores.Load(pid)
	if peerScore == nil {
		peerScore = pscore.NewDynamicPeerScore(pid)
		sm.scores.Store(pid, peerScore)
	}
	peerScore.(*pscore.DynamicPeerScore).Record(event)
}

// clearUp close the lowest grade peers' conn on time when conn pool is almost full
func (sm *ScoreManager) clearUp() {
	logger.Errorf("cleanup invoked")
	var queue []peerConnScore
	t := time.Now()
	sm.peer.conns.Range(func(k, v interface{}) bool {
		pid := k.(peer.ID)
		conn := v.(*Conn)
		peerScore, _ := sm.scores.Load(pid)
		// to prevent nil when no msg receive but ticker arrive
		if peerScore == nil {
			peerScore = pscore.NewDynamicPeerScore(pid)
			sm.scores.Store(pid, peerScore)
		}

		connScore := peerConnScore{
			score: peerScore.(*pscore.DynamicPeerScore).Score(t),
			conn:  conn,
		}
		queue = append(queue, connScore)
		return true
	})

	// Sorting by scores desc
	sort.Slice(queue, func(i, j int) bool {
		return queue[i].score <= queue[j].score
	})

	for _, v := range queue {
		logger.Errorf("%v %v", v.score, v.conn.remotePeer.Pretty())
	}

	if size := len(queue) - int(float32(sm.peer.config.ConnMaxCapacity)*sm.peer.config.ConnLoadFactor); size > 0 {
		for i := 0; i < size; i++ {
			logger.Infof("Close conn %v because of low score %v", queue[i].conn.remotePeer.Pretty(), queue[i].score)
			queue[i].conn.Close()
		}
	}
}
