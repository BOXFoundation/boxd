// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package wallet

import (
	"fmt"

	"github.com/spf13/cobra"
)

// listreceivedbyaccountCmd represents the listreceivedbyaccount command
var listreceivedbyaccountCmd = &cobra.Command{
	Use:   "listreceivedbyaccount",
	Short: "List received transactions groups by account",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("listreceivedbyaccount called")
	},
}

func init() {
	rootCmd.AddCommand(listreceivedbyaccountCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// listreceivedbyaccountCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// listreceivedbyaccountCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
