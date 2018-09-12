// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// getwalletinfoCmd represents the getwalletinfo command
var getwalletinfoCmd = &cobra.Command{
	Use:   "getwalletinfo",
	Short: "Get the basic informatio for a wallet",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("getwalletinfo called")
	},
}

func init() {
	rootCmd.AddCommand(getwalletinfoCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// getwalletinfoCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// getwalletinfoCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
