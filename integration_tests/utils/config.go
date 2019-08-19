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
	circuRepeatRandom  = true

	tokenTestEnable    = true
	tokenAccounts      = 6
	tokenUnitAccounts  = 3
	tokenRepeatTxTimes = 50
	tokenRepeatRandom  = true

	splitAddrTestEnable    = true
	splitAddrAccounts      = 8
	splitAddrUnitAccounts  = 4
	splitAddrRepeatTxTimes = 50
	splitAddrRepeatRandom  = true

	contractTestEnable    = true
	contractAccounts      = 6
	contractUnitAccounts  = 3
	contractRepeatTxTimes = 50
	contractRepeatRandom  = true

	p2pTestEnable = true

	timesToUpdateAddrs = 100

	// NewNodes flag indicates to need to start nodes
	NewNodes = flag.Bool("nodes", true, "need to start nodes?")
	// EnableDocker flag indicates to need start docker
	EnableDocker = flag.Bool("docker", false, "test in docker containers?")
	// NewFullNodes flag indicates to create other full nodes composed networks
	NewFullNodes = flag.Int("fullnodes", 0, "need to create other types of full nodes?")

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

	// transaction test
	// collection
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

	// circulation
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

	circuRepeatRandom, err = GetBoolCfgVal(true, "transaction_test", "enable_repeat_random")
	if err != nil {
		return err
	}
	logger.Infof("transaction test enable_repeat_random = %t", circuRepeatRandom)

	// p2p
	p2pTestEnable, err = GetBoolCfgVal(true, "p2p_test", "enable")
	if err != nil {
		return err
	}
	logger.Infof("p2p test enable = %t", p2pTestEnable)

	// token
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

	tokenRepeatRandom, err = GetBoolCfgVal(true, "token_test", "enable_repeat_random")
	if err != nil {
		return err
	}
	logger.Infof("token test enable_repeat_random = %t", tokenRepeatRandom)

	// split addr
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

	splitAddrRepeatRandom, err = GetBoolCfgVal(true, "split_addr_test", "enable_repeat_random")
	if err != nil {
		return err
	}
	logger.Infof("splitAddr test enable_repeat_random = %t", splitAddrRepeatRandom)

	// contract
	contractTestEnable, err = GetBoolCfgVal(true, "contract_test", "enable")
	if err != nil {
		return err
	}
	logger.Infof("contract test enable = %t", contractTestEnable)

	contractAccounts, err = GetIntCfgVal(10, "contract_test", "accounts")
	if err != nil {
		return err
	}
	logger.Infof("contract accounts = %d", contractAccounts)

	contractUnitAccounts, err = GetIntCfgVal(10, "contract_test", "unit_accounts")
	if err != nil {
		return err
	}
	logger.Infof("contract unit accounts = %d", contractUnitAccounts)

	contractRepeatTxTimes, err = GetIntCfgVal(10, "contract_test", "repeat_times")
	if err != nil {
		return err
	}
	logger.Infof("contract repeat tx times = %d", contractRepeatTxTimes)

	contractRepeatRandom, err = GetBoolCfgVal(true, "contract_test", "enable_repeat_random")
	if err != nil {
		return err
	}
	logger.Infof("contract test enable_repeat_random = %t", contractRepeatRandom)

	timesToUpdateAddrs, err = GetIntCfgVal(100, "times_to_update_addrs")
	if err != nil {
		return err
	}
	logger.Infof("times to update addrs = %d", timesToUpdateAddrs)

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
			addrs, err = GetStrArrayCfgVal(nil, "miners", "docker")
		} else {
			addrs, err = GetStrArrayCfgVal(nil, "miners", "local")
		}
	} else {
		addrs, err = GetStrArrayCfgVal(nil, "miners", "testnet")
	}
	if err != nil {
		logger.Panic(err)
	}
	return addrs
}

// OtherAddrs gets all addresses that are not miners
func OtherAddrs() []string {
	var (
		addrs []string
		err   error
	)
	length := *NewFullNodes
	if length > 0 {
		if *EnableDocker {
			addrs, err = GetStrArrayCfgVal(nil, "iplist", "docker")
		} else {
			addrs, err = GetStrArrayCfgVal(nil, "iplist", "local")
		}
		if length > len(addrs) {
			length = len(addrs)
		}
		return addrs[:length]
	}
	if err != nil {
		logger.Panic(err)
	}
	return []string{}
}

// AllAddrs gets all addresses from config file
func AllAddrs() []string {
	return append(MinerAddrs(), OtherAddrs()...)
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

// CircuRepeatTxTimes return circuRepeatTxTimes
func CircuRepeatTxTimes() int {
	return circuRepeatTxTimes
}

// CircuRepeatRandom return circuRepeatRandom
func CircuRepeatRandom() bool {
	return circuRepeatRandom
}

// TxTestEnable return txTestEnable
func TxTestEnable() bool {
	return txTestEnable
}

// TimesToUpdateAddrs return timesToUpdateAddrs
func TimesToUpdateAddrs() int {
	return timesToUpdateAddrs
}

// TokenAccounts return tokenAccounts
func TokenAccounts() int {
	return tokenAccounts
}

// TokenUnitAccounts return tokenUnitAccounts
func TokenUnitAccounts() int {
	return tokenUnitAccounts
}

// TokenRepeatTxTimes return tokenRepeatTxTimes
func TokenRepeatTxTimes() int {
	return tokenRepeatTxTimes
}

// TokenRepeatRandom return circuRepeatRandom
func TokenRepeatRandom() bool {
	return tokenRepeatRandom
}

// TokenTestEnable return tokenTestEnable
func TokenTestEnable() bool {
	return tokenTestEnable
}

// P2pTestEnable return p2pTestEnable
func P2pTestEnable() bool {
	return p2pTestEnable
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

// SplitAddrRepeatRandom return splitAddrRepeatRandom
func SplitAddrRepeatRandom() bool {
	return splitAddrRepeatRandom
}

// ContractAccounts return contractAccounts
func ContractAccounts() int {
	return contractAccounts
}

// ContractUnitAccounts return contractUnitAccounts
func ContractUnitAccounts() int {
	return contractUnitAccounts
}

// ContractRepeatTxTimes return contractRepeatTxTimes
func ContractRepeatTxTimes() int {
	return contractRepeatTxTimes
}

// ContractRepeatRandom return circuRepeatRandom
func ContractRepeatRandom() bool {
	return contractRepeatRandom
}

// ContractTestEnable return contractTestEnable
func ContractTestEnable() bool {
	return contractTestEnable
}
