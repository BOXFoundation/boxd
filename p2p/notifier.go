// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package p2p

import (
	"sync"

	"github.com/jbenet/goprocess"
)

// Notifier dispatcher & distribute business message.
type Notifier struct {
	notifierMap *sync.Map
	proc        goprocess.Process
	receiveCh   chan Message
}

// Notifiee represent message receiver.
type Notifiee struct {
	Code      uint32
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

// Subscribe notifier
func (notifier *Notifier) Subscribe(notifiee *Notifiee) {
	notifier.notifierMap.Store(notifiee.Code, notifiee)
}

// UnSubscribe notifiee
func (notifier *Notifier) UnSubscribe(notifiee *Notifiee) {
	notifier.notifierMap.Delete(notifiee.Code)
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
	notifier.receiveCh <- msg
}

// NotifierMessage define the notifier message struct and implements Message interface.
type NotifierMessage struct {
	code uint32
	body []byte
}

// NewNotifierMessage new a notifier message
func NewNotifierMessage(code uint32, body []byte) *NotifierMessage {
	return &NotifierMessage{code: code, body: body}
}

// Code return notifier message code
func (msg *NotifierMessage) Code() uint32 {
	return msg.code
}

// Body return notifier message body
func (msg *NotifierMessage) Body() []byte {
	return msg.body
}
