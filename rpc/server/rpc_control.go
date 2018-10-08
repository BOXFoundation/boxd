// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package rpc

import (
	"context"
	"fmt"

	"github.com/BOXFoundation/Quicksilver/rpc/pb"
)

func registerControl(s *Server) {
	rpcpb.RegisterContorlCommandServer(s.server, &ctlserver{server: s})
}

func init() {
	RegisterServiceWithGatewayHandler(
		"control",
		registerControl,
		rpcpb.RegisterContorlCommandHandlerFromEndpoint,
	)
}

type ctlserver struct {
	server GRPCServer
}

// SetDebugLevel implements SetDebugLevel
func (s *ctlserver) SetDebugLevel(ctx context.Context, in *rpcpb.DebugLevelRequest) (*rpcpb.BaseResponse, error) {
	logger.SetLogLevel(in.Level)
	log := s.server.BoxdServer().Cfg().GetLog()
	log.Level = logger.LogLevel()
	if in.Level != logger.LogLevel() {
		var info = fmt.Sprintf("Wrong debug level: %s", in.Level)
		logger.Info(info)
		return &rpcpb.BaseResponse{Code: 1, Message: info}, nil
	}
	var info = fmt.Sprintf("Set debug level: %s", logger.LogLevel())
	logger.Infof(info)
	return &rpcpb.BaseResponse{Code: 0, Message: info}, nil
}
