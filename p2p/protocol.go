// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package p2p

type BoxNet uint32

const (
	// Mainnet velocity of light
	Mainnet BoxNet = 0x11de784a
	Testnet BoxNet = 0x11de784a

	ProtocolID = "/box/1.0.0"
)
