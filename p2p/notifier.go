// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package p2p

import (
	"sync"
	"time"

	"github.com/jbenet/goprocess"
)

const (
	metricsLoopInterval = 2 * time.Second
)

// Notifier dispatcher & distribute business message.
type Notifier struct {
	notifierMap *sync.Map
	proc        goprocess.Process
	receiveCh   chan Message
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
		metricsTicker := time.NewTicker(metricsLoopInterval)
		defer metricsTicker.Stop()
		for {
			select {
			case msg := <-notifier.receiveCh:
				code := msg.Code()
				//logger.Debugf("received network message. Code: %X From: %s", code, msg.From().Pretty())
				notifiee, _ := notifier.notifierMap.Load(code)
				if notifiee != nil {
					select {
					case notifiee.(*Notifiee).messageCh <- msg:
					default:
						logger.Warnf("Message handler is blocked. code: %X", msg.Code())
					}
				} else {
					logger.Debugf("Message is throwing away, %X from %v", code, msg.From().Pretty())
				}
			case <-metricsTicker.C:
				metricsRevieveChSizeGauge.Update(int64(len(notifier.receiveCh)))
			case <-p.Closing():
				logger.Info("Quit notifier loop.")
				return
			}
		}
	})
}

// Notify message to notifier
func (notifier *Notifier) Notify(msg Message) {
	notifier.receiveCh <- msg
}
