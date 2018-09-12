// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// signmessageCmd represents the signmessage command
var signmessageCmd = &cobra.Command{
	Use:   "signmessage [message] [privatekey]",
	Short: "Sign a message with a privatekey",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("signmessage called")
	},
}

func init() {
	rootCmd.AddCommand(signmessageCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// signmessageCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// signmessageCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
