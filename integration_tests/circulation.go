// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/BOXFoundation/boxd/core"
	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/integration_tests/utils"
	"github.com/BOXFoundation/boxd/rpc/client"
	"google.golang.org/grpc"
)

// Circulation manage circulation of transaction
type Circulation struct {
	*BaseFmw
	collAddrCh chan<- string
	cirInfoCh  <-chan CirInfo
}

// NewCirculation construct a Circulation instance
func NewCirculation(accCnt, partLen int, collAddrCh chan<- string,
	cirInfoCh <-chan CirInfo) *Circulation {
	c := &Circulation{}
	c.BaseFmw = NewBaseFmw(accCnt, partLen)
	c.collAddrCh = collAddrCh
	c.cirInfoCh = cirInfoCh
	return c
}

// HandleFunc hooks test func
func (c *Circulation) HandleFunc(addrs []string, idx *int) (exit bool) {
	*idx = *idx % len(addrs)

	quitCh := make(chan os.Signal, 1)
	signal.Notify(quitCh, os.Interrupt, os.Kill)

	select {
	case c.collAddrCh <- addrs[*idx]:
		toIdx := (*idx + 1) % len(addrs)
		toAddr := addrs[toIdx]
		*idx = toIdx
		select {
		case cirInfo, ok := <-c.cirInfoCh:
			if ok {
				logger.Infof("start box circulation between accounts on %s", cirInfo.PeerAddr)
				txRepeatTest(cirInfo.Addr, toAddr, cirInfo.PeerAddr, utils.CircuRepeatTxTimes(), &c.txCnt)
				return false
			}
		case s := <-quitCh:
			logger.Infof("receive quit signal %v, quiting HandleFunc[%d]!", s, idx)
			return true
		}
	case s := <-quitCh:
		logger.Infof("receive quit signal %v, quiting HandleFunc!", s)
		return true
	}
	return
}

func txRepeatTest(fromAddr, toAddr string, execPeer string, times int, txCnt *uint64) {
	logger.Info("=== RUN   txRepeatTest")
	defer logger.Infof("--- DONE: txRepeatTest")
	defer func() {
		if x := recover(); x != nil {
			utils.TryRecordError(fmt.Errorf("%v", x))
			logger.Error(x)
		}
	}()
	if times <= 0 {
		logger.Warn("times is 0, exit")
		return
	}
	//
	fromBalancePre := utils.BalanceFor(fromAddr, execPeer)
	if fromBalancePre == 0 {
		logger.Warnf("balance of %s is 0, exit", fromAddr)
		return
	}
	toBalancePre := utils.BalanceFor(toAddr, execPeer)
	logger.Infof("fromAddr[%s] balance: %d, toAddr[%s] balance: %d",
		fromAddr, fromBalancePre, toAddr, toBalancePre)
	logger.Infof("start to construct txs from %s to %s %d times", fromAddr, toAddr, times)
	start := time.Now()
	txss, transfer, fee, count, err := utils.NewTxs(AddrToAcc[fromAddr], toAddr,
		times, execPeer)
	eclipse := float64(time.Since(start).Nanoseconds()) / 1e6
	logger.Infof("create %d txs cost: %6.3f ms", count, eclipse)
	if err != nil {
		logger.Panic(err)
	}
	conn, _ := grpc.Dial(execPeer, grpc.WithInsecure())
	defer conn.Close()
	var wg sync.WaitGroup
	errChans := make(chan error, len(txss))
	logger.Infof("start to send tx from %s to %s %d times", fromAddr, toAddr, times)
	start = time.Now()
	for _, txs := range txss {
		wg.Add(1)
		go func(txs []*types.Transaction) {
			defer func() {
				wg.Done()
				if x := recover(); x != nil {
					errChans <- fmt.Errorf("%v", x)
				}
			}()

			// reverse txs
			//for i, j := 0, len(txs)-1; i < j; {
			//	txs[i], txs[j] = txs[j], txs[i]
			//	i, j = i+1, j-1
			//}

			for _, tx := range txs {
				if err := client.SendTransaction(conn, tx); err != nil &&
					!strings.Contains(err.Error(), core.ErrOrphanTransaction.Error()) {
					logger.Panic(err)
				}
				time.Sleep(4 * time.Millisecond)
				atomic.AddUint64(txCnt, 1)
			}
		}(txs)
	}
	wg.Wait()
	if len(errChans) > 0 {
		logger.Panic(<-errChans)
	}
	eclipse = float64(time.Since(start).Nanoseconds()) / 1e6
	logger.Infof("send %d txs cost: %6.3f ms", count, eclipse)

	logger.Infof("%s sent %d transactions total %d to %s on peer %s",
		fromAddr, count, transfer, toAddr, execPeer)
	logger.Infof("wait for balance of %s reach %d, timeout %v", toAddr,
		toBalancePre+transfer, timeoutToChain)
	toBalancePost, err := utils.WaitBalanceEnough(toAddr, toBalancePre+transfer,
		execPeer, timeoutToChain)
	if err != nil {
		utils.TryRecordError(err)
		logger.Warn(err)
	}
	// check the balance of sender
	fromBalancePost := utils.BalanceFor(fromAddr, execPeer)
	logger.Infof("fromAddr[%s] balance: %d toAddr[%s] balance: %d",
		fromAddr, fromBalancePost, toAddr, toBalancePost)
	// prerequisite: neither of fromAddr and toAddr are not miner address
	toGap := toBalancePost - toBalancePre
	fromGap := fromBalancePre - fromBalancePost
	if fromGap != fee+transfer || toGap != transfer {
		err := fmt.Errorf("txRepeatTest faild: fromGap %d toGap %d transfer %d and "+
			"fee %d", fromGap, toGap, transfer, fee)
		utils.TryRecordError(err)
		logger.Error(err)
	}
}
