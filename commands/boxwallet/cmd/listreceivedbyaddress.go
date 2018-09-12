// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// listreceivedbyaddressCmd represents the listreceivedbyaddress command
var listreceivedbyaddressCmd = &cobra.Command{
	Use:   "listreceivedbyaddress",
	Short: "List received transactions grouped by address",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("listreceivedbyaddress called")
	},
}

func init() {
	rootCmd.AddCommand(listreceivedbyaddressCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// listreceivedbyaddressCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// listreceivedbyaddressCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
