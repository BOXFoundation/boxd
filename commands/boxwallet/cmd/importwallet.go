// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// importwalletCmd represents the importwallet command
var importwalletCmd = &cobra.Command{
	Use:   "importwallet [filename]",
	Short: "Import a wallet from a file",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("importwallet called")
	},
}

func init() {
	rootCmd.AddCommand(importwalletCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// importwalletCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// importwalletCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
