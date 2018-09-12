// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// sendtoaddressCmd represents the sendtoaddress command
var sendtoaddressCmd = &cobra.Command{
	Use:   "sendtoaddress [address]",
	Short: "Send coins to an address",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("sendtoaddress called")
	},
}

func init() {
	rootCmd.AddCommand(sendtoaddressCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// sendtoaddressCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// sendtoaddressCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
