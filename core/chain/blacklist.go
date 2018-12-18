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
	blackListThreshold    = 100
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
	msgCh      chan uint32
}

func init() {
	blackList = &BlackList{
		blackList: new(sync.Map),
		mutex:     &sync.Mutex{},
		msgCh:     make(chan uint32, 65536),
	}
	blackList.keyCounter, _ = lru.New(4096)
}

// Default returns the default BlackList.
func Default() *BlackList {
	return blackList
}

func (bl *BlackList) run(parent goprocess.Process) {
	logger.Info("Start blacklist loop")
	ticker := time.NewTicker(blackListLoopInterval * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			bl.statistics()
		case checksum := <-blackList.msgCh:
			if val, ok := blackList.keyCounter.Get(checksum); ok {
				blackList.keyCounter.Add(checksum, val.(uint32)+1)
			} else {
				blackList.keyCounter.Add(checksum, 0)
			}
		case <-parent.Closing():
			logger.Info("Stopped black list loop.")
			return
		}
	}
}

func (bl *BlackList) statistics() {
	keys := bl.keyCounter.Keys()
	for _, key := range keys {
		if value, ok := bl.keyCounter.Get(key); ok {
			if value.(int) > blackListThreshold*blackListLoopInterval {
				logger.Warnf("%v is added into blacklist.", key)
				bl.blackList.Store(key, struct{}{})
			}
		}
	}
	bl.keyCounter.Purge()
}
