// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package chain

import (
	"math"
	"strconv"
	"time"

	"github.com/BOXFoundation/boxd/core"
	"github.com/BOXFoundation/boxd/core/pb"
	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/crypto"
	"github.com/BOXFoundation/boxd/script"
)

// GenesisBlock represents genesis block.
var GenesisBlock = types.Block{
	Header: &types.BlockHeader{
		Version:       1,
		PrevBlockHash: crypto.HashType{}, // 0000000000000000000000000000000000000000000000000000000000000000
		TimeStamp:     time.Date(2018, 1, 31, 0, 0, 0, 0, time.UTC).Unix(),
	},
	Height: 0,
}

// GenesisPeriod genesis period
var GenesisPeriod = []map[string]string{
	{
		"addr":   "b1YaxM6pR9vWejxbEJ8cmHhebPcLHyKZn2F",
		"peerID": "12D3KooWPWfagC42R1DjpB8VMaQbwdqNLWtXwzhS1UuVuYHVjGc8",
	},
	{
		"addr":   "b1hE3LghriqzGnuuFtSNWNKRiwEGZACQwyx",
		"peerID": "12D3KooWSTL8TCZpRc6WjTZYtmzbAiDCbEUDuw9VzXCokSFER1v7",
	},
	// {
	// 	"addr":   "b1gKxBmynDZCEdcQr135vWDKswkB21ta7v4",
	// 	"peerID": "12D3KooWHvu1dRM7iagz5GVzAW7zAjmPYQWD8XqjSjW55H9xv8Xw",
	// },
	{
		"addr":   "b1VGrGhmraZ1A96S9qj5FaWWgGgi7bsjY9P",
		"peerID": "12D3KooWPpJsKMsCgu9UpU2KH9PU1un37HZo81mhiMYs7Zkem8qC",
	},
	{
		"addr":   "b1aFeMqvL9AzWoxPi1ojPDM9veTrgBC9fg8",
		"peerID": "12D3KooWHWVZpXoXQqM5F6kLzdxdD2mStRn34UiKccdmtLTyU8iN",
	},
	// {
	// 	"addr":   "b1aNTJAaTcUJPKoUL3tR7kwmZfWYTViSDgN",
	// 	"peerID": "12D3KooWE5XUq1CvyR38cJeq6Esxxo6WSK5ZhZZ7jtz5grJ8jiN6",
	// },
	{
		"addr":   "b1f3WDbds2vypDx1vzHiRnhX1mCEAdUYPFP",
		"peerID": "12D3KooWPXieg5ZEUzbHaKNFMsL3DxxUxoHytuojrKJk74G7weRg",
	},
	{
		"addr":   "b1qLJXfr3xcAL85LKGZbkpvDcfRFiUbg75g",
		"peerID": "12D3KooWEf6zh21ATMAF3JGhyCNJPreJUmUYiB4uchMoqKgjDjCi",
	},
	// {
	// 	"addr":   "b1s2NaBUECME7UeGFXtsvFK5zEnCNjEbe3s",
	// 	"peerID": "12D3KooWAEn2334gQrZKA5r8LKrGq2Hu5Q1V2w8PaqMNGeutgQEB",
	// },
	{
		"addr":   "b1qW9wwyd1VDN75Mao9e8k7nVVVmcc6mJ7w",
		"peerID": "12D3KooWGHyrGhQkPY6nUSnstYwuNFs5CmytofQfdkiR8DLygbrB",
	},
	{
		"addr":   "b1iGd3kcZKnT7zPf6k48j8juv8pRMe8Fw65",
		"peerID": "12D3KooWGUK3nyQwm7Wew3HWu6jX8vB1CyqUqQ7u9wNYUcF3FCXC",
	},
	// {
	// 	"addr":   "b1i7qiS4LBcnDALW3hTMgyk8TixTtv5YjFH",
	// 	"peerID": "12D3KooWQhafjFnktQ5MoFveg1eWWRnfWMHa9NQLmorShMiChNGj",
	// },
	{
		"addr":   "b1nyv68asDNhCUudpF2bvELyBPzEEoQU5KC",
		"peerID": "12D3KooWPkoxuH9sfmfBwBHxn7YgxFSfQAPBms9p1eWPd5BGh2T6",
	},
	{
		"addr":   "b1mDDEWqQnX4yPLq6UDCfXX8LVGhTYHDMB7",
		"peerID": "12D3KooWCy9vMmStynZJuqJ8Yj1r6XaVADtqBAfNWe4De4WDaHR7",
	},
	// {
	// 	"addr":   "b1Y8L4i6cDkBrKGf5kUYsqsLAn8AeuXCTfH",
	// 	"peerID": "12D3KooWLjPyrpnKYxFPkhi8gDQydLzhfF1kXxnSZxhuzztw1Tn1",
	// },
	{
		"addr":   "b1mpGAYQjwSWoDzuXnZqCW9RaRBjKHMCVEx",
		"peerID": "12D3KooWEUHqAhaaSZrG9R8p85XnVJC4e3iz4dM26Z1LQfZsMu7F",
	},
	{
		"addr":   "b1g8TPMT4mFZEcDZWomPJWXit4suGHs1eoq",
		"peerID": "12D3KooWS9afhAEkmT437oNkPBGkNLHJ9EkL5r4h2aXc6bsZgaTD",
	},
	// {
	// 	"addr":   "b1ewgzryAuFmxLnktczToEx8C6KcuxqNSgX",
	// 	"peerID": "12D3KooWAhyTwV6CaQHaRXbMZdm5JP4TqVqgha7Pfjo1FkK9cknV",
	// },
	{
		"addr":   "b1UTAV44cCeCF1SZc23szgHxFiMMbma4PHB",
		"peerID": "12D3KooWNANjisCyouVwekviY7hodpyBJrT5YWsi1xSMYbtDAWhN",
	},
	{
		"addr":   "b1mr7Rk5fQkS3avFDtQpKhzPak87fzZu6ta",
		"peerID": "12D3KooWAMTzTN4DuJck63sfQvRjp4jT1hjoYF2ucbGEvwoy2pRK",
	},
	{
		"addr":   "b1k9dMK6xrsXYCBuJfjTN9eTsMCiRb18Hnt",
		"peerID": "12D3KooWCZVcNiuADTbzQwjNgj5hzbCpQJPo3JkBXwafXSggx4AG",
	},
}

