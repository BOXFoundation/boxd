// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package dpos

import "errors"

// Define err message
var (
	// dpos
	ErrNoLegalPowerToMint     = errors.New("No legal power to mint")
	ErrNotMyTurnToMint        = errors.New("Not my turn to mint")
	ErrWrongTimeToMint        = errors.New("Wrong time to mint")
	ErrNotFoundMiner          = errors.New("Failed to find miner")
	ErrDuplicateSignUpTx      = errors.New("Duplicate sign up tx")
	ErrCandidateNotFound      = errors.New("Candidate not found")
	ErrRepeatedMintAtSameTime = errors.New("Repeated mint at same time")
	ErrFailedToVerifySign     = errors.New("Failed to verify sign block")
	ErrNotMintPeer            = errors.New("Invalid mint peer")
	ErrInvalidMinerEpoch      = errors.New("Invalid miner epoch")

	// context
	ErrInvalidCandidateProtoMessage        = errors.New("Invalid candidate proto message")
	ErrInvalidConsensusContextProtoMessage = errors.New("Invalid consensus context proto message")
	ErrInvalidCandidateContextProtoMessage = errors.New("Invalid condidate context proto message")
	ErrInvalidPeriodContextProtoMessage    = errors.New("Invalid period contex proto message")
	ErrInvalidPeriodProtoMessage           = errors.New("Invalid period proto message")
	ErrInvalidEternalBlockMsgProtoMessage  = errors.New("Invalid eternalBlockMsg proto message")

	// bft_service
	ErrNoNeedToUpdateEternalBlock = errors.New("No need to update Eternal block")
	ErrIllegalMsg                 = errors.New("Illegal message from remote peer")
	ErrEternalBlockMsgHashIsExist = errors.New("EternalBlockMsgHash is already exist")
)
