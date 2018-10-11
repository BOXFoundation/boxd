// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package eventbus

import (
	"sync"
	"testing"
	"time"

	"github.com/facebookgo/ensure"
)

func TestNew(t *testing.T) {
	bus := New()
	if bus == nil {
		t.Log("New EventBus not created!")
		t.Fail()
	}
}

func TestHasSubscriber(t *testing.T) {
	bus := New()
	bus.Subscribe("topic", func() {})
	ensure.False(t, bus.HasSubscriber("topic_topic"))
	ensure.True(t, bus.HasSubscriber("topic"))
}

func TestSubscribe(t *testing.T) {
	bus := New()
	ensure.Nil(t, bus.Subscribe("topic", func() {}))
	ensure.NotNil(t, bus.Subscribe("topic", "String"))
}

func TestSubscribeOnce(t *testing.T) {
	bus := New()
	ensure.Nil(t, bus.SubscribeOnce("topic", func() {}))
	ensure.NotNil(t, bus.SubscribeOnce("topic", "String"))
}

func TestSubscribeOnceAndManySubscribe(t *testing.T) {
	bus := New()
	event := "topic"
	flag := 0
	fn := func() { flag++ }
	bus.SubscribeOnce(event, fn)
	bus.Subscribe(event, fn)
	bus.Subscribe(event, fn)
	bus.Publish(event)

	ensure.DeepEqual(t, flag, 3)
}

func TestUnsubscribe(t *testing.T) {
	bus := New()
	handler := func() {}
	handler2 := func() {}
	bus.Subscribe("topic", handler)
	bus.Subscribe("topic", handler2)
	ensure.Nil(t, bus.Unsubscribe("topic", handler))

	bus.Subscribe("topic2", handler)
	ensure.Nil(t, bus.Unsubscribe("topic2", handler))
	ensure.NotNil(t, bus.Unsubscribe("topic2", handler))
}

func TestPublish(t *testing.T) {
	bus := New()
	bus.Subscribe("topic", func(a int, b int) {
		ensure.DeepEqual(t, a, b)
	})
	bus.Publish("topic", 10, 10)
}

func TestSubcribeOnceAsync(t *testing.T) {
	results := make([]int, 0)

	var sm sync.Mutex

	bus := New()
	bus.SubscribeOnceAsync("topic", func(a int, out *[]int) {
		sm.Lock()
		defer sm.Unlock()
		*out = append(*out, a)
	})

	bus.Publish("topic", 10, &results)
	bus.Publish("topic", 10, &results)

	bus.WaitAsync()

	ensure.DeepEqual(t, len(results), 1)
	ensure.False(t, bus.HasSubscriber("topic"))
}

func TestSubscribeAsyncTransactional(t *testing.T) {
	results := make([]int, 0)

	bus := New()
	bus.SubscribeAsync("topic", func(a int, out *[]int) {
		*out = append(*out, a)
	}, true)

	for i := 0; i < 100; i++ {
		bus.Publish("topic", i, &results)
	}

	bus.WaitAsync()

	ensure.DeepEqual(t, len(results), 100)
}

func TestSubscribeAsync(t *testing.T) {
	results := make(chan int)

	bus := New()
	bus.SubscribeAsync("topic", func(a int, out chan<- int) {
		out <- a
	}, false)

	bus.Publish("topic", 1, results)
	bus.Publish("topic", 2, results)

	numResults := 0

	var sm sync.Mutex
	go func() {
		for range results {
			sm.Lock()
			numResults++
			sm.Unlock()
		}
	}()

	bus.WaitAsync()

	time.Sleep(10 * time.Millisecond)
	close(results)

	sm.Lock()
	ensure.DeepEqual(t, numResults, 2)
	sm.Unlock()
}

func TestReceive(t *testing.T) {
	bus := New()
	ensure.Nil(t, bus.Reply("topic", func(_ int, out chan<- int) {}, false))
	ensure.NotNil(t, bus.Reply("topic", func(_ int, out chan<- int) {}, false))

	ensure.NotNil(t, bus.Reply("topic1", func() {}, false))
	ensure.NotNil(t, bus.Reply("topic2", "string", false))
}

func TestStopReply(t *testing.T) {
	bus := New()
	handler := func(_ int, out chan<- int) {}
	ensure.Nil(t, bus.Reply("topic", handler, false))
	ensure.True(t, bus.HasReplier("topic"))

	ensure.Nil(t, bus.StopReply("topic", handler))
	ensure.NotNil(t, bus.StopReply("topic", func() {}))
	ensure.False(t, bus.HasReplier("topic"))
	ensure.NotNil(t, bus.StopReply("topic", handler))

	ensure.NotNil(t, bus.StopReply("topic2", handler))
}

func TestHasReplier(t *testing.T) {
	bus := New()
	bus.Reply("topic", func(_ int) {}, false)
	ensure.False(t, bus.HasReplier("topic_topic"))
	ensure.True(t, bus.HasReplier("topic"))
}

func TestSend(t *testing.T) {
	bus := New()
	bus.Reply("topic", func(a int, b int, out chan<- int) {
		out <- a + b
	}, false)
	out := make(chan int)
	bus.Send("topic", 10, 10, out)
	r := <-out
	ensure.DeepEqual(t, r, 20)
}

type O struct{}

func (o O) work(a int, b int, out chan<- int) {
	out <- a + b
}

func TestSendObject(t *testing.T) {
	bus := New()
	bus.Reply("topic", O{}.work, false)
	out := make(chan int)
	bus.Send("topic", 10, 10, out)
	r := <-out
	ensure.DeepEqual(t, r, 20)
}

func TestTransactionalReplier(t *testing.T) {
	bus := New()
	var i = 0
	bus.Reply("topic", func(out chan<- int) {
		time.Sleep(10 * time.Millisecond)
		out <- i
		i++
	}, true)

	out := make(chan int)
	for j := 0; j < 32; j++ {
		bus.Send("topic", out)
	}

	var j = 0
	for result := range out {
		ensure.DeepEqual(t, j, result)
		j++
		if j == 32 {
			close(out)
		}
	}
}
