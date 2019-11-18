// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package util

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/user"
	"path"
	"reflect"
	"runtime"
	"strings"

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

// InBytes return if there is an element in the array
func InBytes(o []byte, bytesArray [][]byte) bool {
	for _, b := range bytesArray {
		if bytes.Equal(o, b) {
			return true
		}
	}
	return false
}

// InStrings return if there is an element in the array
func InStrings(o string, strings []string) bool {
	for _, s := range strings {
		if o == s {
			return true
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

// MkDir make a directory with name filename
func MkDir(filename string) error {
	if _, err := os.Stat(filename); err == nil {
		return nil
	} else if os.IsNotExist(err) {
		return os.MkdirAll(filename, os.ModePerm)
	} else {
		return err
	}
}

//FileExists check whether a file exists
func FileExists(filename string) error {
	if _, err := os.Stat(filename); err != nil {
		if os.IsNotExist(err) {
			return errors.New("file is not exists")
		}
		return fmt.Errorf("file status error: %s", err)
	}
	return nil
}

//GetFileName get file name
func GetFileName(filePath string) string {
	file := path.Base(filePath)
	fileSuffix := path.Ext(file)
	fileName := strings.TrimSuffix(file, fileSuffix)
	return fileName
}
