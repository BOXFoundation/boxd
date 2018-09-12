// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// searchrawtxCmd represents the searchrawtx command
var searchrawtxCmd = &cobra.Command{
	Use:   "searchrawtxs [address]",
	Short: "Search transactions for a given address",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("searchrawtx called")
	},
}

func init() {
	rootCmd.AddCommand(searchrawtxCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// searchrawtxCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// searchrawtxCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
