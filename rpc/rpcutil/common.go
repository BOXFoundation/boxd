// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package rpcutil

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/BOXFoundation/boxd/log"
	"google.golang.org/grpc"
)

var logger = log.NewLogger("rpcclient") // logger for client package

// RPCCall calls a rpc api
func RPCCall(
	clientFn interface{}, methodName string, req interface{}, peer string,
) (resp interface{}, err error) {
	conn, err := grpc.Dial(peer, grpc.WithInsecure())
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if !(reflect.TypeOf(clientFn).Kind() == reflect.Func) {
		return nil, fmt.Errorf("%s is not of type reflect.Func", reflect.TypeOf(clientFn).Kind())
	}

	results := reflect.ValueOf(clientFn).Call([]reflect.Value{reflect.ValueOf(conn)})

	args := []reflect.Value{reflect.ValueOf(ctx), reflect.ValueOf(req)}
	results = results[0].MethodByName(methodName).Call(args)

	err, _ = results[1].Interface().(error)
	return results[0].Interface(), err
}
