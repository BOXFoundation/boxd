// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package ctl

import (
	"fmt"

	"github.com/BOXFoundation/boxd/crypto"
	"github.com/BOXFoundation/boxd/rpc/client"
	"github.com/BOXFoundation/boxd/util"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// getrawtxCmd represents the getrawtx command
var getrawtxCmd = &cobra.Command{
	Use:   "getrawtx [txhash]",
	Short: "Get the raw transaction for a txid",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("getrawtx called")
		if len(args) < 1 {
			fmt.Println("Param txhash required")
			return
		}
		hash := crypto.HashType{}
		hash.SetString(args[0])
		tx, err := client.GetRawTransaction(viper.GetViper(), hash.GetBytes())
		if err != nil {
			fmt.Println(err)
		} else {
			fmt.Println(util.PrettyPrint(tx))
		}
	},
}

func init() {
	rootCmd.AddCommand(getrawtxCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// getrawtxCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// getrawtxCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
