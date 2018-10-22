// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package pstore

import (
	"context"
	crand "crypto/rand"
	"testing"

	"github.com/BOXFoundation/boxd/storage"
	"github.com/facebookgo/ensure"
	goprocessctx "github.com/jbenet/goprocess/context"
	crypto "github.com/libp2p/go-libp2p-crypto"
	peer "github.com/libp2p/go-libp2p-peer"
)

type tkb struct {
	kb     *keyBook
	db     storage.Storage
	dbpath string
	ctx    context.Context
	cancel context.CancelFunc
}

func newKb() (*tkb, error) {
	dbpath, db, err := getDatabase()
	if err != nil {
		return nil, err
	}

	t, err := db.Table("peer")
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())
	p := goprocessctx.WithContext(ctx)
	kb, _ := NewKeyBook(p, t)

	return &tkb{
		kb:     kb.(*keyBook),
		db:     db,
		dbpath: dbpath,
		ctx:    ctx,
		cancel: cancel,
	}, nil
}

func releaseKb(b *tkb) {
	b.cancel()
	releaseDatabase(b.dbpath, b.db)
}

func genPriKeybook(tkb *tkb, count int) peer.IDSlice {
	var pids peer.IDSlice
	for i := 0; i < count; i++ {
		pri, _, _ := crypto.GenerateEd25519Key(crand.Reader)
		pid, _ := peer.IDFromPrivateKey(pri)
		tkb.kb.AddPrivKey(pid, pri)
		pids = append(pids, pid)
	}
	return pids
}

func genPubKeybook(tkb *tkb, count int) peer.IDSlice {
	var pids peer.IDSlice
	for i := 0; i < count; i++ {
		pri, pub, _ := crypto.GenerateEd25519Key(crand.Reader)
		pid, _ := peer.IDFromPrivateKey(pri)
		tkb.kb.AddPubKey(pid, pub)
		pids = append(pids, pid)
	}
	return pids
}

func TestKeyBookAddPri(t *testing.T) {
	tkb, err := newKb()
	ensure.Nil(t, err)
	defer releaseKb(tkb)

	pri, pub, _ := crypto.GenerateEd25519Key(crand.Reader)
	pid, _ := peer.IDFromPrivateKey(pri)
	ensure.Nil(t, tkb.kb.AddPrivKey(pid, pri))
	ensure.NotNil(t, tkb.kb.AddPrivKey(pid, nil))

	pri2, _, _ := crypto.GenerateEd25519Key(crand.Reader)
	ensure.NotNil(t, tkb.kb.AddPrivKey(pid, pri2))

	prikey := tkb.kb.PrivKey(pid)
	ensure.DeepEqual(t, prikey, pri)

	pubkey := tkb.kb.PubKey(pid)
	ensure.DeepEqual(t, pubkey, pub)
}

func TestKeyBookAddPub(t *testing.T) {
	tkb, err := newKb()
	ensure.Nil(t, err)
	defer releaseKb(tkb)

	pri, pub, _ := crypto.GenerateEd25519Key(crand.Reader)
	pid, _ := peer.IDFromPrivateKey(pri)
	ensure.Nil(t, tkb.kb.AddPubKey(pid, pub))

	// ensure.NotNil(t, tkb.kb.AddPubKey(pid, nil))

	_, pub2, _ := crypto.GenerateEd25519Key(crand.Reader)
	ensure.NotNil(t, tkb.kb.AddPubKey(pid, pub2))

	pubkey := tkb.kb.PubKey(pid)
	ensure.DeepEqual(t, pubkey, pub)

	prikey := tkb.kb.PrivKey(pid)
	ensure.Nil(t, prikey)
}

func TestKeyBookPeers(t *testing.T) {
	tkb, err := newKb()
	ensure.Nil(t, err)
	defer releaseKb(tkb)

	pubpids := genPubKeybook(tkb, 100)
	pripids := genPriKeybook(tkb, 100)

	peers := tkb.kb.PeersWithKeys()
	ensurePeersEqual(t, peers, append(pubpids, pripids...))
}
