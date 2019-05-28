// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package dpos

import "errors"

// Define err message
var (
	// dpos
	ErrNoLegalPowerToProduce            = errors.New("No legal power to produce block")
	ErrNotMyTurnToProduce               = errors.New("Not my turn to produce block")
	ErrWrongTimeToProduce               = errors.New("Wrong time to produce block")
	ErrNotFoundBookkeeper               = errors.New("Failed to find bookkeeper")
	ErrDuplicateSignUpTx                = errors.New("Duplicate sign up tx")
	ErrCandidateNotFound                = errors.New("Candidate not found")
	ErrRepeatedMintAtSameTime           = errors.New("Repeated produce block at same time")
	ErrFailedToVerifySign               = errors.New("Failed to verify sign block")
	ErrNotBookkeeperPeer                = errors.New("Invalid bookkeeper peer")
	ErrInvalidBookkeeperEpoch           = errors.New("Invalid bookkeeper epoch")
	ErrInvalidCandidateHash             = errors.New("Invalid candidate hash")
	ErrFailedToStoreCandidateContext    = errors.New("Failed to store candidate context")
	ErrCandidateIsAlreadyExist          = errors.New("The candidate is already exist")
	ErrInvalidRegisterCandidateOrVoteTx = errors.New("Invalid register candidate or vote tx")
	ErrCircleTxExistInDag               = errors.New("circle tx exist in dag")

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
