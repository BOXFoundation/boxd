// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package chain

import (
	"math"
	"strconv"
	"time"

	"github.com/BOXFoundation/boxd/core"
	"github.com/BOXFoundation/boxd/core/abi"
	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/crypto"
	"github.com/BOXFoundation/boxd/script"
)

// genesis contract properties
var (
	// Admin represents admin address.
	Admin = "b1na9uCQXA26d94w1tWrttnnsfjKNz9M2EF"

	// ContractAddr genesis contract address.
	ContractAddr types.AddressHash

	// ContractBin genesis contract bin.
	ContractBin []byte

	// ContractAbi genesis contract abi.
	ContractAbi *abi.ABI

	CalcBonus     = "calcBonus"
	ExecBonus     = "execBonus"
	CalcScore     = "calcScore"
	DynastySwitch = "DynastySwitch()"
)

// GenesisBlock represents genesis block.
var GenesisBlock = types.Block{
	Header: &types.BlockHeader{
		Version:       1,
		PrevBlockHash: crypto.HashType{}, // 0000000000000000000000000000000000000000000000000000000000000000
		TimeStamp:     time.Date(2018, 1, 31, 0, 0, 0, 0, time.UTC).Unix(),
		Height:        0,
	},
}

// GenesisPeriod genesis period
var GenesisPeriod = []map[string]string{
	{
		"addr":   "b1ndoQmEd83y4Fza5PzbUQDYpT3mV772J5o",
		"peerID": "12D3KooWFQ2naj8XZUVyGhFzBTEMrMc6emiCEDKLjaJMsK7p8Cza",
	},
	{
		"addr":   "b1b8bzyci5VYUJVKRU2HRMMQiUXnoULkKAJ",
		"peerID": "12D3KooWKPRAK7vBBrVv9szEin55kBnJEEuHG4gDTQEM72ByZDpA",
	},
	{
		"addr":   "b1jh8DSdB6kB7N7RanrudV1hzzMCCcoX6L7",
		"peerID": "12D3KooWSdXLNeoRQQ2a7yiS6xLpTn3LdCr8B8fqPz94Bbi7itsi",
	},
	{
		"addr":   "b1UP5pbfJgZrF1ezoSHLdvkxvgF2BYLtGva",
		"peerID": "12D3KooWRHVAwymCVcA8jqyjpP3r3HBkCW2q5AZRTBvtaungzFSJ",
	},
	{
		"addr":   "b1ZWSdrg48g145VdcmBwMPVuDFdaxDLoktk",
		"peerID": "12D3KooWQSaxCgbWakLcU69f4gmNFMszwhyHbwx4xPAhV7erDC2P",
	},
	{
		"addr":   "b1fRtRnKF4qhQG7bSwqbgR2BMw9VfM2XpT4",
		"peerID": "12D3KooWNcJQzHaNpW5vZDQbTcoLXVCyGS755hTpendGzb5Hqtcu",
	},
}

// tokenPreAllocation token pre_allocation
// total 3 billion, 2.1 billion pre_allocation and 0.9 billion pay for bookkeeper.
// 0.45 billion for team, unlocked in four years, 112.5 million per year.
var tokenPreAllocation = []map[string]string{
	{ // token for team
		"addr":  "b1aft2B3VdkLpfB8xjjcWKG5GJRm1ud3jGv",
		"value": "112500000",
	},
	{ // token for team
		"addr":     "b1ck4dcMF8GBCk1okTG4U41TCE2EnY3dG9X",
		"value":    "112500000",
		"locktime": "31536000",
	},
	{ // token for team
		"addr":     "b1ck4dcMF8GBCk1okTG4U41TCE2EnY3dG9X",
		"value":    "112500000",
		"locktime": "63072000",
	},
	{ // token for team
		"addr":     "b1eFzqrMJ6HA1gLAQ4T7c9ZepnCxBPfza9y",
		"value":    "112500000",
		"locktime": "94608000",
	},
	{
		"addr":  "b1fc1Vzz73WvBtzNQNbBSrxNCUC1Zrbnq4m",
		"value": "330000000",
	},
	{
		"addr":  "b1fvVgBViefbz6JXtLjcrQZzY4qnMHCMwWy",
		"value": "330000000",
	},
	{
		"addr":  "b1g9okr2zpfGUQU9peBvWb2tmHxMJGxnKiD",
		"value": "330000000",
	},
	{
		"addr":  "b1hoWW5VYorvjBchj95LbBjq2uqXrayVWiu",
		"value": "330000000",
	},
	{
		"addr":  "b1iKrYVfrFq1g6xyyG3vSgGaiBYyNJx4No8",
		"value": "330000000",
	},
}

// TokenPreAllocation is for token preallocation
func TokenPreAllocation() ([]*types.Transaction, error) {

	txs := make([]*types.Transaction, len(tokenPreAllocation))
	for idx, v := range tokenPreAllocation {
		addr, err := types.NewAddress(v["addr"])
		if err != nil {
			return nil, err
		}
		pubkeyhash := addr.Hash()
		value, err := strconv.ParseUint(v["value"], 10, 64)
		if err != nil {
			return nil, err
		}

		var locktime int64
		if v["locktime"] != "" {
			locktime, err = strconv.ParseInt(v["locktime"], 10, 64)
			if err != nil {
				return nil, err
			}
		}
		coinbaseScriptSig := script.StandardCoinbaseSignatureScript(0)
		var pkScript script.Script
		if locktime == 0 {
			pkScript = *script.PayToPubKeyHashScript(pubkeyhash)
		} else {
			pkScript = *script.PayToPubKeyHashCLTVScript(pubkeyhash, locktime)
		}

		tx := &types.Transaction{
			Version: 1,
			Vin: []*types.TxIn{
				{
					PrevOutPoint: *types.NewNullOutPoint(),
					ScriptSig:    *coinbaseScriptSig,
					Sequence:     math.MaxUint32,
				},
			},
			Vout: []*types.TxOut{
				{
					Value:        value * core.DuPerBox,
					ScriptPubKey: pkScript,
				},
			},
		}
		txs[idx] = tx

	}
	return txs, nil
}
