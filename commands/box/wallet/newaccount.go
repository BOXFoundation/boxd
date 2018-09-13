// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package wallet

import (
	"fmt"

	"github.com/spf13/cobra"
)

// newaccountCmd represents the newaccount command
var newaccountCmd = &cobra.Command{
	Use:   "newaccount [account]",
	Short: "Create a new account",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("newaccount called")
	},
}

func init() {
	rootCmd.AddCommand(newaccountCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// newaccountCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// newaccountCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
