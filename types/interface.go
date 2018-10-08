// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package types

import (
	"github.com/BOXFoundation/Quicksilver/core"
	logtypes "github.com/BOXFoundation/Quicksilver/log/types"
	"github.com/spf13/viper"
)

// BoxdServer Define boxd server interface
type BoxdServer interface {
	Start(v *viper.Viper) error
	Cfg() Config
	BlockChain() *core.BlockChain
}

// Config Define config interface
type Config interface {
	Prepare()
	GetLog() logtypes.Config
}
