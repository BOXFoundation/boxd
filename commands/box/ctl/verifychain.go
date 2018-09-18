// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package ctl

import (
	"fmt"

	"github.com/spf13/cobra"
)

// verifychainCmd represents the verifychain command
var verifychainCmd = &cobra.Command{
	Use:   "verifychain",
	Short: "Verify the local chain",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("verifychain called")
	},
}

func init() {
	rootCmd.AddCommand(verifychainCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// verifychainCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// verifychainCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
