// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package box

import (
	"fmt"
	"os"

	_ "github.com/BOXFoundation/boxd/commands/box/ctl" // init ctl cmd
	root "github.com/BOXFoundation/boxd/commands/box/root"
	_ "github.com/BOXFoundation/boxd/commands/box/splitaddr"   // init splitaddr cmd
	_ "github.com/BOXFoundation/boxd/commands/box/start"       // init start cmd
	_ "github.com/BOXFoundation/boxd/commands/box/token"       // init token cmd
	_ "github.com/BOXFoundation/boxd/commands/box/transaction" // init transaction cmd
	_ "github.com/BOXFoundation/boxd/commands/box/wallet"      // init wallet cmd
)

// Execute is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := root.RootCmd.Execute(); err != nil {
		fmt.Print(err)
		os.Exit(1)
	}
}
