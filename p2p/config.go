// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package p2p

import (
	"time"
)

// Config for peer configuration
type Config struct {
	Magic      uint32
	KeyPath    string
	Port       uint32
	Seeds      []string
	Bucketsize int
	Latency    time.Duration
}
