// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package types

import (
	"bytes"

	"github.com/BOXFoundation/boxd/util"
)

// RegisterCandidateContent identify the tx of RegisterCandidate type
type RegisterCandidateContent struct {
	addr AddressHash
}

// Marshal marshals the RegisterCandidateContent to a binary representation of it.
func (sc *RegisterCandidateContent) Marshal() (data []byte, err error) {

	var w bytes.Buffer
	if err := util.WriteVarBytes(&w, sc.addr[:]); err != nil {
		return nil, err
	}

	return w.Bytes(), nil
}

// Unmarshal unmarshals RegisterCandidateContent from binary data.
func (sc *RegisterCandidateContent) Unmarshal(data []byte) error {

	var r = bytes.NewBuffer(data)
	varbytes, err := util.ReadVarBytes(r)
	if err != nil {
		return err
	}
	copy(sc.addr[:], varbytes)

	return nil
}

// Addr returns addr in registerCandidateContent.
func (sc *RegisterCandidateContent) Addr() AddressHash {
	return sc.addr
}

// VoteContent identify the tx of vote type
type VoteContent struct {
	addr  AddressHash
	votes int64
}

// Marshal marshals the VoteContent to a binary representation of it.
func (vc *VoteContent) Marshal() (data []byte, err error) {

	var w bytes.Buffer
	if err := util.WriteVarBytes(&w, vc.addr[:]); err != nil {
		return nil, err
	}
	if err := util.WriteInt64(&w, vc.votes); err != nil {
		return nil, err
	}

	return w.Bytes(), nil
}

// Unmarshal unmarshals VoteContent from binary data.
func (vc *VoteContent) Unmarshal(data []byte) error {
	var r = bytes.NewBuffer(data)
	varbytes, err := util.ReadVarBytes(r)
	if err != nil {
		return err
	}
	copy(vc.addr[:], varbytes)
	if vc.votes, err = util.ReadInt64(r); err != nil {
		return err
	}

	return nil
}

// Addr returns addr in voteContent.
func (vc *VoteContent) Addr() AddressHash {
	return vc.addr
}

// Votes returns votes in voteContent.
func (vc *VoteContent) Votes() int64 {
	return vc.votes
}
