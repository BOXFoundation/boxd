// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// verifymessageCmd represents the verifymessage command
var verifymessageCmd = &cobra.Command{
	Use:   "verifymessage [message] [publickey]",
	Short: "Verify a message with a public key",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("verifymessage called")
	},
}

func init() {
	rootCmd.AddCommand(verifymessageCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// verifymessageCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// verifymessageCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
