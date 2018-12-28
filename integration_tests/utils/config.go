// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package utils

import (
	"flag"
	"time"
)

var (
	tickerDurationTxs = 5

	txTestEnable       = true
	collAccounts       = 10
	collUnitAccounts   = 5
	circuAccounts      = 10
	circuUnitAccounts  = 5
	circuRepeatTxTimes = 200

	tokenTestEnable    = true
	tokenAccounts      = 6
	tokenUnitAccounts  = 3
	tokenRepeatTxTimes = 50

	splitAddrTestEnable    = true
	splitAddrAccounts      = 8
	splitAddrUnitAccounts  = 4
	splitAddrRepeatTxTimes = 50

	// NewNodes flag indicates to need to start nodes
	NewNodes = flag.Bool("nodes", true, "need to start nodes?")
	// EnableDocker flag indicates to need start docker
	EnableDocker = flag.Bool("docker", false, "test in docker containers?")

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

	tickerDurationTxs, err = GetIntCfgVal(5, "ticker_duration_txs")
	if err != nil {
		return err
	}
	logger.Infof("ticker duration = %d", tickerDurationTxs)

	txTestEnable, err = GetBoolCfgVal(true, "transaction_test", "enable")
	if err != nil {
		return err
	}
	logger.Infof("transaction test enable = %t", txTestEnable)

	collAccounts, err = GetIntCfgVal(10, "transaction_test", "collection_accounts")
	if err != nil {
		return err
	}
	logger.Infof("collection accounts = %d", collAccounts)

	collUnitAccounts, err = GetIntCfgVal(5, "transaction_test", "collection_unit_accounts")
	if err != nil {
		return err
	}
	logger.Infof("collection unit accounts = %d", collUnitAccounts)

	circuAccounts, err = GetIntCfgVal(10, "transaction_test", "circulation_accounts")
	if err != nil {
		return err
	}
	logger.Infof("circulation accounts = %d", circuAccounts)

	circuUnitAccounts, err = GetIntCfgVal(5, "transaction_test", "circulation_unit_accounts")
	if err != nil {
		return err
	}
	logger.Infof("circulation unit accounts = %d", circuUnitAccounts)

	circuRepeatTxTimes, err = GetIntCfgVal(100, "transaction_test",
		"circulation_repeat_times")
	if err != nil {
		return err
	}
	logger.Infof("circulation repeat tx times = %d", circuRepeatTxTimes)

	tokenTestEnable, err = GetBoolCfgVal(true, "token_test", "enable")
	if err != nil {
		return err
	}
	logger.Infof("token test enable = %t", tokenTestEnable)

	tokenAccounts, err = GetIntCfgVal(10, "token_test", "accounts")
	if err != nil {
		return err
	}
	logger.Infof("token accounts = %d", tokenAccounts)

	tokenUnitAccounts, err = GetIntCfgVal(10, "token_test", "unit_accounts")
	if err != nil {
		return err
	}
	logger.Infof("token unit accounts = %d", tokenUnitAccounts)

	tokenRepeatTxTimes, err = GetIntCfgVal(10, "token_test", "repeat_times")
	if err != nil {
		return err
	}
	logger.Infof("token repeat tx times = %d", tokenRepeatTxTimes)

	splitAddrTestEnable, err = GetBoolCfgVal(true, "split_addr_test", "enable")
	if err != nil {
		return err
	}
	logger.Infof("splitAddr test enable = %t", splitAddrTestEnable)

	splitAddrAccounts, err = GetIntCfgVal(8, "split_addr_test", "accounts")
	if err != nil {
		return err
	}
	logger.Infof("splitAddr accounts = %d", splitAddrAccounts)

	splitAddrUnitAccounts, err = GetIntCfgVal(4, "split_addr_test", "unit_accounts")
	if err != nil {
		return err
	}
	logger.Infof("splitAddrTest unit accounts = %d", splitAddrUnitAccounts)

	splitAddrRepeatTxTimes, err = GetIntCfgVal(50, "split_addr_test", "repeat_times")
	if err != nil {
		return err
	}
	logger.Infof("splitAddr repeat tx times = %d", splitAddrRepeatTxTimes)

	return nil
}

// MinerAddrs gets miners addresses from config file
func MinerAddrs() []string {
	var (
		addrs []string
		err   error
	)
	if *NewNodes {
		if *EnableDocker {
			addrs, err = GetStrArrayCfgVal(nil, "iplist", "docker")
		} else {
			addrs, err = GetStrArrayCfgVal(nil, "iplist", "local")
		}
	} else {
		addrs, err = GetStrArrayCfgVal(nil, "iplist", "testnet")
	}
	if err != nil {
		logger.Panic(err)
	}
	return addrs
}

// PeerAddrs gets execute transactions peers addresses from config file
func PeerAddrs() []string {
	addrs, err := GetStrArrayCfgVal(nil, "iplist", "tx_peers")
	if err != nil {
		logger.Panic(err)
	}
	if len(addrs) == 0 {
		return MinerAddrs()
	}
	return addrs
}

// TickerDurationTxs get ticker duration for calc tx count
func TickerDurationTxs() time.Duration {
	return time.Duration(tickerDurationTxs) * time.Second
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

// SplitAddrTestEnable return splitAddrTestEnable
func SplitAddrTestEnable() bool {
	return splitAddrTestEnable
}

// SplitAddrAccounts return splitAddrAccounts
func SplitAddrAccounts() int {
	return splitAddrAccounts
}

// SplitAddrUnitAccounts return splitAddrUnitAccounts
func SplitAddrUnitAccounts() int {
	return splitAddrUnitAccounts
}

// SplitAddrRepeatTxTimes return splitAddrRepeatTxTimes
func SplitAddrRepeatTxTimes() int {
	return splitAddrRepeatTxTimes
}
