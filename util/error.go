// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package util

import "errors"

// error
var (
	ErrKeyNotFound  = errors.New("key not found in dag")
	ErrKeyIsExisted = errors.New("key is exist in dag")
)
