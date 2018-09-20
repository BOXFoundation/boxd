// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package node

import (
	"context"
	"fmt"

	ctlpb "github.com/BOXFoundation/Quicksilver/rpc/pb"
	rpcserver "github.com/BOXFoundation/Quicksilver/rpc/server"
	"google.golang.org/grpc"
)

func registerControl(s *grpc.Server) {
	ctlpb.RegisterContorlCommandServer(s, &ctlserver{})
}

func init() {
	rpcserver.RegisterService("control", registerControl)
}

type ctlserver struct{}

// SetDebugLevel implements SetDebugLevel
func (s *ctlserver) SetDebugLevel(ctx context.Context, in *ctlpb.DebugLevelRequest) (*ctlpb.Reply, error) {
	logger.SetLogLevel(in.Level)
	nodeServer.cfg.Log.Level = logger.LogLevel()
	if in.Level != logger.LogLevel() {
		var info = fmt.Sprintf("Wrong debug level: %s", in.Level)
		logger.Info(info)
		return &ctlpb.Reply{Code: 1, Message: info}, nil
	}
	var info = fmt.Sprintf("Set debug level %s", logger.LogLevel())
	logger.Infof(info)
	return &ctlpb.Reply{Code: 0, Message: info}, nil
}
