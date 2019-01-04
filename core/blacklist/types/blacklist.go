// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package bltypes

import (
	"sync"

	"github.com/BOXFoundation/boxd/boxd/eventbus"
	"github.com/BOXFoundation/boxd/p2p"
	lru "github.com/hashicorp/golang-lru"
	"github.com/jbenet/goprocess"
)

var (
	blackList *BlackList
)

// BlackList represents the black list of public keys
type BlackList struct {
	// checksumIEEE(pubKey) -> struct{}{}
	Details *sync.Map
	SceneCh chan *Evidence
	// checksumIEEE(pubKey) -> []ch
	evidenceNote *lru.Cache

	// checkousum(hash) -> [][]byte([]signature)
	confirmMsgNote *lru.Cache
	// checkousum(hash) -> struct{}{}
	existConfirmedKey *lru.Cache

	bus      eventbus.Bus
	notifiee p2p.Net
	msgCh    chan p2p.Message
	proc     goprocess.Process
	mutex    *sync.Mutex
}
