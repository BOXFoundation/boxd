// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package client

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/spf13/viper"
	"google.golang.org/grpc"

	config "github.com/BOXFoundation/Quicksilver/config"
	pb "github.com/BOXFoundation/Quicksilver/rpc/pb"
)

func unmarshalConfig(v *viper.Viper) *config.Config {
	var cfg config.Config
	viper.Unmarshal(&cfg)
	return &cfg
}

// SetDebugLevel calls the DebugLevel gRPC methods.
func SetDebugLevel(v *viper.Viper, level string) error {
	var cfg = unmarshalConfig(v)

	conn, err := grpc.Dial(fmt.Sprintf("%s:%d", cfg.RPC.Address, cfg.RPC.Port), grpc.WithInsecure())
	if err != nil {
		return err
	}
	defer conn.Close()

	c := pb.NewContorlCommandClient(conn)

	// Contact the server and print out its response.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	log.Printf("Set debug level %s", level)
	r, err := c.SetDebugLevel(ctx, &pb.DebugLevelRequest{Level: level})
	if err != nil {
		return err
	}
	log.Printf("Result: %d, Message: %s", r.Code, r.Message)
	return nil
}
