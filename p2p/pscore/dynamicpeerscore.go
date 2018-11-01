// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package pscore

import (
	"container/list"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/BOXFoundation/boxd/boxd/eventbus"
	"github.com/BOXFoundation/boxd/log"
	peer "github.com/libp2p/go-libp2p-peer"
)

var logger = log.NewLogger("pscore")

const (
	// baseScore indicates the default score of the peer.
	baseScore = 100

	// punishLimit indicates the upper limit of publishment.
	punishLimit = 1000

	// awardLimit indicates the upper limit of achievement.
	rewardLimit = 900

	// ConnCleanupLoopInterval indicates the loop interval for conn cleaning up
	ConnCleanupLoopInterval = 30 * time.Second
)

const (
	punishConnTimeOutScore     = 40
	punishConnTimeOutThreshold = 0

	punishBadBlockScore     = 100
	punishBadBlockThreshold = 0

	punishBadTxScore     = 30
	punishBadTxThreshold = 0

	punishSyncMsgScore     = 20
	punishSyncMsgThreshold = 0

	punishNoHeartBeatScore = 60
	punishHeartBeatCeiling = 5

	punishConnUnsteadinessScore = 100
	punishDisconnThreshold      = 3

	rewardNewBlockScore     = 80
	rewardNewBlockThreshold = 0

	rewardNewTxScore     = 10
	rewardNewTxThreshold = 0
)

var (
	// PunishFactors contains factors of punishment.
	PunishFactors = newFactors(60, 1800, 64)
	// RechieveFactors contains factors of achievement.
	RechieveFactors = newFactors(600, 18000, 512)
)

type factors struct {

	// halflife defines the time (in seconds) by which the publishment/achievement part
	// of the ban score decays to one half of it's original value.
	halflife int

	// lambda is the decaying constant.
	lambda float64

	// lifetime defines the maximum age of the publishment/achievement part of the ban
	// score to be considered a non-zero score (in seconds).
	lifetime int

	// precomputedLen defines the amount of decay factors (one per second) that
	// should be precomputed at initialization.
	precomputedLen int

	// precomputedFactor stores precomputed exponential decay factors for the first
	// 'precomputedLen' seconds starting from t == 0.
	precomputedFactor []float64
}

func newFactors(halflife, lifetime, precomputedLen int) *factors {
	factors := new(factors)
	factors.halflife = halflife
	factors.lambda = math.Ln2 / float64(halflife)
	factors.lifetime = lifetime
	factors.precomputedLen = precomputedLen
	factors.precomputedFactor = make([]float64, precomputedLen)
	for i := range factors.precomputedFactor {
		factors.precomputedFactor[i] = math.Exp(-1.0 * float64(i) * factors.lambda)
	}
	return factors
}

// decayRate returns the decay rate at t milliseconds, using precalculated values
// if available, or calculating the rate if needed.
func (factors *factors) decayRate(t int64) float64 {
	t = int64(t / 1000)
	if t < int64(factors.precomputedLen) {
		return factors.precomputedFactor[t]
	}
	return math.Exp(-1.0 * float64(t) * factors.lambda)
}

// DynamicPeerScore provides dynamic ban scores consisting of a punishment, a achievement
// and a decaying component. The punished score and the achieved score could be utilized
// to create simple additive banning policies similar to those found in other node implementations.
//
// The decaying score enables the creation of evasive logic which handles
// misbehaving peers (especially application layer DoS attacks) gracefully
// by disconnecting and banning peers attempting various kinds of flooding.
// DynamicPeerScore allows these two approaches to be used in tandem.
//
// baseScore: Values of type DynamicPeerScore are immediately ready for use upon
// declaration.
type DynamicPeerScore struct {
	pid         peer.ID
	lastUnix    int64
	punishment  float64
	achievement float64

	timeOutCounter  int
	badBlockCounter int
	badTxCounter    int
	syncCounter     int
	hbCounter       int
	disconnCounter  int
	newBlockCounter int
	newTxCounter    int

	mtx sync.Mutex
}

// NewDynamicPeerScore returns new DynamicPeerScore.
func NewDynamicPeerScore(pid peer.ID) *DynamicPeerScore {
	return &DynamicPeerScore{
		pid: pid,
	}
}

// String returns the peer score as a human-readable string.
func (s *DynamicPeerScore) String(t time.Time) string {
	list.New()
	s.mtx.Lock()
	r := fmt.Sprintf("achievement %v + punishment %v at %v = %v as of now",
		s.achievement, s.punishment, s.lastUnix, s.Score(t))
	s.mtx.Unlock()
	return r
}

// Score returns the current peer score, the sum of the achieved and
// punished scores.
//
// This function is safe for concurrent access.
func (s *DynamicPeerScore) Score(t time.Time) int64 {
	s.mtx.Lock()
	r := s.score(t)
	s.mtx.Unlock()
	return r
}

