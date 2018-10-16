// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package types

import (
	"github.com/BOXFoundation/boxd/core/chain"
	"github.com/BOXFoundation/boxd/core/txpool"
	logtypes "github.com/BOXFoundation/boxd/log/types"
	"github.com/spf13/viper"
)

// BoxdServer Define boxd server interface
type BoxdServer interface {
	Start(v *viper.Viper) error
	Cfg() Config
	BlockChain() *chain.BlockChain
	TxPool() *txpool.TransactionPool
}

// Config Define config interface
type Config interface {
	String() string
	GetLog() logtypes.Config
}
