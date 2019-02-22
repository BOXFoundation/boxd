// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package rpc

import (
	"net/http"
	"net/url"
	"testing"
)

func TestFaucetClaim(t *testing.T) {
	resp, err := http.PostForm("http://example.com/form",
		url.Values{"addr": {"Value"}, "amount": {"123000"}})
}
