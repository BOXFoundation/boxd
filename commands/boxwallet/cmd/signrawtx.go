// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// signrawtxCmd represents the signrawtx command
var signrawtxCmd = &cobra.Command{
	Use:   "signrawtx [rawtx]",
	Short: "Sign a transaction with privatekey and send it to the network",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("signrawtx called")
	},
}

func init() {
	rootCmd.AddCommand(signrawtxCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// signrawtxCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// signrawtxCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
