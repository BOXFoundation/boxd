// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package client

import (
	"fmt"

	"github.com/BOXFoundation/boxd/config"
	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/log"
	"github.com/BOXFoundation/boxd/script"
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

func getScriptAddressFromPubKeyHash(pubKeyHash []byte) ([]byte, error) {
	addr, err := types.NewAddressPubKeyHash(pubKeyHash)
	if err != nil {
		return nil, err
	}
	return *script.PayToPubKeyHashScript(addr.ScriptAddress()), nil
}

func getScriptAddress(address types.Address) []byte {
	return *script.PayToPubKeyHashScript(address.ScriptAddress())
}

// NewConnectionWithViper initializes a grpc connection using configs parsed by viper
func NewConnectionWithViper(v *viper.Viper) *grpc.ClientConn {
	return mustConnect(v)
}

// NewConnectionWithHostPort initializes a grpc connection using host and port params
func NewConnectionWithHostPort(host string, port int) *grpc.ClientConn {
	conn, err := grpc.Dial(fmt.Sprintf("%s:%d", host, port), grpc.WithInsecure())
	if err != nil {
		panic("Fail to establish grpc connection")
	}
	return conn
}
