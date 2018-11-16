package main

import "os"

// Audit manage transaction audit
type Audit struct {
	quitAuditCh chan os.Signal
}
