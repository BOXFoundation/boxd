// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package main

import (
	"io/ioutil"
	"strconv"
	"strings"
	"time"

	"github.com/BOXFoundation/boxd/integration_tests/utils"
	"github.com/BOXFoundation/boxd/rpc/rpcutil"
	"github.com/BOXFoundation/boxd/util"
	"github.com/jbenet/goprocess"
	yaml "gopkg.in/yaml.v2"
)

// Yaml asaasdf
type Yaml struct {
	// P2pYaml is a p2p profile mapping
	P2p struct {
		Seeds      []string `yaml:"seeds"`
		Principals []string `yaml:"principals"`
		Agents     []string `yaml:"agents"`
	}
}

func testP2p(proc goprocess.Process) {

	miners := utils.MinerAddrs()
	others := utils.OtherAddrs()
	if len(others) == 0 {
		return
	}

	ticker := time.NewTicker(10 * time.Second)
	for {

		select {
		case <-ticker.C:
		case <-proc.Closing():
			logger.Info("Quit p2p test loop.")
			return
		}

		minerAddrs := []string{}
		for _, addr := range miners {
			conn, err := rpcutil.GetGRPCConn(addr)
			if err != nil {
				logger.Error(err)
				panic(err)
			}
			peerid := utils.PeerID(conn)
			minerAddrs = append(minerAddrs, peerid)
		}
		logger.Debugf("minerAddrs: %v", minerAddrs)

		for i, addr := range miners {
			seeds, agents, principals, err := getTableConfig(i + 1)
			if err != nil {
				panic(err)
			}

			conn, err := rpcutil.GetGRPCConn(addr)
			if err != nil {
				logger.Error(err)
				panic(err)
			}
			connectings := utils.Table(conn)

			expectedConnSet := append(append(append(minerAddrs, seeds...), agents...), principals...)
			for _, remoteID := range connectings {
				if !util.InStrings(remoteID, expectedConnSet) {
					logger.Errorf("Miner: %s connected to node %s that were not expected to be connected", addr, remoteID)
				}
			}
		}

		for i, addr := range others {
			_, agents, principals, err := getTableConfig(i + len(miners) + 1)
			if err != nil {
				panic(err)
			}

			conn, err := rpcutil.GetGRPCConn(addr)
			if err != nil {
				logger.Error(err)
				panic(err)
			}
			connectings := utils.Table(conn)

			for _, expectedConnID := range append(agents, principals...) {
				if !util.InStrings(expectedConnID, connectings) {
					logger.Errorf("%s are not connected to the node %s it expect to connect to", addr, expectedConnID)
				}
			}
		}
	}
}

func getTableConfig(idx int) (seeds, agents, principals []string, err error) {
	conf := new(Yaml)
	yamlFile, err := ioutil.ReadFile(".devconfig/.box-" + strconv.FormatInt(int64(idx), 10) + ".yaml")

	if err != nil {
		logger.Errorf("yamlFile.Get err %v ", err)
		return
	}
	err = yaml.Unmarshal(yamlFile, conf)
	if err != nil {
		logger.Errorf("Unmarshal: %v", err)
		return
	}

	for _, seed := range conf.P2p.Seeds {
		strs := strings.Split(seed, "/")
		seeds = append(seeds, strs[len(strs)-1])
	}
	for _, agent := range conf.P2p.Agents {
		strs := strings.Split(agent, "/")
		agents = append(agents, strs[len(strs)-1])
	}
	for _, principal := range conf.P2p.Principals {
		strs := strings.Split(principal, "/")
		principals = append(principals, strs[len(strs)-1])
	}
	return
}
