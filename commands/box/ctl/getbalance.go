// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package ctl

import (
	"fmt"

	"github.com/BOXFoundation/boxd/rpc/client"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// getbalanceCmd represents the getbalance command
var getbalanceCmd = &cobra.Command{
	Use:   "getbalance [address]",
	Short: "Get the balance for any given address",
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) < 1 {
			fmt.Println("Address required")
			return
		}
		amount, err := client.GetBalance(viper.GetViper(), args[0])
		if err != nil {
			fmt.Println(err)
		} else {
			fmt.Println("Balance of address ", args[0], " is: ", amount)
		}
	},
}

func init() {
	rootCmd.AddCommand(getbalanceCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// getbalanceCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// getbalanceCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
