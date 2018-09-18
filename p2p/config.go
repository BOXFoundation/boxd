// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package p2p

type Config struct {
	Magic   uint32
	KeyPath string
	Port    uint32
	Seeds   []string
}
