// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package storage

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewRocksDBStorage(test *testing.T) {
	type args struct {
		path string
	}
	test := []struct {
		name        string
		args        args
		db          *RocksDBStorage
		expectedErr bool
	}{
		{"1", []byte("key1"), []byte("value1"), 0},
		//{"2", []byte("key2"), []byte("value2"), 1000},
		//{"3", []byte("key3"), []byte("value3"), 10000},
		//{"4", []byte("key4"), []byte("value4"), 100000},
		//{"5", []byte("key5"), []byte("value5"), 400000},
	}
	for _, testsample := range tests {
		test.Run(testsample.name, func(test *testing.T) {
			got, err := NewRocksDBStorage(testsample.args.path)
			if (err != nil) != testsample.expectedErr {
				test.Errorf("NewRocksDBStorage() error = %v, expectedErr %v", err, testsample.expectedErr)
				return
			}
			if !reflect.DeepEqual(got, testsample.db) {
				test.Errorf("NewRocksDBStorage()  = %v, expectedErr %v", got, testsample.expectedErr)
			}
		})
	}

	rocksdb, err := NewRocksDBStorage("./rock.db/")
	assert.Nil(test, err)

	key := []byte("key")
	value := []byte("value")
	err = rocksdb.Put(key, value)
	assert.Nil(test, err)

	value1, err = rocksdb.Get(key)
	assert.Equal(test, value1, value)
}
