// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

// Modify https://github.com/asaskevich/EventBus to support Send/Reply

package eventbus

import (
	"fmt"
	"reflect"
	"sync"
)

//BusSubscriber defines subscription-related bus behavior
type BusSubscriber interface {
	Subscribe(topic string, fn interface{}) error
	SubscribeAsync(topic string, fn interface{}, transactional bool) error
	SubscribeOnce(topic string, fn interface{}) error
	SubscribeOnceAsync(topic string, fn interface{}) error
	Unsubscribe(topic string, handler interface{}) error
}

//BusPublisher defines publishing-related bus behavior
type BusPublisher interface {
	Publish(topic string, args ...interface{})
}

//MsgReplier defines worker behavior for message sent by sender
type MsgReplier interface {
	Reply(topic string, fn interface{}, transactional bool) error
	StopReply(topic string, fn interface{}) error
}

//MsgSender sends message to replier who should reply the message
type MsgSender interface {
	Send(topic string, args ...interface{})
}

//BusController defines bus control behavior (checking handler's presence, synchronization)
type BusController interface {
	HasSubscriber(topic string) bool
	HasReplier(topic string) bool
	WaitAsync()
}

//Bus englobes global (subscribe, publish, control) bus behavior
type Bus interface {
	BusController
	BusSubscriber
	BusPublisher
	MsgReplier
	MsgSender
}

// EventBus - box for handlers and callbacks.
type EventBus struct {
	pubHandlers map[string][]*eventHandler
	subLock     sync.Mutex // a lock for the map

	sendHandlers map[string]*eventHandler
	replyLock    sync.Mutex // a lock for the map

	wg sync.WaitGroup
}

type eventHandler struct {
	callBack      reflect.Value
	flagOnce      bool
	async         bool
	transactional bool
	sync.Mutex    // lock for an event handler - useful for running async callbacks serially
}

var defaultBus Bus

func init() {
	defaultBus = New()
}

// Default returns the default EventBus.
func Default() Bus {
	return defaultBus
}

// New returns new EventBus with empty handlers.
func New() Bus {
	return &EventBus{
		pubHandlers:  make(map[string][]*eventHandler),
		subLock:      sync.Mutex{},
		sendHandlers: make(map[string]*eventHandler),
		replyLock:    sync.Mutex{},
		wg:           sync.WaitGroup{},
	}
}

// doSubscribe handles the subscription logic and is utilized by the public Subscribe functions
func (bus *EventBus) doSubscribe(topic string, fn interface{}, handler *eventHandler) error {
	bus.subLock.Lock()
	defer bus.subLock.Unlock()
	if !(reflect.TypeOf(fn).Kind() == reflect.Func) {
		return fmt.Errorf("%s is not of type reflect.Func", reflect.TypeOf(fn).Kind())
	}
	bus.pubHandlers[topic] = append(bus.pubHandlers[topic], handler)
	return nil
}

// Subscribe subscribes to a topic.
// Returns error if `fn` is not a function.
func (bus *EventBus) Subscribe(topic string, fn interface{}) error {
	return bus.doSubscribe(topic, fn, &eventHandler{
		reflect.ValueOf(fn), false, false, false, sync.Mutex{},
	})
}

// SubscribeAsync subscribes to a topic with an asynchronous callback
// Transactional determines whether subsequent callbacks for a topic are
// run serially (true) or concurrently (false)
// Returns error if `fn` is not a function.
func (bus *EventBus) SubscribeAsync(topic string, fn interface{}, transactional bool) error {
	return bus.doSubscribe(topic, fn, &eventHandler{
		reflect.ValueOf(fn), false, true, transactional, sync.Mutex{},
	})
}

// SubscribeOnce subscribes to a topic once. Handler will be removed after executing.
// Returns error if `fn` is not a function.
func (bus *EventBus) SubscribeOnce(topic string, fn interface{}) error {
	return bus.doSubscribe(topic, fn, &eventHandler{
		reflect.ValueOf(fn), true, false, false, sync.Mutex{},
	})
}

// SubscribeOnceAsync subscribes to a topic once with an asynchronous callback
// Handler will be removed after executing.
// Returns error if `fn` is not a function.
func (bus *EventBus) SubscribeOnceAsync(topic string, fn interface{}) error {
	return bus.doSubscribe(topic, fn, &eventHandler{
		reflect.ValueOf(fn), true, true, false, sync.Mutex{},
	})
}

// HasSubscriber returns true if exists any callback subscribed to the topic.
func (bus *EventBus) HasSubscriber(topic string) bool {
	bus.subLock.Lock()
	defer bus.subLock.Unlock()
	_, ok := bus.pubHandlers[topic]
	if ok {
		return len(bus.pubHandlers[topic]) > 0
	}
	return false
}

