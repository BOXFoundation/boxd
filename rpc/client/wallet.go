// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package client

import (
	"context"
	"log"
	"time"

	"github.com/BOXFoundation/Quicksilver/rpc/pb"

	"github.com/spf13/viper"
)

// ListTransactions list transactions of certain address
func ListTransactions(v *viper.Viper, addr string) error {
	conn := mustConnect(v)
	defer conn.Close()
	c := rpcpb.NewWalletCommandClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	log.Printf("List Transactions of address: %s", addr)

	r, err := c.ListTransactions(ctx, &rpcpb.ListTransactionsRequest{Addr: addr, Offset: 0, Limit: 20})
	if err != nil {
		return err
	}
	log.Printf("Result: %+v", r)
	return nil
}
