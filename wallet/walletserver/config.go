// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package walletserver

// Config contains config information for wallet server
type Config struct {
	Enable    bool `mapstructure:"enable"`
	CacheSize int  `mapstructure:"cache_size"`
}
