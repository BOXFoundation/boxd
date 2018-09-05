// Copyright 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

// Package cmd defines and implements command-line commands and flags
// used by Quicksilver. Commands and flags are implemented using Cobra.
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "boxd",
	Short: "BOX Payout command-line interface",
	Long: `BOX Payout, a lightweight blockchain built for processing
			multi-party payments on digital content apps.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Starting p2p node...")
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
