// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package core

// const defines constants
const (
	// subsidyReductionInterval is the interval of blocks before the subsidy is reduced.
	SubsidyReductionInterval = 180 * 24 * 3600

	// decimals is the number of digits after decimal point of value/amount
	Decimals = 8

	DuPerBox = 1e8

	// MinCoinbaseScriptLen is the minimum length a coinbase script can be.
	MinCoinbaseScriptLen = 2

	// MaxCoinbaseScriptLen is the maximum length a coinbase script can be.
	MaxCoinbaseScriptLen = 1000

	MaxBlockTimeOut = 1

	TransferGasLimit = 21000
	MinGasPrice      = 0
)

// TransferMode
const (
	DefaultMode TransferMode = iota
	BroadcastMode
	RelayMode
)

// TransferMode indicates the transfer mode
type TransferMode uint8