// HasReplier returns true if exists a receiver on the topic.
func (bus *EventBus) HasReplier(topic string) bool {
	bus.replyLock.Lock()
	defer bus.replyLock.Unlock()
	_, ok := bus.sendHandlers[topic]
	return ok
}

// Unsubscribe removes callback defined for a topic.
// Returns error if there are no callbacks subscribed to the topic.
func (bus *EventBus) Unsubscribe(topic string, handler interface{}) error {
	bus.subLock.Lock()
	defer bus.subLock.Unlock()
	if _, ok := bus.pubHandlers[topic]; ok && len(bus.pubHandlers[topic]) > 0 {
		bus.removeHandler(topic, reflect.ValueOf(handler))
		return nil
	}
	return fmt.Errorf("topic %s doesn't exist", topic)
}

// Publish executes callback defined for a topic. Any additional argument will be transferred to the callback.
func (bus *EventBus) Publish(topic string, args ...interface{}) {
	bus.subLock.Lock() // will unlock if handler is not found or always after setUpPublish
	defer bus.subLock.Unlock()
	if handlers, ok := bus.pubHandlers[topic]; ok && 0 < len(handlers) {
		// Handlers slice may be changed by removeHandler and Unsubscribe during iteration,
		// so make a copy and iterate the copied slice.
		copyHandlers := make([]*eventHandler, 0, len(handlers))
		copyHandlers = append(copyHandlers, handlers...)
		for _, handler := range copyHandlers {
			if handler.flagOnce {
				bus.removeHandler(topic, handler.callBack)
			}
			if !handler.async {
				bus.doPublish(handler, args...)
			} else {
				bus.wg.Add(1)
				go bus.doPublishAsync(handler, args...)
			}
		}
	}
}

func (bus *EventBus) doPublish(handler *eventHandler, args ...interface{}) {
	passedArguments := bus.setUpPublish(args...)
	handler.callBack.Call(passedArguments)
}

func (bus *EventBus) doPublishAsync(handler *eventHandler, args ...interface{}) {
	defer bus.wg.Done()
	if handler.transactional {
		handler.Lock()
		defer handler.Unlock()
	}
	bus.doPublish(handler, args...)
}

func (bus *EventBus) removeHandler(topic string, callback reflect.Value) {
	if handlers, ok := bus.pubHandlers[topic]; ok {
		var copy = make([]*eventHandler, 0)
		for _, h := range handlers {
			if h.callBack != callback {
				copy = append(copy, h)
			}
		}
		bus.pubHandlers[topic] = copy
	}
}

func (bus *EventBus) setUpPublish(args ...interface{}) []reflect.Value {
	passedArguments := make([]reflect.Value, 0)
	for _, arg := range args {
		passedArguments = append(passedArguments, reflect.ValueOf(arg))
	}
	return passedArguments
}

// Reply receives send-reply message on a topic.
// There should be only one function receiving message on one topic.
// Transactional determines whether subsequent callbacks for a topic are
// run serially (true) or concurrently (false)
// Returns error if `fn` is not a function.
func (bus *EventBus) Reply(topic string, fn interface{}, transactional bool) error {
	bus.replyLock.Lock()
	defer bus.replyLock.Unlock()

	v := reflect.ValueOf(fn)
	if v.Kind() != reflect.Func {
		return fmt.Errorf("%s is not of type reflect.Func", v.Kind())
	}
	t := reflect.TypeOf(fn)
	if t.NumIn() < 1 {
		return fmt.Errorf("%s should have at least one chan parameter to receive result", t.Kind())
	}

	if _, ok := bus.sendHandlers[topic]; ok {
		return fmt.Errorf("topic %s already has a receiver", topic)
	}

	bus.sendHandlers[topic] = &eventHandler{
		v, false, false, transactional, sync.Mutex{},
	}
	return nil
}

// StopReply removes replier callback defined for a topic.
// Returns error if there is no callback is receiving the topic.
func (bus *EventBus) StopReply(topic string, fn interface{}) error {
	bus.replyLock.Lock()
	defer bus.replyLock.Unlock()

	if handler, ok := bus.sendHandlers[topic]; ok {
		if handler.callBack == reflect.ValueOf(fn) {
			delete(bus.sendHandlers, topic)
		}
		return nil
	}
	return fmt.Errorf("topic %s doesn't exist", topic)

}

// Send sends a send-reply message on a topic to replier
func (bus *EventBus) Send(topic string, args ...interface{}) {
	bus.replyLock.Lock()
	defer bus.replyLock.Unlock()

	if handler, ok := bus.sendHandlers[topic]; ok {
		bus.wg.Add(1)
		go bus.doPublishAsync(handler, args...)
	}
}

// WaitAsync waits for all async callbacks to complete
func (bus *EventBus) WaitAsync() {
	bus.wg.Wait()
}
