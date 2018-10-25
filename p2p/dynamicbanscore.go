package main

import (
	"fmt"
	"math"
	"sync"
	"time"
)

const (
	// punishment indicates that the peer need to be punished
	punishment = iota

	// achievement indicates that the peer need to be awarded
	achievement
)

const (
	// baseScore indicates the default score of the peer
	baseScore = 100
)

type Factors struct {

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
}

var (
	punishFactors  = NewFactors(60, 1800, 64)
	achieveFactors = NewFactors(600, 18000, 512)
)

func NewFactors(halflife, lifetime, precomputedLen int) *Factors {
	factors := new(Factors)
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

// decayRate returns the decay rate at t seconds, using precalculated values
// if available, or calculating the rate if needed.
func (factors *Factors) decayRate(t int64) float64 {
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
	lastUnix    int64
	punishment  float64
	achievement float64
	mtx         sync.Mutex
}

// String returns the ban score as a human-readable string.
func (s *DynamicPeerScore) String() string {
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
func (s *DynamicPeerScore) Int() uint32 {
	s.mtx.Lock()
	r := s.int(time.Now())
	s.mtx.Unlock()
	return r
}

// Increase increases both the persistent and decaying scores by the values
// passed as parameters. The resulting score is returned.
//
// This function is safe for concurrent access.
func (s *DynamicPeerScore) Award(achievement uint32) uint32 {
	s.mtx.Lock()
	r := s.award(achievement, time.Now())
	s.mtx.Unlock()
	return r
}

func (s *DynamicPeerScore) Punish(punishment uint32) uint32 {
	s.mtx.Lock()
	r := s.punish(punishment, time.Now())
	s.mtx.Unlock()
	return r
}

// Reset set both persistent and decaying scores to zero.
//
// This function is safe for concurrent access.
func (s *DynamicPeerScore) Reset() {
	s.mtx.Lock()
	s.punishment = 0
	s.achievement = 0
	s.lastUnix = 0
	s.mtx.Unlock()
}

// int returns the ban score, the sum of the persistent and decaying scores at a
// given point in time.
//
// This function is not safe for concurrent access. It is intended to be used
// internally and during testing.
func (s *DynamicPeerScore) int(t time.Time) uint32 {
	dt := t.Unix() - s.lastUnix
	s.verifyLifeTime(dt)
	return baseScore + uint32(s.achievement*achieveFactors.decayRate(dt)) - uint32(s.punishment*punishFactors.decayRate(dt))
}

// verifyLifeTime reset punishment or achievement when lifetime < dt
func (s *DynamicPeerScore) verifyLifeTime(dt int64) {
	if punishFactors.lifetime < int(dt) {
		s.punishment = 0
	}
	if achieveFactors.lifetime < int(dt) {
		s.achievement = 0
	}
}

// award increases the achievement, the decaying or both scores by the values
// passed as parameters. The resulting score is calculated as if the action was
// carried out at the point time represented by the third parameter. The
// resulting score is returned.
//
// This function is not safe for concurrent access.
func (s *DynamicPeerScore) award(achievement uint32, t time.Time) uint32 {
	tu := t.Unix()
	dt := tu - s.lastUnix

	s.verifyLifeTime(dt)

	if dt > 0 {
		if s.achievement > 1 {
			s.achievement *= achieveFactors.decayRate(dt)
		}
		if s.punishment > 1 {
			s.punishment *= punishFactors.decayRate(dt)
		}
		s.achievement += float64(achievement)
		s.lastUnix = tu
	}
	return baseScore + uint32(s.achievement) - uint32(s.punishment)
}

func (s *DynamicPeerScore) punish(punishment uint32, t time.Time) uint32 {
	tu := t.Unix()
	dt := tu - s.lastUnix

	s.verifyLifeTime(dt)

	if dt > 0 {
		if s.achievement > 1 {
			s.achievement *= achieveFactors.decayRate(dt)
		}
		if s.punishment > 1 {
			s.punishment *= punishFactors.decayRate(dt)
		}
		s.punishment += float64(punishment)
		s.lastUnix = tu
	}
	return baseScore + uint32(s.achievement) - uint32(s.punishment)
}
