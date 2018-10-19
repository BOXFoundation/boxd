// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package ctl

import (
	"fmt"

	"github.com/spf13/viper"

	rpc "github.com/BOXFoundation/boxd/rpc/client"
	"github.com/BOXFoundation/boxd/util"
	"github.com/spf13/cobra"
)

// getblockheaderCmd represents the getblockheader command
var getblockheaderCmd = &cobra.Command{
	Use:   "getblockheader [hash]",
	Short: "Get the block header for a hash",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("getblockheader called")
		if len(args) == 0 {
			fmt.Println("Parameter block hash required")
			return
		}
		hash := args[0]
		header, err := rpc.GetBlockHeader(viper.GetViper(), hash)
		if err != nil {
			fmt.Println(err)
		} else {
			fmt.Printf("Block Header of hash %s is\n%s\n", hash, util.PrettyPrint(header))
		}
	},
}

func init() {
	rootCmd.AddCommand(getblockheaderCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// getblockheaderCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// getblockheaderCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
