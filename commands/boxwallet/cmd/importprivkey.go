// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// importprivkeyCmd represents the importprivkey command
var importprivkeyCmd = &cobra.Command{
	Use:   "importprivkey [privatekey]",
	Short: "Import a private key from other wallets",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("importprivkey called")
	},
}

func init() {
	rootCmd.AddCommand(importprivkeyCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// importprivkeyCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// importprivkeyCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
