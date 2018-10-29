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
	// baseScore indicates the default score of the peer
	baseScore = 100

	punishLimit = 150

	awardLimit = 900

	// HeartBeatLatencyTime sadfa
	HeartBeatLatencyTime = 10000

	// DisconnTimesThreshold hgkjh
	DisconnTimesThreshold = 30000

	// DisconnMinTime hgkjh
	DisconnMinTime = 5

	// PunishConnTimeOut sdafa
	PunishConnTimeOut ScoreEvent = 40

	// PunishBadBlock sdafa
	PunishBadBlock ScoreEvent = 60

	// PunishBadTx sdafa
	PunishBadTx ScoreEvent = 30

	// PunishSyncMsg sdafa
	PunishSyncMsg ScoreEvent = 25

	// PunishNoHeartBeat sdafa
	PunishNoHeartBeat ScoreEvent = 90

	// PunishConnUnsteadiness sdafa
	PunishConnUnsteadiness ScoreEvent = 100

	// AwardNewBlock sdafa
	AwardNewBlock ScoreEvent = 80

	// AwardNewTx sdafa
	AwardNewTx ScoreEvent = 20
)

// ScoreEvent sadf
type ScoreEvent int64

var (
	// PunishFactors sadf
	PunishFactors = newFactors(60, 1800, 64)
	// AchieveFactors sadf
	AchieveFactors = newFactors(600, 18000, 512)
)

var logger = log.NewLogger("pscore") // logger

type factors struct {

	// halflife defines the time (in seconds) by which the transient part
	// of the ban score decays to one half of it's original value.
	halflife int

	// lambda is the decaying constant.
	lambda float64

	// lifetime defines the maximum age of the transient part of the ban
	// score to be considered a non-zero score (in seconds).
	lifetime int

	// precomputedLen defines the amount of decay factors (one per second) that
	// should be precomputed at initialization.
	precomputedLen int

	// precomputedFactor stores precomputed exponential decay factors for the first
	// 'precomputedLen' seconds starting from t == 0.
	precomputedFactor []float64

	scoreCh chan int64
}

// NewFactors asdfas
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
	factors.scoreCh = make(chan int64, 65536)
	return factors
}

// decayRate returns the decay rate at t seconds, using precalculated values
// if available, or calculating the rate if needed.
func (factors *factors) decayRate(t int64) float64 {
	t = int64(t / 1000)
	if t < int64(factors.precomputedLen) {
		return factors.precomputedFactor[t]
	}
	return math.Exp(-1.0 * float64(t) * factors.lambda)
}

// DynamicPeerScore provides dynamic ban scores consisting of a persistent and a
// decaying component. The persistent score could be utilized to create simple
// additive banning policies similar to those found in other bitcoin node
// implementations.
//
// The decaying score enables the creation of evasive logic which handles
// misbehaving peers (especially application layer DoS attacks) gracefully
// by disconnecting and banning peers attempting various kinds of flooding.
// DynamicPeerScore allows these two approaches to be used in tandem.
//
// Zero value: Values of type DynamicPeerScore are immediately ready for use upon
// declaration.
type DynamicPeerScore struct {
	pid         peer.ID
	lastUnix    int64
	punishment  float64
	achievement float64
	connRecords *list.List
	mtx         sync.Mutex
}

// NewDynamicPeerScore asda
func NewDynamicPeerScore(pid peer.ID) *DynamicPeerScore {
	return &DynamicPeerScore{
		pid:         pid,
		connRecords: list.New(),
	}
}

// ConnRecords saf
func (s *DynamicPeerScore) ConnRecords() *list.List {
	return s.connRecords
}

// String returns the ban score as a human-readable string.
func (s *DynamicPeerScore) String() string {
	list.New()
	s.mtx.Lock()
	r := fmt.Sprintf("achievement %v + punishment %v at %v = %v as of now",
		s.achievement, s.punishment, s.lastUnix, s.Int())
	s.mtx.Unlock()
	return r
}

// Int returns the current ban score, the sum of the persistent and decaying
// scores.
//
// This function is safe for concurrent access.
func (s *DynamicPeerScore) Int() int64 {
	s.mtx.Lock()
	r := s.int(time.Now())
	s.mtx.Unlock()
	return r
}

// Award increases both the persistent and decaying scores by the values
// passed as parameters. The resulting score is returned.
//
// This function is safe for concurrent access.
func (s *DynamicPeerScore) Award(achievement ScoreEvent) int64 {
	s.mtx.Lock()
	r := s.award(int64(achievement), time.Now())
	s.mtx.Unlock()
	return r
}

// Punish asdfas
func (s *DynamicPeerScore) Punish(punishment ScoreEvent) int64 {
	s.mtx.Lock()
	r := s.punish(int64(punishment), time.Now())
	s.mtx.Unlock()
	return r
}

// Disconnect asdfas
func (s *DynamicPeerScore) Disconnect() {
	t := time.Now().UnixNano() / 1e6
	s.connRecords.PushBack(t)
	s.Reset(t)
}

// Reset set both persistent and decaying scores to zero.
//
// This function is safe for concurrent access.
func (s *DynamicPeerScore) Reset(t int64) {
	s.mtx.Lock()
	s.achievement = 0
	s.lastUnix = t
	s.mtx.Unlock()
}

// int returns the ban score, the sum of the persistent and decaying scores at a
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

// award increases the achievement, the decaying or both scores by the values
// passed as parameters. The resulting score is calculated as if the action was
// carried out at the point time represented by the third parameter. The
// resulting score is returned.
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
