// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package chain

import (
	"sync"
	"time"

	lru "github.com/hashicorp/golang-lru"
	"github.com/jbenet/goprocess"
)

const (
	blackListLoopInterval = 3
	blackListThreshold    = 2
)

var (
	blackList *BlackList
)

// BlackList represents the black list of public keys
type BlackList struct {
	mutex *sync.Mutex
	// checksumIEEE(pubKey) -> struct{}{}
	blackList *sync.Map
	// checksumIEEE(pubKey) -> uint32
	keyCounter *lru.Cache
	proc       goprocess.Process
}

func init() {
	blackList = New()
}

// Default returns the default BlackList.
func Default() *BlackList {
	return blackList
}

// New returns new BlackList.
func New() *BlackList {
	bl := &BlackList{
		blackList: new(sync.Map),
		mutex:     &sync.Mutex{},
	}
	bl.keyCounter, _ = lru.New(4096)

	go func() {
		logger.Info("Start blacklist loop")
		ticker := time.NewTicker(blackListLoopInterval * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				bl.statistics()
				// case <-p.Closing():
				// 	logger.Info("Stopped black list loop.")
				// 	return
			}
		}
	}()

	return bl
}

func (bl *BlackList) statistics() {
	// TODO: synchronization issue? conditions need to be considered
	keys := bl.keyCounter.Keys()
	for _, key := range keys {
		if value, ok := bl.keyCounter.Get(key); ok {
			if value.(uint32) > blackListThreshold*blackListLoopInterval {
				logger.Warnf("%v is added into black list.", key)
				bl.blackList.Store(key, struct{}{})
			}
		}
	}
	bl.keyCounter.Purge()
}
