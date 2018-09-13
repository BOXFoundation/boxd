// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package ctl

import (
	"fmt"

	"github.com/spf13/cobra"
)

// getblockhashCmd represents the getblockhash command
var getblockhashCmd = &cobra.Command{
	Use:   "getblockhash [index]",
	Short: "Get the hash of a block at a given index",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("getblockhash called")
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
