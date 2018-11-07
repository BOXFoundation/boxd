// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package metrics

// Config for metrics configuration
type Config struct {
	Enable   bool     `mapstructure:"enable"`
	Host     string   `mapstructure:"host"`
	Db       string   `mapstructure:"db"`
	User     string   `mapstructure:"user"`
	Password string   `mapstructure:"password"`
	Tags     []string `mapstructure:"tags"`
}
