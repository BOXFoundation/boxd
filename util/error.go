package util

import "errors"

// error
var (
	ErrKeyNotFound  = errors.New("key not found in dag")
	ErrKeyIsExisted = errors.New("key is exist in dag")
)
