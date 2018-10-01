// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package client

import (
	"context"
	"time"

	"github.com/spf13/viper"

	pb "github.com/BOXFoundation/Quicksilver/rpc/pb"
)

// SetDebugLevel calls the DebugLevel gRPC methods.
func SetDebugLevel(v *viper.Viper, level string) error {
	conn := mustConnect(v)
	defer conn.Close()

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
