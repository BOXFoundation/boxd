// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// sendfromCmd represents the sendfrom command
var sendfromCmd = &cobra.Command{
	Use:   "sendfrom [fromaccount] [toaddress] [amount]",
	Short: "Send coins from an account to an address",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("sendfrom called")
	},
}

func init() {
	rootCmd.AddCommand(sendfromCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// sendfromCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// sendfromCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
