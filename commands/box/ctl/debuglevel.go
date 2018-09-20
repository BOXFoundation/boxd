// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package ctl

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"google.golang.org/grpc"

	config "github.com/BOXFoundation/Quicksilver/config"
	pb "github.com/BOXFoundation/Quicksilver/rpc/pb"
)

// debuglevelCmd represents the debuglevel command
var debuglevelCmd = &cobra.Command{
	Use:   "debuglevel [debug|info|warning|error|fatal]",
	Short: "Set the debug level of boxd",
	RunE: func(cmd *cobra.Command, args []string) error {
		var cfg config.Config
		viper.Unmarshal(&cfg)

		conn, err := grpc.Dial(fmt.Sprintf("%s:%d", cfg.RPC.Address, cfg.RPC.Port), grpc.WithInsecure())
		if err != nil {
			return err
		}
		defer conn.Close()

		c := pb.NewContorlCommandClient(conn)

		// Contact the server and print out its response.
		level := "info"
		if len(args) > 0 {
			level = args[0]
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		log.Printf("Set debug level %s", level)
		r, err := c.SetDebugLevel(ctx, &pb.DebugLevelRequest{Level: level})
		if err != nil {
			return err
		}
		log.Printf("Result: %d, Message: %s", r.Code, r.Message)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(debuglevelCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// debuglevelCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// debuglevelCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
