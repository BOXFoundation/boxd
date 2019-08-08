// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package service

// MinerReader reads the status information of miners and candidates during the era.
type MinerReader interface {
	Miners() []string

	Candidates() []string
}