// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package client

import (
	"fmt"

	"github.com/BOXFoundation/boxd/config"
	"github.com/BOXFoundation/boxd/core"
	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/log"
	"github.com/spf13/viper"
	"google.golang.org/grpc"
)

var logger = log.NewLogger("rpcclient") // logger for client package

func unmarshalConfig(v *viper.Viper) *config.Config {
	var cfg config.Config
	viper.Unmarshal(&cfg)
	return &cfg
}

func mustConnect(v *viper.Viper) *grpc.ClientConn {
	var cfg = unmarshalConfig(v)
	conn, err := grpc.Dial(fmt.Sprintf("%s:%d", cfg.RPC.Address, cfg.RPC.Port), grpc.WithInsecure())
	if err != nil {
		panic("Fail to establish grpc connection")
	}
	return conn
}

func getScriptAddress(pubKeyHash []byte) ([]byte, error) {
	addr, err := types.NewAddressPubKeyHash(pubKeyHash, 0x00)
	if err != nil {
		return nil, err
	}
	return core.PayToPubKeyHashScript(addr.ScriptAddress())
}
