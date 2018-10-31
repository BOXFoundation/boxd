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

	scoreMgr.bus.Subscribe(eventbus.TopicConnEvent, scoreMgr.scoreForEvent)
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
				sm.checkConnLastUnix()
				sm.checkConnStability()
				sm.clearUp()
			case <-p.Closing():
				logger.Info("Quit score manager loop.")
				return
			}
		}
	})
}

func (sm *ScoreManager) scoreForEvent(pid peer.ID, event pscore.ScoreEvent) {
	peerScore, _ := sm.scores.Load(pid)
	if peerScore == nil {
		peerScore = pscore.NewDynamicPeerScore(pid)
		sm.scores.Store(pid, peerScore)
	}

	switch event {
	case pscore.ConnTimeOutEvent, pscore.BadBlockEvent, pscore.BadTxEvent, pscore.SyncMsgEvent, pscore.NoHeartBeatEvent, pscore.ConnUnsteadinessEvent:
		logger.Debugf("Punish peer %v because %v", pid.Pretty(), event)
		peerScore.(*pscore.DynamicPeerScore).Punish(event)
	case pscore.NewBlockEvent, pscore.NewTxEvent:
		logger.Debugf("Reward peer %v because %v", pid.Pretty(), event)
		peerScore.(*pscore.DynamicPeerScore).Reward(event)
	case pscore.PeerConnEvent:
	case pscore.PeerDisconnEvent:
		peerScore.(*pscore.DynamicPeerScore).Disconnect()
	default:
		logger.Debugf("No such event found: %v", event)
	}
}

// checkConnLastUnix check last ping/pong time
func (sm *ScoreManager) checkConnLastUnix() {
	p := sm.peer
	t := time.Now().UnixNano() / 1e6
	p.conns.Range(func(k, v interface{}) bool {
		conn := v.(*Conn)
		if conn.Established() && conn.LastUnix() != 0 && t-conn.LastUnix() > pscore.HeartBeatLatencyTime {
			logger.Errorf("Punish peer %v because %v milliseconds no hb", k.(peer.ID).Pretty(), t-conn.LastUnix())
			p.bus.Publish(eventbus.TopicConnEvent, k.(peer.ID), pscore.NoHeartBeatEvent)
		}
		return true
	})
}

// checkConnStability check whether the peers' disconn times > DisconnMinTime in DisconnTimesPeriod milliseconds
func (sm *ScoreManager) checkConnStability() {
	p := sm.peer
	t := time.Now().UnixNano() / 1e6
	limit := t - pscore.DisconnTimesPeriod
	p.conns.Range(func(k, v interface{}) bool {
		pid := k.(peer.ID)
		if v.(*Conn).Established() {
			peerScore, _ := sm.scores.Load(pid)
			records := peerScore.(*pscore.DynamicPeerScore).ConnRecords()

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
				p.bus.Publish(eventbus.TopicConnEvent, p.id, pscore.ConnUnsteadinessEvent)
			}
		}
		return true
	})
}

// Gc close the lowest grade peers' conn on time when conn pool is almost full
func (sm *ScoreManager) clearUp() {
	logger.Errorf("cleanup invoked")
	var queue []peerConnScore
	sm.peer.conns.Range(func(k, v interface{}) bool {
		pid := k.(peer.ID)
		conn := v.(*Conn)
		score := sm.peer.Score(pid)
		peerScore := peerConnScore{
			score: score,
			conn:  conn,
		}
		queue = append(queue, peerScore)
		return true
	})

	// Sorting by scores desc
	sort.Slice(queue, func(i, j int) bool {
		return queue[i].score <= queue[j].score
	})

	for _,v := range queue {
		logger.Errorf("aaaaa %v %v", v.conn.remotePeer.Pretty(), v.score)
	}

	if size := len(queue) - int(float32(sm.peer.config.ConnMaxCapacity)*sm.peer.config.ConnLoadFactor); size > 0 {
		for i := 0; i < size; i++ {
			logger.Infof("Close conn %v because of low score %v", queue[i].conn.remotePeer.Pretty(), queue[i].score)
			queue[i].conn.Close()
		}
	}
}
