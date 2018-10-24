// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package ctl

import (
	"fmt"

	"github.com/BOXFoundation/boxd/core/types"
	"github.com/spf13/cobra"
)

// validateaddressCmd represents the validateaddress command
var validateaddressCmd = &cobra.Command{
	Use:   "validateaddress [address]",
	Short: "Check if an address is valid",
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) < 1 {
			fmt.Println("Please specify the address to validate")
			return
		}
		fmt.Println("validateing address", args[0])
		addr := &types.AddressPubKeyHash{}
		if err := addr.SetString(args[0]); err != nil {
			fmt.Println(err)
		} else {
			fmt.Println(args[0], " is a valid address")
		}
	},
}

func init() {
	rootCmd.AddCommand(validateaddressCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// validateaddressCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// validateaddressCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
