// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package ctl

import (
	"fmt"
	"strconv"

	rpc "github.com/BOXFoundation/boxd/rpc/client"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// getblockhashCmd represents the getblockhash command
var getblockhashCmd = &cobra.Command{
	Use:   "getblockhash [height]",
	Short: "Get the hash of a block at a given index",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("getblockhash called")
		if len(args) == 0 {
			fmt.Println("Parameter block height required")
			return
		}
		height64, err := strconv.ParseUint(args[0], 10, 32)
		if err != nil {
			fmt.Println(err)
			return
		}
		height := uint32(height64)
		hash, err := rpc.GetBlockHash(viper.GetViper(), height)
		if err != nil {
			fmt.Println(err)
		} else {
			fmt.Printf("Block hash of height %d is %s\n", height, hash)
		}
	},
}

func init() {
	rootCmd.AddCommand(getblockhashCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// getblockhashCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// getblockhashCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
