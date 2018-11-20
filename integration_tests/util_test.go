// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package main

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"reflect"
	"testing"
)

func TestGetIniKV(t *testing.T) {
	iniBuf := `spawn ./box --config .devconfig/.box-1.yaml wallet newaccount
Please Input Your Passphrase
Created new account: 49a7d13735b2614a65cabf4f4058783e4cecf13a
Address:b1aXG2SsUzVKePYsZxkUjHoDSYsKmyCA7wk`
	if "b1aXG2SsUzVKePYsZxkUjHoDSYsKmyCA7wk" != GetIniKV(iniBuf, "Address") {
		t.Fatalf("want: %s, got: %s", "b1aXG2SsUzVKePYsZxkUjHoDSYsKmyCA7wk",
			GetIniKV(iniBuf, "Address"))
	}
}

func TestParseIPList(t *testing.T) {
	var testIPList = []string{
		"127.0.0.1:19111",
		"127.0.0.1:19121",
		"127.0.0.1:19131",
		"127.0.0.1:19141",
		"127.0.0.1:19151",
		"127.0.0.1:19161",
	}
	bytes, _ := json.Marshal(testIPList)
	tmpFile := "./tmp123.tmp"
	if err := ioutil.WriteFile(tmpFile, bytes, 0600); err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile)
	list, err := parseIPlist(tmpFile)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(list, testIPList) {
		t.Fatalf("want: %+v, got: %+v", testIPList, list)
	}
}
