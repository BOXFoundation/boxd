// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package p2p

import (
	"time"

	"github.com/spf13/viper"
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

// NewP2PConfig create a p2p config
func NewP2PConfig(v *viper.Viper) *Config {

	config := &Config{
		Magic:      uint32(v.GetInt32("p2p.magic")),
		KeyPath:    v.GetString("p2p.key_path"),
		Port:       uint32(v.GetInt32("p2p.port")),
		Seeds:      v.GetStringSlice("p2p.seeds"),
		Bucketsize: v.GetInt("p2p.bucket_size"),
		Latency:    v.GetDuration("p2p.latency"),
	}
	return config
}
