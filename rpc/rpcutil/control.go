// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package rpcutil

import (
	"context"
	"time"

	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/log"
	pb "github.com/BOXFoundation/boxd/rpc/pb"
	"google.golang.org/grpc"
)

var logger = log.NewLogger("rpcclient") // logger for client package

// SetDebugLevel calls the DebugLevel gRPC methods.
func SetDebugLevel(conn *grpc.ClientConn, level string) error {

	c := pb.NewContorlCommandClient(conn)

	// Contact the server and print out its response.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	logger.Infof("Set debug level %s", level)
	r, err := c.SetDebugLevel(ctx, &pb.DebugLevelRequest{Level: level})
	if err != nil {
		return err
	}
	logger.Infof("Result: %d, Message: %s", r.Code, r.Message)

	return nil
}

// UpdateNetworkID calls the UpdateNetworkID gRPC methods.
func UpdateNetworkID(conn *grpc.ClientConn, id uint32) error {

	c := pb.NewContorlCommandClient(conn)

	// Contact the server and print out its response.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	logger.Infof("update network id %d", id)
	r, err := c.UpdateNetworkID(ctx, &pb.UpdateNetworkIDRequest{Id: id})
	if err != nil {
		return err
	}
	logger.Infof("Result: %d, Message: %s", r.Code, r.Message)

	return nil
}

// GetBlockCount query chain height
func GetBlockCount(conn *grpc.ClientConn) (uint32, error) {
	c := pb.NewContorlCommandClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	logger.Info("Querying block count")
	r, err := c.GetBlockHeight(ctx, &pb.GetBlockHeightRequest{})
	if err != nil {
		return 0, err
	}
	logger.Infof("Block info: %+v", r)
	return r.Height, nil
}

// GetBlockHash returns block hash of a height
func GetBlockHash(conn *grpc.ClientConn, height uint32) (string, error) {
	c := pb.NewContorlCommandClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	logger.Infof("Query block hash of height: %d", height)
	r, err := c.GetBlockHash(ctx, &pb.GetBlockHashRequest{Height: height})
	if err != nil {
		return "", err
	}
	return r.Hash, nil
}

// GetBlockHeader returns header info of a block
func GetBlockHeader(conn *grpc.ClientConn, hash string) (*types.BlockHeader, error) {
	c := pb.NewContorlCommandClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	logger.Infof("Query block header of a hash: %s", hash)
	r, err := c.GetBlockHeader(ctx, &pb.GetBlockRequest{BlockHash: hash})
	if err != nil {
		return nil, err
	}
	header := &types.BlockHeader{}
	err = header.FromProtoMessage(r.Header)
	return header, err
}

// GetBlock returns block info of a block hash
func GetBlock(conn *grpc.ClientConn, hash string) (*types.Block, error) {
	c := pb.NewContorlCommandClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	logger.Infof("Query block info of a hash :%s", hash)
	r, err := c.GetBlock(ctx, &pb.GetBlockRequest{BlockHash: hash})
	if err != nil {
		return nil, err
	}

	block := &types.Block{}
	err = block.FromProtoMessage(r.Block)
	return block, err
}
