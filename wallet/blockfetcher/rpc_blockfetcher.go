// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package blockfetcher

import (
	"fmt"
	"sync"

	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/rpc/client"
	"github.com/BOXFoundation/boxd/wallet/walletdata"
	"google.golang.org/grpc"
)

type rpcBlockFetcher struct {
	conn *grpc.ClientConn
	mux  *sync.Mutex
}

// Height returns current block height of rpc target server
func (r *rpcBlockFetcher) Height() (uint32, error) {
	r.mux.Lock()
	defer r.mux.Unlock()
	return client.GetBlockCount(r.conn)
}

// HashForHeight returns hash string for the height param
func (r *rpcBlockFetcher) HashForHeight(height uint32) (string, error) {
	r.mux.Lock()
	defer r.mux.Unlock()
	return client.GetBlockHash(r.conn, height)
}

// Block returns block info of hash param
func (r *rpcBlockFetcher) Block(hashStr string) (*types.Block, error) {
	r.mux.Lock()
	defer r.mux.Unlock()
	return client.GetBlock(r.conn, hashStr)
}

var _ walletdata.BlockFetcher = (*rpcBlockFetcher)(nil)

// NewRPCBlockFetcherWithHostPort returns an block fetch using rpc client
func NewRPCBlockFetcherWithHostPort(host string, port int) (walletdata.BlockFetcher, error) {
	conn, err := grpc.Dial(fmt.Sprintf("%s:%d", host, port), grpc.WithInsecure())
	if err != nil {
		return nil, err
	}

	return &rpcBlockFetcher{
		conn: conn,
		mux:  &sync.Mutex{},
	}, nil
}
