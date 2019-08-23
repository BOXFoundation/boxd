// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"math/rand"
	"os"
	"os/signal"
	"strings"
	"sync"
	"time"

	"github.com/BOXFoundation/boxd/core"
	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/integration_tests/utils"
	"github.com/BOXFoundation/boxd/log"
	"github.com/BOXFoundation/boxd/rpc/rpcutil"
	acc "github.com/BOXFoundation/boxd/wallet/account"
	"github.com/jbenet/goprocess"
	"google.golang.org/grpc"
)

type scopeValue string

const (
	voidScope     scopeValue = "void"
	basicScope    scopeValue = "basic"
	mainScope     scopeValue = "main"
	fullScope     scopeValue = "full"
	continueScope scopeValue = "continue"
)

var logger = log.NewLogger("integration") // logger

var (
	scope = flag.String("scope", "basic", "can select basic/main/full/continue cases")

	peersAddr []string
	origAddrs []string
	origAccs  []*acc.Account

	preAddr string
	preAcc  *acc.Account

	//AddrToAcc stores addr to account
	AddrToAcc = new(sync.Map)
)

func init() {
	rand.Seed(time.Now().Unix())
}

func initAcc() {
	origAccCnt := 6
	origAddrs = utils.GenTestAddr(origAccCnt)
	logger.Debugf("original account addrs: %v\n", origAddrs)
	for _, addr := range origAddrs {
		acc := utils.UnlockAccount(addr)
		AddrToAcc.Store(addr, acc)
		origAccs = append(origAccs, acc)
	}
	utils.RemoveKeystoreFiles(origAddrs...)

	preKeyStore := utils.LocalConf.KeyDir + "pre.keystore"
	preAddrs, preAccs := utils.LoadAccounts(preKeyStore)
	preAddr, preAcc = preAddrs[0], preAccs[0]
	AddrToAcc.Store(preAddr, preAcc)
	logger.Infof("init pre-allocation accounts: %s", preAddr)
}

func main() {
	proc := goprocess.WithSignals(os.Interrupt)
	defer func() {
		if x := recover(); x != nil {
			os.Exit(1)
		}
	}()
	flag.Parse()
	if err := utils.LoadConf(); err != nil {
		logger.Panic(err)
	}
	initAcc()
	initOrigMinerPicker(len(origAddrs))
	peersAddr = utils.PeerAddrs()

	if *utils.NewNodes {
		// prepare environment and clean history data
		if err := utils.PrepareEnv(len(utils.MinerAddrs())); err != nil {
			logger.Panic(err)
		}
		logger.Warnf("miner addrs: %v", utils.MinerAddrs())
		//defer utils.TearDown(len(minerAddrs))

		// start nodes
		if *utils.EnableDocker {
			if err := utils.StartDockerNodes(); err != nil {
				logger.Panic(err)
			}
			defer utils.StopNodes()
		} else {
			processes, err := utils.StartLocalNodes(len(utils.MinerAddrs()))
			defer utils.StopLocalNodes(processes...)
			if utils.P2pTestEnable() {
				go testP2p(proc)
			}
			if err != nil {
				logger.Panic(err)
			}
		}
		time.Sleep(3 * time.Second) // wait for 3s to let boxd started
	}

	switch scopeValue(*scope) {
	case voidScope:
		logger.Info("integration run in void mode, nodes run without txs")
		quitCh := make(chan os.Signal, 1)
		signal.Notify(quitCh, os.Interrupt, os.Kill)
		<-quitCh
		logger.Info("quit integration in void mode")
		return
	case continueScope:
		// print tx count per TickerDurationTxs
		go CountGlobalTxs()
		fallthrough
	default:
		go topupOrigAccs()
	}

	var wg sync.WaitGroup
	testCnt := 3
	errChans := make(chan error, testCnt)

	for _, f := range testItems() {
		runItem(&wg, errChans, f)
	}

	wg.Wait()
	for len(errChans) > 0 {
		utils.TryRecordError(<-errChans)
	}
	// check whether integration success
	for _, e := range utils.ErrItems {
		logger.Error(e)
	}
	if len(utils.ErrItems) > 0 {
		// use panic to exit since it need to execute defer clause above
		logger.Panicf("integration tests exits with %d errors", len(utils.ErrItems))
	}
	logger.Info("\r\n\n====>> CONGRATULATION! All CASES PASSED, GREAT JOB! <<====\n\n\r")
}

func testItems() []func() {
	var items []func()
	// test tx
	if utils.TxTestEnable() {
		items = append(items, txTest)
	}

	// test token
	if utils.TokenTestEnable() {
		items = append(items, tokenTest)
	}

	// test split address
	if utils.SplitAddrTestEnable() {
		items = append(items, splitAddrTest)
	}

	// test contract
	if utils.ContractTestEnable() {
		items = append(items, contractTest)
	}

	return items
}

func topupOrigAccs() {
	defer func() {
		//if x := recover(); x != nil {
		//	logger.Warn(x)
		//}
	}()
	// quit channel
	quitCh := make(chan os.Signal, 1)
	signal.Notify(quitCh, os.Interrupt, os.Kill)
	// conn
	conn, err := grpc.Dial(peersAddr[0], grpc.WithInsecure())
	if err != nil {
		logger.Panic(err)
	}
	defer conn.Close()

	t := time.NewTicker(2 * time.Second)
	defer t.Stop()
	origAccCnt := len(origAddrs)
	i := 0
	for {
		select {
		case <-t.C:
			balance, err := utils.BalanceNoPanicFor(preAddr, conn)
			if err != nil {
				logger.Errorf("fetch balance for pre addr %s error %s", preAddr, err)
				continue
			}
			accI, accJ := origAccs[i%origAccCnt], origAccs[(i+3)%origAccCnt]
			i++
			amount := 25 * uint64(core.DuPerBox)
			tx, _, fee, err := rpcutil.NewTx(preAcc,
				[]*types.AddressHash{accI.AddressHash(), accJ.AddressHash()},
				[]uint64{amount, amount}, conn)
			if err != nil {
				logger.Errorf("new tx for pre addr %s to origal account %s %s error %s",
					preAddr, accI.Addr(), accJ.Addr(), err)
				continue
			}
			_, err = rpcutil.SendTransaction(conn, tx)
			if err != nil && !strings.Contains(err.Error(), core.ErrOrphanTransaction.Error()) {
				logger.Error(err)
				continue
				//logger.Panic(err)
			}
			select {
			case <-quitCh:
				logger.Info("quit topupOrigAccs.")
				return
			default:
			}
			balance, err = utils.WaitBalanceEqual(preAddr, balance-2*amount-fee, conn,
				10*time.Second)
			if err != nil {
				logger.Error(err)
				continue
			}
		case <-quitCh:
			logger.Info("quit topupOrigAccs.")
			return
		}
	}
}
