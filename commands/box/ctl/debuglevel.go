// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package ctl

import (
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	rpc "github.com/BOXFoundation/boxd/rpc/client"
)

// debuglevelCmd represents the debuglevel command
var debuglevelCmd = &cobra.Command{
	Use:   "debuglevel [debug|info|warning|error|fatal]",
	Short: "Set the debug level of boxd",
	RunE: func(cmd *cobra.Command, args []string) error {
		level := "info"
		if len(args) > 0 {
			level = args[0]
		}
		return rpc.SetDebugLevel(viper.GetViper(), level)
	},
}

func init() {
	rootCmd.AddCommand(debuglevelCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// debuglevelCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// debuglevelCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
