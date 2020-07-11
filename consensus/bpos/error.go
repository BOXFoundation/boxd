// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package bpos

import "errors"

// Define err message
var (
	// bpos
	ErrNoLegalPowerToProduce     = errors.New("No legal power to produce block")
	ErrNotMyTurnToProduce        = errors.New("Not my turn to produce block")
	ErrNotFoundBookkeeper        = errors.New("Failed to find bookkeeper")
	ErrNotBookkeeperPeer         = errors.New("Invalid bookkeeper peer")
	ErrCircleTxExistInDag        = errors.New("circle tx exist in dag")
	ErrInvalidDynastyHash        = errors.New("Invalid dynasty hash in block header")
	ErrInvalidDynastySwitchTx    = errors.New("Invalid dynasty switch tx")
	ErrMultipleDynastySwitchTx   = errors.New("multiple dynasty switch tx")
	ErrDynastySwitchIsNotAllowed = errors.New("Dynasty switching is not allowed in the current block")
	ErrFailedToVerifySign        = errors.New("Failed to verify sign block")
	ErrInvalidCoinbaseTx         = errors.New("Invalid coinbase tx")
	ErrInvalidCalcScoreTx        = errors.New("Invalid calc score tx")

	// context
	ErrInvalidEternalBlockMsgProtoMessage = errors.New("Invalid eternalBlockMsg proto message")
	ErrInvalidDelegateProtoMessage        = errors.New("Invalid delegate proto message")
	ErrInvalidDynastyProtoMessage         = errors.New("Invalid dynasty proto message")

	// bft_service
	ErrNoNeedToUpdateEternalBlock = errors.New("No need to update Eternal block")
	ErrIllegalMsg                 = errors.New("Illegal message from remote peer")
	ErrEternalBlockMsgHashIsExist = errors.New("EternalBlockMsgHash is already exist")
)
