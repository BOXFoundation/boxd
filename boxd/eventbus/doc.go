// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

// Package eventbus provides two message models: pub/sub and send/reply.
// There may be multiple subscribers subscribed to one topic, but there
// must be only one replier replying a topic.
//
// New a EventBus:
//
//   var bus = New()
//
// Get a global default EventBus:
//
//   var bus = Default()
//
// Subscriber:
//
//   func handler(a int, b int, out *int) {
//   	(*out) = a + b
//   }
//
//   bus.Subscribe("topic", handler)
//
// or handler will be triggerred async:
//
//   bus.SubscribeAsync("async:topic", handler, false)
//
// Publisher:
//
//   var out int
//   bus.Publish("topic", 10, 13, &out)
//   fmt.Print(out) // 23
//
//   var out2 int
//   bus.Publish("async:topic", 10, 13, &out2)
//   fmt.Print(out2) // 0, as the subscriber is triggerred async
//
// Replier:
//
//   func worker(a int, b int, out chan<- int) {
//   	out <- a + b
//   }
//
//   bus.Reply("task:add", worker, false)
//
// Sender:
//
//   var c = make(chan int)
//   bus.Send("task:add", 11, 11, c)
//   fmt.Print(<-c) // 22, replier is triggerred async

package eventbus