// score returns the peer score, the sum of the achieved and punished scores at a
// given point in time.
//
// This function is not safe for concurrent access. It is intended to be used
// internally and during testing.
func (s *DynamicPeerScore) score(t time.Time) int64 {

	dt := t.UnixNano()/1e6 - s.lastUnix
	s.verifyLifeTime(dt)

	if dt > 0 {
		var punishment, achievement int
		if s.timeOutCounter > punishConnTimeOutThreshold {
			logger.Errorf("timeOutCounter %v", s.timeOutCounter)
			punishment += punishConnTimeOutScore * s.timeOutCounter
			s.timeOutCounter = 0
		}
		if s.badBlockCounter > punishBadBlockThreshold {
			logger.Errorf(" badBlockCounter %v", s.badBlockCounter)
			punishment += punishBadBlockScore * s.badBlockCounter
			s.badBlockCounter = 0
		}
		if s.badTxCounter > punishBadTxThreshold {
			logger.Errorf("badTxCounter %v", s.badTxCounter)
			punishment += punishBadTxScore * s.badTxCounter
			s.badTxCounter = 0
		}
		if s.syncCounter > punishSyncMsgThreshold {
			logger.Errorf("syncCounter %v", s.syncCounter)
			punishment += punishSyncMsgScore * s.syncCounter
			s.syncCounter = 0
		}
		if s.hbCounter < punishHeartBeatCeiling {
			logger.Errorf("hbCounter %v", s.hbCounter)
			punishment += punishNoHeartBeatScore
			s.hbCounter = 0
		}
		if s.disconnCounter > punishDisconnThreshold {
			logger.Errorf("disconnCounter %v", s.disconnCounter)
			punishment += punishConnUnsteadinessScore
			s.disconnCounter = 0
		}
		if s.newBlockCounter > rewardNewBlockThreshold {
			logger.Errorf("newBlockCounter %v", s.newBlockCounter)
			achievement += rewardNewBlockScore * s.newBlockCounter
			s.newBlockCounter = 0
		}
		if s.newTxCounter > rewardNewTxThreshold {
			logger.Errorf("newTxCounter %v", s.newTxCounter)
			achievement += rewardNewTxScore * s.newTxCounter
			s.newTxCounter = 0
		}
		logger.Errorf("punishment = %v, achievement = %v", punishment, achievement)
		s.punish(int64(punishment), t)
		s.reward(int64(achievement), t)

		return baseScore + int64(s.achievement) - int64(s.punishment)
	}

	return baseScore + int64(s.achievement*RechieveFactors.decayRate(dt)) - int64(s.punishment*PunishFactors.decayRate(dt))
}

// verifyLifeTime reset punishment or achievement when lifetime < dt
func (s *DynamicPeerScore) verifyLifeTime(dt int64) {
	if PunishFactors.lifetime < int(dt/1000) {
		s.punishment = 0
	}
	if RechieveFactors.lifetime < int(dt/1000) {
		s.achievement = 0
	}
}

// reward increases the achievement. The resulting score is calculated
// as if the action was carried out at the point time. The resulting
// score is returned.
//
// This function is not safe for concurrent access.
func (s *DynamicPeerScore) reward(achievement int64, t time.Time) int64 {
	tu := t.UnixNano() / 1e6
	dt := tu - s.lastUnix

	if s.lastUnix != 0 {
		s.verifyLifeTime(dt)
	}

	if dt > 0 {
		if s.achievement > 1 {
			s.achievement *= RechieveFactors.decayRate(dt)
		}
		if s.punishment > 1 {
			s.punishment *= PunishFactors.decayRate(dt)
		}
		s.achievement += float64(achievement)
		if s.achievement > rewardLimit {
			s.achievement = rewardLimit
		}
		s.lastUnix = tu
	}
	return baseScore + int64(s.achievement) - int64(s.punishment)
}

// punish increases the punishment. The resulting score is calculated
// as if the action was carried out at the point time. The resulting
// score is returned.
//
// This function is not safe for concurrent access.
func (s *DynamicPeerScore) punish(punishment int64, t time.Time) int64 {
	tu := t.UnixNano() / 1e6
	dt := tu - s.lastUnix

	if s.lastUnix != 0 {
		s.verifyLifeTime(dt)
	}

	if dt > 0 {
		if s.achievement > 1 {
			s.achievement *= RechieveFactors.decayRate(dt)
		}
		if s.punishment > 1 {
			s.punishment *= PunishFactors.decayRate(dt)
		}
		s.punishment += float64(punishment)
		if s.punishment > punishLimit {
			s.punishment = punishLimit
		}
		s.lastUnix = tu
	}

	return baseScore + int64(s.achievement) - int64(s.punishment)
}

// Record record event
func (s *DynamicPeerScore) Record(event eventbus.BusEvent) {
	switch event {
	case eventbus.ConnTimeOutEvent:
		s.timeOutCounter++
	case eventbus.BadBlockEvent:
		s.badBlockCounter++
	case eventbus.BadTxEvent:
		s.badTxCounter++
	case eventbus.SyncMsgEvent:
		s.syncCounter++
	case eventbus.HeartBeatEvent:
		s.hbCounter++
	case eventbus.NewBlockEvent:
		s.newBlockCounter++
	case eventbus.NewTxEvent:
		s.newTxCounter++
	case eventbus.PeerDisconnEvent:
		s.disconnCounter++
	default:
		logger.Debugf("No such event found: %v", event)
	}
}