// tokenPreAllocation token pre_allocation
// total 3 billion, 2.1 billion pre_allocation and 0.9 billion pay for bookkeeper.
// 0.45 billion for team, unlocked in four years, 112.5 million per year.
var tokenPreAllocation = []map[string]string{
	{ // token for team
		"addr":  "b1juygrf1XAcjmSFUMpXytvwn6WHnVkGt1u",
		"value": "112500000",
	},
	{ // token for team
		"addr":     "b1ZSfM34fQDWr22WRuRjusgLdrSPvon4vQt",
		"value":    "112500000",
		"locktime": "31536000",
	},
	{ // token for team
		"addr":     "b1aNEP5JmGajhCUtro67yiYTG3KeFajuiaf",
		"value":    "112500000",
		"locktime": "63072000",
	},
	{ // token for team
		"addr":     "b1V8qzFaMDQjG4G7a1HfHf1VGAfTmyLkngG",
		"value":    "112500000",
		"locktime": "94608000",
	},
	{
		"addr":  "b1fc1Vzz73WvBtzNQNbBSrxNCUC1Zrbnq4m",
		"value": "330000000",
	},
	{
		"addr":  "b1gXTJfPrNV6uSUabwWLZiMRKSLTiMpprkR",
		"value": "330000000",
	},
	{
		"addr":  "b1oFwB6e7khBNN5Yvv9vFSvGPdpRKuG8bVc",
		"value": "330000000",
	},
	{
		"addr":  "b1daU7C5eBWyiRrdjuT18AujBKrk6sDPtYm",
		"value": "330000000",
	},
	{
		"addr":  "b1diXub4Eufncsqp4uSWrpCzgwBZEdWs6qq",
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
					PrevOutPoint: types.OutPoint{
						Hash:  zeroHash,
						Index: math.MaxUint32,
					},
					ScriptSig: *coinbaseScriptSig,
					Sequence:  math.MaxUint32,
				},
			},
			Vout: []*corepb.TxOut{
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
