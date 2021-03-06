// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"time"

	"github.com/BOXFoundation/boxd/core/chain"
	"github.com/BOXFoundation/boxd/integration_tests/utils"
	"google.golang.org/grpc"
)

type picker struct {
	sync.Mutex
	status []bool
}

const (
	timeoutToChain = 15 * time.Second
)

var (
	minerPicker picker

	lastTxTestTxCnt    = uint64(0)
	lastTokenTestTxCnt = uint64(0)
	txTestTxCnt        = uint64(0)
	tokenTestTxCnt     = uint64(0)
)

func initOrigMinerPicker(minerCnt int) {
	minerPicker = picker{status: make([]bool, minerCnt)}
}

// PickOneMiner picks a miner address that was not picked by other goroutine
func PickOneMiner() (string, bool) {
	minerPicker.Lock()
	defer minerPicker.Unlock()
	conn, err := grpc.Dial(peersAddr[0], grpc.WithInsecure())
	if err != nil {
		logger.Error(err)
		return "", false
	}
	defer conn.Close()

	for i, picked := range minerPicker.status {
		if !picked {
			if _, err := utils.WaitBalanceEnough(origAddrs[i], testCoins, conn,
				time.Second); err != nil {
				time.Sleep(time.Second)
				continue
			}
			logger.Infof("Picked miner %s box with %d", origAddrs[i], chain.BaseSubsidy)
			time.Sleep(time.Second) // to avoid utxo cache in wallet agent
			minerPicker.status[i] = true
			return origAddrs[i], true
		}
	}
	return "", false
}

// UnpickMiner unpick a miner address
func UnpickMiner(addr string) bool {
	minerPicker.Lock()
	defer minerPicker.Unlock()
	for i, a := range origAddrs {
		if a == addr {
			minerPicker.status[i] = false
			return true
		}
	}
	return false
}

func runItem(wg *sync.WaitGroup, errChans chan<- error, run func()) {
	wg.Add(1)
	go func() {
		defer func() {
			wg.Done()
			if x := recover(); x != nil {
				errChans <- fmt.Errorf("%v", x)
			}
		}()
		run()
	}()
}

// CountTxs count txs
func CountTxs(globalCnt *uint64, localCnt ...*uint64) {
	logger.Info("txs ticker start...")
	t := time.NewTicker(time.Second)
	defer t.Stop()
	quitCh := make(chan os.Signal, 1)
	signal.Notify(quitCh, os.Interrupt, os.Kill)

	for {
		select {
		case <-t.C:
			count := uint64(0)
			for _, c := range localCnt {
				count += atomic.LoadUint64(c)
			}
			atomic.StoreUint64(globalCnt, count)
		case <-quitCh:
			logger.Info("txs ticker exit")
			return
		}
	}
}

// CountGlobalTxs count global txs
func CountGlobalTxs() {
	logger.Info("txs ticker for main start")
	time.Sleep(timeoutToChain)
	d := utils.TickerDurationTxs()
	t := time.NewTicker(d)
	defer t.Stop()
	quitCh := make(chan os.Signal, 1)
	signal.Notify(quitCh, os.Interrupt, os.Kill)

	for {
		select {
		case <-t.C:
			txCnt := atomic.LoadUint64(&txTestTxCnt)
			tokenCnt := atomic.LoadUint64(&tokenTestTxCnt)
			totalTxs := txCnt + tokenCnt
			lastTotalTxs := lastTxTestTxCnt + lastTokenTestTxCnt
			txs := totalTxs - lastTotalTxs
			logger.Infof("TPS = %6.2f during last %v, total txs = %d",
				float64(txs)/float64(d/time.Second), d, totalTxs)
			lastTxTestTxCnt, lastTokenTestTxCnt = txCnt, tokenCnt
		case <-quitCh:
			logger.Info("txs ticker for main exit")
			return
		}
	}
}
