// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package rpc

import (
	"context"

	"github.com/BOXFoundation/boxd/boxd/eventbus"
	"github.com/BOXFoundation/boxd/rpc/pb"
)

func registerDatabase(s *Server) {
	rpcpb.RegisterDatabaseCommandServer(s.server, &dbserver{server: s})
}

func init() {
	RegisterServiceWithGatewayHandler(
		"database",
		registerDatabase,
		rpcpb.RegisterDatabaseCommandHandlerFromEndpoint,
	)
}

type dbserver struct {
	server GRPCServer
}

// get all keys of database
func (svr *dbserver) GetDatabaseKeys(ctx context.Context, in *rpcpb.GetDatabaseKeysRequest) (*rpcpb.GetDatabaseKeysResponse, error) {
	if in.Skip < 0 {
		in.Skip = 0
	}
	if in.Limit <= 0 {
		in.Limit = 20
	}
	var out = make(chan []byte)
	svr.server.GetEventBus().Send(eventbus.TopicGetDatabaseKeys, ctx, in.Table, in.Prefix, in.Skip, in.Limit, out)

	var keys []string
Loop:
	for {
		select {
		case k := <-out:
			if k != nil {
				keys = append(keys, string(k))
			} else {
				break Loop
			}
		case <-ctx.Done():
			break Loop
		}
	}
	return &rpcpb.GetDatabaseKeysResponse{Code: 0, Message: "ok", Skip: in.Skip, Keys: keys}, nil
}

// get value of associate with passed key in database
func (svr *dbserver) GetDatabaseValue(ctx context.Context, in *rpcpb.GetDatabaseValueRequest) (*rpcpb.GetDatabaseValueResponse, error) {
	var out = make(chan []byte)
	svr.server.GetEventBus().Send(eventbus.TopicGetDatabaseValue, in.Table, in.Key, out)
	select {
	case v := <-out:
		return &rpcpb.GetDatabaseValueResponse{
			Code:    0,
			Message: "ok",
			Value:   v,
		}, nil
	case <-ctx.Done():
		return &rpcpb.GetDatabaseValueResponse{
			Code:    1,
			Message: "Timeout",
		}, nil
	}
}
