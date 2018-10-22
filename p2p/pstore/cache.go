// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package pstore

// cache abstracts all methods we access from ARCCache, to enable alternate
// implementations such as a no-op one.
type cache interface {
	Get(key interface{}) (value interface{}, ok bool)
	Add(key, value interface{})
	Remove(key interface{})
	Contains(key interface{}) bool
	Peek(key interface{}) (value interface{}, ok bool)
}
