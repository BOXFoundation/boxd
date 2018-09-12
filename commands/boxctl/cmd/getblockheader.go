// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// getblockheaderCmd represents the getblockheader command
var getblockheaderCmd = &cobra.Command{
	Use:   "getblockheader [hash]",
	Short: "Get the block header for a hash",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("getblockheader called")
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
