// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package transaction

import (
	"fmt"

	"github.com/BOXFoundation/boxd/rpc/client"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// listutxosCmd represents the sendfrom command
var listutxosCmd = &cobra.Command{
	Use:   "listutxos",
	Short: "list all utxos",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("list utxos called")
		return client.ListUtxos(viper.GetViper())
	},
}

func init() {
	rootCmd.AddCommand(listutxosCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// sendfromCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// sendfromCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
