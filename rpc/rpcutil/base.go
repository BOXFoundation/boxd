// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package rpcutil

import (
	"fmt"

	"github.com/BOXFoundation/boxd/config"
	"github.com/BOXFoundation/boxd/log"
	"github.com/spf13/viper"
	"google.golang.org/grpc"
)

var logger = log.NewLogger("rpcclient") // logger for client package

func calcFee() uint64 {
	return 100
}

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

// NewConnectionWithViper initializes a grpc connection using configs parsed by viper
func NewConnectionWithViper(v *viper.Viper) *grpc.ClientConn {
	return mustConnect(v)
}
