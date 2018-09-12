// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// addnodeCmd represents the addnode command
var addnodeCmd = &cobra.Command{
	Use:   "addnode [netaddr] add|remove",
	Short: "Add or remove a peer node",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("addnode called")
	},
}

func init() {
	rootCmd.AddCommand(addnodeCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// addnodeCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// addnodeCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
