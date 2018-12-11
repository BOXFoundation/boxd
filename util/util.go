// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package util

import (
	"bytes"
	"os"
	"os/user"
	"reflect"
	"runtime"

	"github.com/BOXFoundation/boxd/log"
)

var logger = log.NewLogger("util") // logger

// InArray return if there is an element in the array
func InArray(obj interface{}, array interface{}) bool {
	arrayValue := reflect.ValueOf(array)
	if reflect.TypeOf(array).Kind() == reflect.Array || reflect.TypeOf(array).Kind() == reflect.Slice {
		for i := 0; i < arrayValue.Len(); i++ {
			if reflect.DeepEqual(arrayValue.Index(i).Interface(), obj) {
				return true
			}
		}
	}
	return false
}

// HomeDir returns home directory of current user
func HomeDir() string {
	if runtime.GOOS == "windows" {
		home := os.Getenv("HOMEDRIVE") + os.Getenv("HOMEPATH")
		if home == "" {
			home = os.Getenv("USERPROFILE")
		}
		return home
	}
	if home := os.Getenv("HOME"); home != "" {
		return home
	}
	if usr, err := user.Current(); err == nil {
		return usr.HomeDir
	}
	return ""
}

// IsPrefixed returns if s has the passed prefix
func IsPrefixed(s, prefix []byte) bool {
	prefixLen := len(prefix)
	if len(s) < prefixLen {
		return false
	}
	s = s[:prefixLen]
	return bytes.Equal(s, prefix)
}
