// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package p2p

import (
	"time"
)

// Config for peer configuration
type Config struct {
	Magic      uint32        `mapstructure:"magic"`
	KeyPath    string        `mapstructure:"key_path"`
	Port       uint32        `mapstructure:"port"`
	Address    string        `mapstructure:"address"`
	Seeds      []string      `mapstructure:"seeds"`
	Bucketsize int           `mapstructure:"bucket_size"`
	Latency    time.Duration `mapstructure:"latency"`
	AddPeers   []string      `mapstructure:"addpeer"`
}
