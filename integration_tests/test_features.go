// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package main

import (
	"github.com/BOXFoundation/boxd/integration_tests/utils"
	"github.com/BOXFoundation/boxd/wallet"
)

// TxSortTest tests whether orphan tx in tx pool in the case that txs sent to
// blockchain in the same time
func TxSortTest() {
	count := 2
	addrs, _ := utils.GenTestAddr(count)
	accs := make([]*wallet.Account, 2)
	for _, addr := range addrs {
		accs = append(accs, utils.UnlockAccount(addr))
	}
}
