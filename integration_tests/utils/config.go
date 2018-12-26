// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package utils

import (
	"flag"
	"time"
)

var (
	collAccounts       = 10
	collUnitAccounts   = 5
	circuAccounts      = 10
	circuUnitAccounts  = 5
	tokenAccounts      = 3
	tokenUnitAccounts  = 3
	circuRepeatTxTimes = 100
	tokenRepeatTxTimes = 10
	txTestEnable       = true
	tokenTestEnable    = true
	peersAddr          []string
	tickerDurationTxs  = 5
	tokenWorkers       = 1

	// NewNodes flag indicates to need to start nodes
	NewNodes = flag.Bool("nodes", true, "need to start nodes?")
	// EnableDocker flag indicates to need start docker
	EnableDocker = flag.Bool("docker", false, "test in docker containers?")
	// TestsCnt flag indicates to start how many nodes
	TestsCnt = flag.Int("accounts", 10, "how many need to create test acconts?")

	// LocalConf defines local test devconfig
	LocalConf = struct {
		ConfDir, WorkDir, KeyDir string
	}{"./.devconfig/", "./.devconfig/", "./.devconfig/keyfile/"}

	// DockerConf defines docker test devconfig
	DockerConf = struct {
		ConfDir, WorkDir, KeyDir string
	}{"../docker/.devconfig/", "../docker/.devconfig/", "../docker/.devconfig/keyfile/"}
)

// LoadConf load config file
func LoadConf() error {
	var err error
	if err = InitConf("./param.json"); err != nil {
		logger.Panicf("init conf using param.json error: %s", err)
	}
	collAccounts, err = GetIntCfgVal(10, "transaction_test", "collection_accounts")
	if err != nil {
		return err
	}
	logger.Infof("collAccounts = %d", collAccounts)

	collUnitAccounts, err = GetIntCfgVal(5, "transaction_test", "collection_unit_accounts")
	if err != nil {
		return err
	}
	logger.Infof("collUnitAccounts = %d", collUnitAccounts)

	circuAccounts, err = GetIntCfgVal(10, "transaction_test", "circulation_accounts")
	if err != nil {
		return err
	}
	logger.Infof("circuAccounts = %d", circuAccounts)

	circuUnitAccounts, err = GetIntCfgVal(5, "transaction_test", "circulation_unit_accounts")
	if err != nil {
		return err
	}
	logger.Infof("circuUnitAccounts = %d", circuUnitAccounts)

	tokenAccounts, err = GetIntCfgVal(10, "token_test", "accounts")
	if err != nil {
		return err
	}
	logger.Infof("tokenAccounts = %d", tokenAccounts)

	tokenUnitAccounts, err = GetIntCfgVal(10, "token_test", "unit_accounts")
	if err != nil {
		return err
	}
	logger.Infof("tokenUnitAccounts = %d", tokenUnitAccounts)

	circuRepeatTxTimes, err = GetIntCfgVal(100, "transaction_test",
		"circulation_repeat_times")
	if err != nil {
		return err
	}
	logger.Infof("circuRepeatTxTimes = %d", circuRepeatTxTimes)

	tokenRepeatTxTimes, err = GetIntCfgVal(10, "token_test", "repeat_times")
	if err != nil {
		return err
	}
	logger.Infof("tokenRepeatTxTimes = %d", tokenRepeatTxTimes)

	txTestEnable, err = GetBoolCfgVal(true, "transaction_test", "enable")
	if err != nil {
		return err
	}
	logger.Infof("TxTestEnable = %t", txTestEnable)

	tokenTestEnable, err = GetBoolCfgVal(true, "token_test", "enable")
	if err != nil {
		return err
	}
	logger.Infof("tokenTestEnable = %t", tokenTestEnable)

	tickerDurationTxs, err = GetIntCfgVal(5, "ticker_duration_txs")
	if err != nil {
		return err
	}
	logger.Infof("tickerDurationTxs = %d", tickerDurationTxs)

	return nil
}

// CollAccounts return collAccounts
func CollAccounts() int {
	return collAccounts
}

// CollUnitAccounts return collUnitAccounts
func CollUnitAccounts() int {
	return collUnitAccounts
}

// CircuAccounts return circuAccounts
func CircuAccounts() int {
	return circuAccounts
}

// CircuUnitAccounts return circuUnitAccounts
func CircuUnitAccounts() int {
	return circuUnitAccounts
}

// TokenAccounts return tokenAccounts
func TokenAccounts() int {
	return tokenAccounts
}

// TokenUnitAccounts return tokenUnitAccounts
func TokenUnitAccounts() int {
	return tokenUnitAccounts
}

// CircuRepeatTxTimes return circuRepeatTxTimes
func CircuRepeatTxTimes() int {
	return circuRepeatTxTimes
}

// TokenRepeatTxTimes return tokenRepeatTxTimes
func TokenRepeatTxTimes() int {
	return tokenRepeatTxTimes
}

// TokenTestEnable return tokenTestEnable
func TokenTestEnable() bool {
	return tokenTestEnable
}

// TxTestEnable return txTestEnable
func TxTestEnable() bool {
	return txTestEnable
}

// PeerAddrs gett peers address from config file
func PeerAddrs() []string {
	var (
		peerAddrs []string
		err       error
	)
	if *NewNodes {
		if *EnableDocker {
			peerAddrs, err = GetStrArrayCfgVal(nil, "iplist", "docker")
		} else {
			peerAddrs, err = GetStrArrayCfgVal(nil, "iplist", "local")
		}
	} else {
		peerAddrs, err = GetStrArrayCfgVal(nil, "iplist", "testnet")
	}
	if err != nil {
		logger.Panic(err)
	}
	return peerAddrs
}

// TickerDurationTxs get ticker duration for calc tx count
func TickerDurationTxs() time.Duration {
	return time.Duration(tickerDurationTxs) * time.Second
}
