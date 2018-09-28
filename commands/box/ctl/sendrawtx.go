// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package ctl

import (
	"fmt"

	"github.com/spf13/cobra"
)

// sendrawtxCmd represents the sendrawtx command
var sendrawtxCmd = &cobra.Command{
	Use:   "sendrawtx [rawtx]",
	Short: "Send a raw transaction to the network",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("sendrawtx called")
	},
}

func init() {
	rootCmd.AddCommand(sendrawtxCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// sendrawtxCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// sendrawtxCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
