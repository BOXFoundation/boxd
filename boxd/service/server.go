// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package service

import "github.com/jbenet/goprocess"

// Server defines methods to start/stop a server
type Server interface {
	// Run a server
	Run() error
	// Stop the service. It is blocked unitl the server is down.
	Stop()

	// Proc returns the goprocess of server is running
	Proc() goprocess.Process
}
