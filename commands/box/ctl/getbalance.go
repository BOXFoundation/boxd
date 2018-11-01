// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package ctl

import (
	"fmt"
	"github.com/BOXFoundation/boxd/wallet"

	"github.com/BOXFoundation/boxd/rpc/client"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// getbalanceCmd represents the getbalance command
var getbalanceCmd = &cobra.Command{
	Use:   "getbalance [address]",
	Short: "Get the balance for any given address",
	Run: func(cmd *cobra.Command, args []string) {
		addrs := make([]string, 0)
		if len(args) < 1 {
			wltMgr, err := wallet.NewWalletManager(walletDir)
			if err != nil {
				fmt.Println(err)
				return
			}
			for _, acc := range wltMgr.ListAccounts() {
				addrs = append(addrs, acc.Addr())
			}
		} else {
			addrs = append(addrs, args[0])
		}
		balances, err := client.GetBalance(viper.GetViper(), addrs)
		if err != nil {
			fmt.Println(err)
			return
		}
		var total uint64
		for addr, balance := range balances {
			fmt.Printf("Addr: %s\t Balance: %d\n", addr, balance)
			total += balance
		}
		if len(balances) > 1 {
			fmt.Println("Total balance: ", total)
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
