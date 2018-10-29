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
	Scores map[peer.ID]*pscore.DynamicPeerScore
	bus    eventbus.Bus
	peer   *BoxPeer
	Mutex  sync.Mutex
	proc   goprocess.Process
}

// NewScoreManager returns new ScoreManager.
func NewScoreManager(parent goprocess.Process, bus eventbus.Bus, boxPeer *BoxPeer) *ScoreManager {
	scoreMgr := new(ScoreManager)
	scoreMgr.Scores = make(map[peer.ID]*pscore.DynamicPeerScore)
	scoreMgr.bus = bus
	scoreMgr.peer = boxPeer
	score := func(pid peer.ID, event pscore.ScoreEvent) {
		peerScore := scoreMgr.Scores[pid]
		switch event {
		case pscore.PunishConnTimeOutEvent, pscore.PunishBadBlockEvent, pscore.PunishBadTxEvent, pscore.PunishSyncMsgEvent:
			logger.Errorf("Punish peer %v because %v", pid.Pretty(), event)
			peerScore.Punish(event)
		case pscore.AwardNewBlockEvent, pscore.AwardNewTxEvent:
			logger.Errorf("Award peer %v because %v", pid.Pretty(), event)
			peerScore.Award(event)
		default:
			logger.Error("No such event found: %v", event)
		}
	}
	scoreMgr.bus.Subscribe(eventbus.TopicChainScoreEvent, score)

	scoreMgr.proc = parent.Go(func(p goprocess.Process) {
		loopTicker := time.NewTicker(PeerDiscoverLoopInterval)
		defer loopTicker.Stop()
		for {
			select {
			case <-loopTicker.C:
				scoreMgr.checkConnLastUnix()
				scoreMgr.checkConnStability()
				scoreMgr.clearUp()
			case <-p.Closing():
				logger.Info("Quit route table loop.")
				return
			}
		}
	})
	return scoreMgr
}

// checkConnLastUnix check last ping/pong time
func (sm *ScoreManager) checkConnLastUnix() {
	p := sm.peer
	t := time.Now().UnixNano() / 1e6
	for pid, v := range p.conns {
		conn := v.(*Conn)
		if conn.Established() && conn.LastUnix() != 0 && t-conn.LastUnix() > pscore.HeartBeatLatencyTime {
			logger.Errorf("Punish peer %v because %v milliseconds no hb", pid.Pretty(), t-conn.LastUnix())
			p.Punish(pid, pscore.PunishNoHeartBeatEvent)
		}
	}
}

// checkConnStability check whether the peers' disconn times > DisconnMinTime in DisconnTimesPeriod milliseconds
func (sm *ScoreManager) checkConnStability() {
	p := sm.peer
	t := time.Now().UnixNano() / 1e6
	limit := t - pscore.DisconnTimesPeriod
	for pid, v := range p.conns {
		if v.(*Conn).Established() {
			records := sm.Scores[pid].ConnRecords()

			var cnt int
			for ts := records.Back(); ts != nil; ts = ts.Prev() {
				if ts.Value.(int64) > limit {
					cnt++
				} else {
					break
				}
			}
			if cnt > pscore.DisconnMinTime {
				logger.Errorf("Punish peer %v because %v times disconnection last %v seconds ", pid.Pretty(), pscore.DisconnTimesPeriod, pscore.DisconnMinTime)
				p.Punish(pid, pscore.PunishConnUnsteadinessEvent)
			}
		}
	}
}

// Gc close the lowest grade peers' conn on time when conn pool is almost full
func (sm *ScoreManager) clearUp() {
	logger.Errorf("clearUp invoked")
	var queue []peerConnScore
	for pid, v := range sm.peer.conns {
		conn := v.(*Conn)
		score := sm.peer.Score(pid)
		peerScore := peerConnScore{
			score: score,
			conn:  conn,
		}
		queue = append(queue, peerScore)
	}

	for _, v := range queue {
		logger.Errorf("score = %v", v.score)
	}
	// Sorting by scores desc
	sort.Slice(queue, func(i, j int) bool {
		return queue[i].score <= queue[j].score
	})

	if size := len(queue) - ConnMaxCapacity*ConnLoadFactor; size > 0 {
		for i := 0; i < size; i++ {
			queue[i].conn.Close()
		}
	}
}
