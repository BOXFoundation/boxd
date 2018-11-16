// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package main

import "os"

// Audit manage transaction audit
type Audit struct {
	quitAuditCh chan os.Signal
}
