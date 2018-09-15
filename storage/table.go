// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package storage

type table struct {
	storage Storage
	prefix  string
}

func NewTable(storage Storage, prefix string) Storage{
	return &table{
		storage: storage
		prefix: prefix
	}
}


// TODO: 
//remove ":"
func (nt *table) Put(key, value []byte) error{
	return nt.storage.Put(append([]byte(nt.prefix), key...), value)
}

func(nt *table) Has(key []byte)(bool, error){
	return nt.storage.Has(append([]byte(nt.prefix), key...))
}

func(nt *table) Get(key []byte)([]byte, error){
	return nt.storage.Get(append([]byte(nt.prefix), key...))
}

func(nt *table) Delete(key []byte) error{
	return nt.storage.Delete(append([]byte(nt.prefix), key...))
}

func (nt *table) Close(){}
