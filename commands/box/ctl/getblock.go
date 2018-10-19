// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package ctl

import (
	"fmt"

	rpc "github.com/BOXFoundation/boxd/rpc/client"
	"github.com/BOXFoundation/boxd/util"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// getblockCmd represents the getblock command
var getblockCmd = &cobra.Command{
	Use:   "getblock [hash]",
	Short: "Get the block with a specific hash",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("getblock called")
		if len(args) == 0 {
			fmt.Println("Parameter block hash required")
			return
		}
		hash := args[0]
		block, err := rpc.GetBlock(viper.GetViper(), hash)
		if err != nil {
			fmt.Println(err)
		} else {
			fmt.Printf("Block info of hash %s is\n%s\n", hash, util.PrettyPrint(block))
		}
	},
}

func init() {
	rootCmd.AddCommand(getblockCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// getblockCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// getblockCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
