// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package ctl

import (
	"fmt"

	"github.com/spf13/cobra"
)

// createrawtxCmd represents the createrawtx command
var createrawtxCmd = &cobra.Command{
	Use:   "createrawtx",
	Short: "Create a raw transaction",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("createrawtx called")
	},
}

func createRawTransaction(fromPubKey []byte, toPubKey []byte) error {
	res, err = 
}

func init() {
	rootCmd.AddCommand(createrawtxCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// createrawtxCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// createrawtxCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
