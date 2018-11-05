// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package p2p

import (
	"crypto/sha256"
	"sync"

	"github.com/BOXFoundation/boxd/crypto"
	lru "github.com/hashicorp/golang-lru"
	"github.com/jbenet/goprocess"
)

const (
	repeatableMsg = iota
	uniqueBodyMsg
	uniqueBodyMsgPerPeer
)

var msgTypeToFilter = map[uint32]uint32{
	NewBlockMsg:    uniqueBodyMsg,
	ChainUpdateMsg: uniqueBodyMsgPerPeer,
}

// Notifier dispatcher & distribute business message.
type Notifier struct {
	notifierMap *sync.Map
	proc        goprocess.Process
	receiveCh   chan Message
	cache       *lru.Cache
}

// Notifiee represent message receiver.
type Notifiee struct {
	code      uint32
	messageCh chan Message
}

// NewNotifier new a notifiee
func NewNotifier() *Notifier {
	notifier := &Notifier{
		notifierMap: new(sync.Map),
		receiveCh:   make(chan Message, 65536),
	}
	notifier.cache, _ = lru.New(512)
	return notifier
}

// NewNotifiee return a message notifiee.
func NewNotifiee(code uint32, messageCh chan Message) *Notifiee {
	return &Notifiee{code: code, messageCh: messageCh}
}

// Subscribe notifier
func (notifier *Notifier) Subscribe(notifiee *Notifiee) {
	notifier.notifierMap.Store(notifiee.code, notifiee)
}

// UnSubscribe notifiee
func (notifier *Notifier) UnSubscribe(notifiee *Notifiee) {
	notifier.notifierMap.Delete(notifiee.code)
}

// Loop handle notifiee message
func (notifier *Notifier) Loop(parent goprocess.Process) {
	notifier.proc = parent.Go(func(p goprocess.Process) {
		for {
			select {
			case msg := <-notifier.receiveCh:
				code := msg.Code()
				notifiee, _ := notifier.notifierMap.Load(code)
				if notifiee != nil {
					notifiee.(*Notifiee).messageCh <- msg
				}
			case <-p.Closing():
				logger.Info("Quit notifier loop.")
				return
			}
		}
	})
}

// Notify message to notifier
func (notifier *Notifier) Notify(msg Message) {
	notify := notifier.filter(msg)
	if notify {
		notifier.receiveCh <- msg
	}
}

func (notifier *Notifier) filter(msg Message) bool {
	if msgTypeToFilter[msg.Code()] == repeatableMsg {
		return true
	}
	key := notifier.lruKey(msg)
	if notifier.cache.Contains(key) {
		return false
	}
	notifier.cache.Add(key, msg)
	return true
}

func (notifier *Notifier) lruKey(msg Message) crypto.HashType {
	key := []byte(msg.Body())

	if msgTypeToFilter[msg.Code()] == uniqueBodyMsgPerPeer {
		key = append(key, msg.From()...)
	}

	hash := sha256.Sum256(msg.Body())

	return hash
}
