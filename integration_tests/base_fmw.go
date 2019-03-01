// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"os"
	"os/signal"
	"reflect"
	"runtime"
	"sync"

	"github.com/BOXFoundation/boxd/integration_tests/utils"
)

// HandleFunc defines handler func
type HandleFunc func(addrs []string, idx *int) bool

// BaseFmw define a base test framework
type BaseFmw struct {
	accCnt  int
	partLen int
	txCnt   uint64
	addrs   []string
	quitCh  []chan os.Signal
}

// NewBaseFmw construct a BaseFmw instance
func NewBaseFmw(accCnt, partLen int) *BaseFmw {
	b := &BaseFmw{}
	// get account address
	b.accCnt = accCnt
	b.partLen = partLen
	logger.Infof("start to gen %d address", accCnt)
	b.addrs = utils.GenTestAddr(b.accCnt)
	logger.Debugf("addrs: %v\n", b.addrs)
	for _, addr := range b.addrs {
		acc := utils.UnlockAccount(addr)
		AddrToAcc.Store(addr, acc)
	}
	utils.RemoveKeystoreFiles(b.addrs...)
	for i := 0; i < (accCnt+partLen-1)/partLen; i++ {
		b.quitCh = append(b.quitCh, make(chan os.Signal, 1))
		signal.Notify(b.quitCh[i], os.Interrupt, os.Kill)
	}
	return b
}

// TearDown clean test accounts files
func (b *BaseFmw) TearDown() {
	utils.RemoveKeystoreFiles(b.addrs...)
}

// Run consumes transaction pending on circulation channel
func (b *BaseFmw) Run(handle HandleFunc) {
	var wg sync.WaitGroup
	for i := 0; i*b.partLen < len(b.addrs); i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			b.doTest(index, handle)
		}(i)
	}
	wg.Wait()
	name := runtime.FuncForPC(reflect.ValueOf(handle).Pointer()).Name()
	//name[strings.LastIndexByte(name, '/')+1:]
	logger.Infof("done %s", name)
}

func (b *BaseFmw) doTest(index int, handle HandleFunc) {
	handlerName := runtime.FuncForPC(reflect.ValueOf(handle).Pointer()).Name()
	defer func() {
		logger.Infof("done doTest[%s#%d]", handlerName, index)
		if x := recover(); x != nil {
			utils.TryRecordError(fmt.Errorf("%v", x))
		}
	}()
	start := index * b.partLen
	end := start + b.partLen
	if end > len(b.addrs) {
		end = len(b.addrs)
	}
	addrs := b.addrs[start:end]
	idx := 0
	logger.Infof("start doTest[%s#%d]", handlerName, index)
	addrsCh := make(chan []string)
	if scopeValue(*scope) == continueScope {
		go genAddrs(end-start, addrsCh)
	}
	times := 0
	for {
		if utils.Closing(b.quitCh[index]) {
			logger.Infof("receive quit signal, quiting doTest[%s#%d]!", handlerName, index)
			return
		}
		if handle(addrs, &idx) {
			break
		}
		if scopeValue(*scope) == basicScope {
			break
		}
		times++
		if times%utils.TimesToUpdateAddrs() == 0 {
			for _, addr := range addrs {
				AddrToAcc.Delete(addr)
			}
			select {
			case <-b.quitCh[index]:
				logger.Infof("receive quit signal, quiting doTest[%s#%d]!", handlerName, index)
				return
			case addrs = <-addrsCh:
			}
			logger.Warnf("times: %d", times)
		}
	}
}

func genAddrs(n int, addrsCh chan<- []string) {
	quitCh := make(chan os.Signal, 1)
	signal.Notify(quitCh, os.Interrupt, os.Kill)
	for {
		addrs := utils.GenTestAddr(n)
		for _, addr := range addrs {
			acc := utils.UnlockAccount(addr)
			AddrToAcc.Store(addr, acc)
		}
		logger.Infof("done to gen %d address", n)
		utils.RemoveKeystoreFiles(addrs...)
		select {
		case <-quitCh:
			logger.Infof("receive quit signal, quiting genAddrs")
			return
		case addrsCh <- addrs:
		}
	}
}
