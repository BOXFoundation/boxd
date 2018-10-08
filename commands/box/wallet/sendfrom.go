// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package wallet

import (
	"encoding/hex"
	"fmt"
	"strconv"

	"github.com/spf13/viper"

	rpc "github.com/BOXFoundation/boxd/rpc/client"
	"github.com/spf13/cobra"
)

// sendfromCmd represents the sendfrom command
var sendfromCmd = &cobra.Command{
	Use:   "sendfrom [fromaccount] [toaddress] [amount]",
	Short: "Send coins from an account to an address",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("sendfrom called")
		if len(args) != 3 {
			return fmt.Errorf("Invalid argument number")
		}
		fromPubKey, err1 := hex.DecodeString(args[0])
		toPubKey, err2 := hex.DecodeString(args[1])
		amount, err3 := strconv.Atoi(args[2])
		if err1 != nil || err2 != nil || err3 != nil {
			return fmt.Errorf("Invalid argument format")
		}
		return rpc.CreateTransaction(viper.GetViper(), fromPubKey, toPubKey, int64(amount))
	},
}

func init() {
	rootCmd.AddCommand(sendfromCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// sendfromCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// sendfromCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
