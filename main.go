// Copyright 2018 ContentBox. gang.hu@castbox.fm.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package main

import (
	"runtime"

	"github.com/BOXFoundation/Quicksilver/cmd"
)

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	cmd.Execute()
}
