// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// getblockcountCmd represents the getblockcount command
var getblockcountCmd = &cobra.Command{
	Use:   "getblockcount",
	Short: "Get the total block count",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("getblockcount called")
	},
}

func init() {
	rootCmd.AddCommand(getblockcountCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// getblockcountCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// getblockcountCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
