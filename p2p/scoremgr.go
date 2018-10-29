// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package p2p

import (
	"time"

	"github.com/BOXFoundation/boxd/boxd/eventbus"
	"github.com/BOXFoundation/boxd/p2p/pscore"
	"github.com/jbenet/goprocess"
	peer "github.com/libp2p/go-libp2p-peer"
)

var scoreMgr *ScoreManager

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
	proc   goprocess.Process
}

func init() {
	scoreMgr = newScoreManager()
}

func newScoreManager() *ScoreManager {
	scoreMgr := new(ScoreManager)
	scoreMgr.Scores = make(map[peer.ID]*pscore.DynamicPeerScore)
	return scoreMgr
}

// NewScoreManager returns new ScoreManager.
func NewScoreManager(parent goprocess.Process, bus eventbus.Bus, boxPeer *BoxPeer) *ScoreManager {
	scoreMgr.bus = bus
	scoreMgr.peer = boxPeer
	score := func(pid peer.ID, score pscore.ScoreEvent) {
		peerScore := scoreMgr.Scores[pid]
		switch score {
		case pscore.PunishConnTimeOut, pscore.PunishBadBlock, pscore.PunishBadTx, pscore.PunishSyncMsg:
			logger.Errorf("Punish peer %v because %v", pid.Pretty(), score)
			peerScore.Punish(score)
		case pscore.AwardNewBlock, pscore.AwardNewTx:
			logger.Errorf("Award peer %v because %v", pid.Pretty(), score)
			peerScore.Award(score)
		default:
			logger.Error("No such event found: %v", score)
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
				boxPeer.Gc()
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
			p.Punish(pid, pscore.PunishNoHeartBeat)
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
				p.Punish(pid, pscore.PunishConnUnsteadiness)
			}
		}
	}
}
