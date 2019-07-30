// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package p2p

import (
	"time"
)

// Config for peer configuration
type Config struct {
	Type            string        `mapstructure:"type"`
	Magic           uint32        `mapstructure:"magic"`
	KeyPath         string        `mapstructure:"key_path"`
	Port            uint32        `mapstructure:"port"`
	Address         string        `mapstructure:"address"`
	Seeds           []string      `mapstructure:"seeds"`
	Principals      []string      `mapstructure:"principals"`
	Agents          []string      `mapstructure:"agents"`
	Bucketsize      int           `mapstructure:"bucket_size"`
	Latency         time.Duration `mapstructure:"latency"`
	AddPeers        []string      `mapstructure:"addpeer"`
	ConnMaxCapacity uint32        `mapstructure:"conn_max_capacity"`
	ConnLoadFactor  float32       `mapstructure:"conn_load_factor"`
	RelaySize       uint32        `mapstructure:"relay_size"`
}
