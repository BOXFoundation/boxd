// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// getblockCmd represents the getblock command
var getblockCmd = &cobra.Command{
	Use:   "getblock [hash]",
	Short: "Get the block with a specific hash",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("getblock called")
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
