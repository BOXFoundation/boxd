// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// encryptwalletCmd represents the encryptwallet command
var encryptwalletCmd = &cobra.Command{
	Use:   "encryptwallet [passphrase]",
	Short: "Encrypt a wallet with a passphrase",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("encryptwallet called")
	},
}

func init() {
	rootCmd.AddCommand(encryptwalletCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// encryptwalletCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// encryptwalletCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
