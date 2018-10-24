// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package ctl

import (
	"encoding/hex"
	"fmt"

	"github.com/BOXFoundation/boxd/wallet"
	"github.com/spf13/cobra"
)

// signmessageCmd represents the signmessage command
var signmessageCmd = &cobra.Command{
	Use:   "signmessage [message] [optional publickey]",
	Short: "Sign a message with a publickey",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("signmessage called")
		if len(args) < 2 {
			fmt.Println("Please input the hex format of signature and publickey hash")
			return
		}
		msg, err := hex.DecodeString(args[0])
		if err != nil {
			fmt.Println(err)
			return
		}
		wltMgr, err := wallet.NewWalletManager(walletDir)
		if err != nil {
			fmt.Println(err)
			return
		}
		if _, exists := wltMgr.GetAccount(args[1]); !exists {
			fmt.Println(args[1], " is not a managed account")
			return
		}
		passphrase, err := wallet.ReadPassphraseStdin()
		if err != nil {
			fmt.Println(err)
			return
		}
		sig, err := wltMgr.Sign(msg, args[1], passphrase)
		if err != nil {
			fmt.Println(err)
		}
		fmt.Println("Signature: ", hex.EncodeToString(sig))
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
