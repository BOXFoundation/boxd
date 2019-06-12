// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package rpcutil

import (
	"context"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/BOXFoundation/boxd/core/abi"
	pb "github.com/BOXFoundation/boxd/rpc/pb"
	lru "github.com/hashicorp/golang-lru"
	"google.golang.org/grpc"
)

var abiCache, _ = lru.New(512)

// ImportAbi caches abi.
func ImportAbi(addr, abistr string) error {
	if len(addr) == 0 {
		return fmt.Errorf("Address is nil")
	}
	if len(abistr) == 0 {
		return fmt.Errorf("Abi is nil")
	}
	abi, err := abi.JSON(strings.NewReader(abistr))
	if err != nil {
		return err
	}

	abiCache.Add(addr, abi)
	return nil
}

// DoCall executes a message call transaction, which is directly executed in the VM
// of the node, but never mined into the blockchain.
//
// blockNumber selects the block height at which the call runs. It can be nil, in which
// case the code is taken from the latest known block. Note that state from very old
// blocks might not be available.
func DoCall(conn *grpc.ClientConn, from, to, data string, height, timeout uint32) (interface{}, error) {

	cabi, ok := abiCache.Get(to)
	if !ok {
		return nil, fmt.Errorf("Abi for %s not found", to)
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
	method, err := cabi.(abi.ABI).MethodByID(dd)
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
