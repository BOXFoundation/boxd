// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// sendmanyCmd represents the sendmany command
var sendmanyCmd = &cobra.Command{
	Use:   "sendmany [fromaccount] [toaddresslist]",
	Short: "Send coins to multiple addresses",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("sendmany called")
	},
}

func init() {
	rootCmd.AddCommand(sendmanyCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// sendmanyCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// sendmanyCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
