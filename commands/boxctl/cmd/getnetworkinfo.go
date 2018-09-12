// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// getnetworkinfoCmd represents the getnetworkinfo command
var getnetworkinfoCmd = &cobra.Command{
	Use:   "getnetworkinfo [network]",
	Short: "Get the basic info and performance metrics of a network",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("getnetworkinfo called")
	},
}

func init() {
	rootCmd.AddCommand(getnetworkinfoCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// getnetworkinfoCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// getnetworkinfoCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
