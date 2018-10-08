// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package wallet

import (
	"fmt"

	"github.com/spf13/viper"

	"github.com/BOXFoundation/boxd/rpc/client"

	"github.com/spf13/cobra"
)

// listtransactionsCmd represents the listtransactions command
var listtransactionsCmd = &cobra.Command{
	Use:   "listtransactions [account] [count] [from]",
	Short: "List transactions for an account",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("listtransactions called")
		if len(args) > 0 {
			client.ListTransactions(viper.GetViper(), args[0])
		}
	},
}

func init() {
	rootCmd.AddCommand(listtransactionsCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// listtransactionsCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// listtransactionsCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
