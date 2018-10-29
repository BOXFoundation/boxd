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

	"github.com/BOXFoundation/boxd/log"
	peer "github.com/libp2p/go-libp2p-peer"
)

const (
	// baseScore indicates the default score of the peer.
	baseScore = 100

	// punishLimit indicates the upper limit of publishment.
	punishLimit = 1000

	// awardLimit indicates the upper limit of achievement.
	awardLimit = 900

	// HeartBeatLatencyTime indicates the max latency time of hb.
	HeartBeatLatencyTime = 10000

	// DisconnTimesPeriod indicates the period of the peer's disconn queue.
	DisconnTimesPeriod = 30000

	// DisconnMinTime indicates the threshold of disconn time through DisconnTimesPeriod.
	DisconnMinTime = 5

	// PunishConnTimeOut indicates the punishment if the conn time out.
	PunishConnTimeOut ScoreEvent = 40

	// PunishBadBlock indicates the punishment if process new block throwing err.
	PunishBadBlock ScoreEvent = 60

	// PunishBadTx indicates the punishment if process new tx throwing err.
	PunishBadTx ScoreEvent = 30

	// PunishSyncMsg indicates the punishment when receive sync msg.
	PunishSyncMsg ScoreEvent = 25

	// PunishNoHeartBeat indicates the punishment when long time no receive hb.
	PunishNoHeartBeat ScoreEvent = 90

	// PunishConnUnsteadiness indicates the punishment when conn is not steady.
	PunishConnUnsteadiness ScoreEvent = 100

	// AwardNewBlock indicates the award for new block.
	AwardNewBlock ScoreEvent = 80

	// AwardNewTx indicates the award for new tx.
	AwardNewTx ScoreEvent = 20
)

// ScoreEvent means events happened to change score.
type ScoreEvent int64

var (
	// PunishFactors contains factors of punishment.
	PunishFactors = newFactors(60, 1800, 64)
	// AchieveFactors contains factors of achievement.
	AchieveFactors = newFactors(600, 18000, 512)
)

var logger = log.NewLogger("pscore")

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
	connRecords *list.List
	mtx         sync.Mutex
}

// NewDynamicPeerScore returns new DynamicPeerScore.
func NewDynamicPeerScore(pid peer.ID) *DynamicPeerScore {
	return &DynamicPeerScore{
		pid:         pid,
		connRecords: list.New(),
	}
}

// ConnRecords returns DynamicPeerScore.connRecords
func (s *DynamicPeerScore) ConnRecords() *list.List {
	return s.connRecords
}

// String returns the peer score as a human-readable string.
func (s *DynamicPeerScore) String() string {
	list.New()
	s.mtx.Lock()
	r := fmt.Sprintf("achievement %v + punishment %v at %v = %v as of now",
		s.achievement, s.punishment, s.lastUnix, s.Int())
	s.mtx.Unlock()
	return r
}

// Int returns the current peer score, the sum of the achieved and 
// punished scores.
//
// This function is safe for concurrent access.
func (s *DynamicPeerScore) Int() int64 {
	s.mtx.Lock()
	r := s.int(time.Now())
	s.mtx.Unlock()
	return r
}

// Award increases achieved scores by the values
// passed as parameters. The resulting score is returned.
//
// This function is safe for concurrent access.
func (s *DynamicPeerScore) Award(achievement ScoreEvent) int64 {
	s.mtx.Lock()
	r := s.award(int64(achievement), time.Now())
	s.mtx.Unlock()
	return r
}

// Punish increases achieved scores by the values
// passed as parameters. The resulting score is returned.
//
// This function is safe for concurrent access.
func (s *DynamicPeerScore) Punish(punishment ScoreEvent) int64 {
	s.mtx.Lock()
	r := s.punish(int64(punishment), time.Now())
	s.mtx.Unlock()
	return r
}

// Disconnect reset the peer's DynamicPeerScore
func (s *DynamicPeerScore) Disconnect() {
	t := time.Now().UnixNano() / 1e6
	s.connRecords.PushBack(t)
	s.Reset(t)
}

// Reset achieved scores to zero but not punished
//
// This function is safe for concurrent access.
func (s *DynamicPeerScore) Reset(t int64) {
	s.mtx.Lock()
	s.achievement = 0
	s.lastUnix = t
	s.mtx.Unlock()
}

// int returns the peer score, the sum of the achieved and punished scores at a
// given point in time.
//
// This function is not safe for concurrent access. It is intended to be used
// internally and during testing.
func (s *DynamicPeerScore) int(t time.Time) int64 {
	dt := t.UnixNano()/1e6 - s.lastUnix
	s.verifyLifeTime(dt)
	a := baseScore + int64(s.achievement*AchieveFactors.decayRate(dt)) - int64(s.punishment*PunishFactors.decayRate(dt))
	return a
}

// verifyLifeTime reset punishment or achievement when lifetime < dt
func (s *DynamicPeerScore) verifyLifeTime(dt int64) {
	if PunishFactors.lifetime < int(dt/1000) {
		s.punishment = 0
	}
	if AchieveFactors.lifetime < int(dt/1000) {
		s.achievement = 0
	}
}

// award increases the achievement. The resulting score is calculated 
// as if the action was carried out at the point time. The resulting 
// score is returned.
//
// This function is not safe for concurrent access.
func (s *DynamicPeerScore) award(achievement int64, t time.Time) int64 {
	tu := t.UnixNano() / 1e6
	dt := tu - s.lastUnix

	if s.lastUnix != 0 {
		s.verifyLifeTime(dt)
	}

	if dt > 0 {
		if s.achievement > 1 {
			s.achievement *= AchieveFactors.decayRate(dt)
		}
		if s.punishment > 1 {
			s.punishment *= PunishFactors.decayRate(dt)
		}
		s.achievement += float64(achievement)
		if s.achievement > awardLimit {
			s.achievement = awardLimit
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
			s.achievement *= AchieveFactors.decayRate(dt)
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
