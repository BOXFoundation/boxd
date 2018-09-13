// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package ctl

import (
	"fmt"

	"github.com/spf13/cobra"
)

// getrawtxCmd represents the getrawtx command
var getrawtxCmd = &cobra.Command{
	Use:   "getrawtx [txid]",
	Short: "Get the raw transaction for a txid",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("getrawtx called")
	},
}

func init() {
	rootCmd.AddCommand(getrawtxCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// getrawtxCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// getrawtxCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
