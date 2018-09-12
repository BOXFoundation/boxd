// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// dumprivkeyCmd represents the dumprivkey command
var dumpprivkeyCmd = &cobra.Command{
	Use:   "dumpprivkey [address]",
	Short: "Dump private key for an address",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("dumprivkey called")
	},
}

func init() {
	rootCmd.AddCommand(dumpprivkeyCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// dumprivkeyCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// dumprivkeyCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
