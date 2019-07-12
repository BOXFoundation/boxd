// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package rpcutil

import (
	"context"
	"encoding/hex"
	"time"

	"github.com/BOXFoundation/boxd/core/abi"
	pb "github.com/BOXFoundation/boxd/rpc/pb"
	"google.golang.org/grpc"
)

// DoCall executes a message call transaction, which is directly executed in the VM
// of the node, but never mined into the blockchain.
//
// blockNumber selects the block height at which the call runs. It can be nil, in which
// case the code is taken from the latest known block. Note that state from very old
// blocks might not be available.
func DoCall(conn *grpc.ClientConn, from, to, data string, height, timeout uint32, abidata []byte) (interface{}, error) {

	aabi := abi.ABI{}
	err := aabi.UnmarshalJSON(abidata)
	if err != nil {
		return nil, err
	}

	c := pb.NewWebApiClient(conn)

	var ctx context.Context
	var cancel context.CancelFunc
	if timeout != 0 {
		ctx, cancel = context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	} else {
		ctx, cancel = context.WithCancel(context.Background())
	}
	defer cancel()

	// Get method from abi.
	dd, err := hex.DecodeString(data)
	if err != nil {
		return nil, err
	}
	method, err := aabi.MethodByID(dd)
	if err != nil {
		return nil, err
	}

	// Excute evm.
	r, err := c.DoCall(ctx, &pb.CallReq{From: from, To: to, Data: data, Height: height, Timeout: timeout})
	if err != nil {
		return nil, err
	}

	if len(method.Outputs) == 0 {
		return nil, nil
	}
	// Decode outputs.
	rr, err := hex.DecodeString(r.Output)
	if err != nil {
		return nil, err
	}
	res, err := method.Outputs.UnpackValues(rr)
	if err != nil {
		return nil, err
	}

	// outputs := method.Outputs
	// if len(outputs) == 1 {
	// 	v := reflect.New(outputs[0].Type.Type).Elem().Interface()
	// 	err = cabi.(abi.ABI).Unpack(&v, method.Name, rr)
	// 	if err != nil {
	// 		return r, err
	// 	}
	// 	return v, nil
	// } else if len(outputs) > 1 {
	// 	v := make([]interface{}, len(outputs))
	// 	err = cabi.(abi.ABI).Unpack(&v, method.Name, rr)
	// 	if err != nil {
	// 		return r, err
	// 	}
	// 	return v, nil
	// }

	if len(method.Outputs) == 1 {
		return res[0], nil
	}
	return res, nil
}
