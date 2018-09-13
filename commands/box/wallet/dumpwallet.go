// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package wallet

import (
	"fmt"

	"github.com/spf13/cobra"
)

// dumpwalletCmd represents the dumpwallet command
var dumpwalletCmd = &cobra.Command{
	Use:   "dumpwallet [filename]",
	Short: "Dump wallet to a file",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("dumpwallet called")
	},
}

func init() {
	rootCmd.AddCommand(dumpwalletCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// dumpwalletCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// dumpwalletCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
