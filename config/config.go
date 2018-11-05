// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BOXFoundation/boxd/consensus/dpos"
	logtypes "github.com/BOXFoundation/boxd/log/types"
	"github.com/BOXFoundation/boxd/metrics"
	"github.com/BOXFoundation/boxd/p2p"
	rpc "github.com/BOXFoundation/boxd/rpc/server"
	"github.com/BOXFoundation/boxd/storage"
)

////////////////////////////////////////////////////////////////
// build time variants

// Version number of the build
var Version string

// GitCommit id of source code
var GitCommit string

// GitBranch name of source code
var GitBranch string

////////////////////////////////////////////////////////////////

// Config is a configuration data structure for box blockchain server,
// which is read from config file or parsed from command line.
type Config struct {
	Workspace string          `mapstructure:"workspace"`
	Network   string          `mapstructure:"network"`
	Log       logtypes.Config `mapstructure:"log"`
	P2p       p2p.Config      `mapstructure:"p2p"`
	RPC       rpc.Config      `mapstructure:"rpc"`
	Database  storage.Config  `mapstructure:"database"`
	Dpos      dpos.Config     `mapstructure:"dpos"`
	Metrics   metrics.Config  `mapstructure:"metrics"`
}

var format = `workspace: %s
network: %s
log: %v
p2p: %v`

func (c Config) String() string {
	return fmt.Sprintf(format, c.Workspace, c.Network, c.Log, c.P2p)
}

// GetLog return log config.
func (c Config) GetLog() logtypes.Config {
	return c.Log
}

// Prepare function makes sure all configurations are correct.
func (c *Config) Prepare() {
	ws, err := filepath.Abs(c.Workspace)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	c.Workspace = ws // change to abs path
	mkDirAll(c.Workspace)

	// check if the network is correct.
	if magic, ok := p2p.NetworkNamtToMagic[c.Network]; ok {
		c.P2p.Magic = magic
	} else {
		fmt.Println("Incorrect network name ", c.Network)
		os.Exit(1)
	}

	// check log file configuration
	for _, hook := range c.Log.Hooks {
		if hook.Name == "file" || hook.Name == "filewithformatter" { // only check file logs
			filename, ok := hook.Options["filename"]
			if !ok {
				logfile := filepath.Join(c.Workspace, "logs", c.Network, "box.log")
				mkDirAll(filepath.Dir(logfile))
				hook.Options["filename"] = logfile
			} else if strV, ok := filename.(string); ok {
				if filepath.IsAbs(strV) { // abs dir
					mkDirAll(filepath.Dir(strV))
				} else {
					if strings.Contains(strV, "/") { // incorrect filename
						fmt.Println("Incorrect log filename ", strV)
						os.Exit(1)
					}
					if len(strV) == 0 {
						strV = "box.log"
					}
					logfile := filepath.Join(c.Workspace, "logs", c.Network, strV)
					mkDirAll(filepath.Dir(logfile))
					hook.Options["filename"] = logfile
				}
			}
		}
	}

	// database
	dbpath := filepath.Join(c.Workspace, "database", c.Network)
	mkDirAll(dbpath)
	c.Database.Path = dbpath

	// p2p
	var keyPath = c.P2p.KeyPath
	if filepath.IsAbs(keyPath) {
		mkDirAll(filepath.Dir(keyPath))
	} else if strings.Contains(keyPath, "/") {
		fmt.Println("Incorrect key filename ", keyPath)
		os.Exit(1)
	} else {
		if len(keyPath) == 0 {
			keyPath = "peer.key"
		}
		c.P2p.KeyPath = filepath.Join(c.Workspace, keyPath)
	}
}

func mkDirAll(p string) {
	if err := os.MkdirAll(p, 0700); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
