// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package pstore

import (
	"errors"

	"github.com/BOXFoundation/boxd/storage"
	"github.com/BOXFoundation/boxd/storage/key"
	"github.com/jbenet/goprocess"
	ic "github.com/libp2p/go-libp2p-crypto"
	peer "github.com/libp2p/go-libp2p-peer"
	"github.com/libp2p/go-libp2p-peerstore"
)

// Public and private keys are stored under the following db key pattern:
// /peers/keys/<b58 peer id no padding>/{pub, priv}
var (
	kbBase     = key.NewKey("/peers/keys")
	pubSuffix  = key.NewKey("/pub")
	privSuffix = key.NewKey("/priv")
)

type keyBook struct {
	store storage.Table
}

var _ peerstore.KeyBook = (*keyBook)(nil)

// NewKeyBook creates a keybook using storage.Table
func NewKeyBook(_ goprocess.Process, store storage.Table) (peerstore.KeyBook, error) {
	return &keyBook{store}, nil
}

func (kb *keyBook) PubKey(p peer.ID) ic.PubKey {
	key := kbBase.ChildString(p.Pretty()).Child(pubSuffix)

	var pk ic.PubKey
	if value, err := kb.store.Get(key.Bytes()); err == nil {
		if len(value) > 0 {
			pk, err = ic.UnmarshalPublicKey(value)
			if err != nil {
				logger.Errorf("error when unmarshalling pubkey from datastore for peer %s: %s", p.Pretty(), err)
			}
		} else {
			pk, err = p.ExtractPublicKey()
			switch err {
			case nil:
			case peer.ErrNoPublicKey:
				return nil
			default:
				logger.Errorf("error when extracting pubkey from peer ID for peer %s: %s", p.Pretty(), err)
				return nil
			}
			pkb, err := pk.Bytes()
			if err != nil {
				logger.Errorf("error when turning extracted pubkey into bytes for peer %s: %s", p.Pretty(), err)
				return nil
			}
			err = kb.store.Put(key.Bytes(), pkb)
			if err != nil {
				logger.Errorf("error when adding extracted pubkey to peerstore for peer %s: %s", p.Pretty(), err)
				return nil
			}
		}
	} else {
		logger.Errorf("error when fetching pubkey from datastore for peer %s: %s", p.Pretty(), err)
	}

	return pk
}

func (kb *keyBook) AddPubKey(p peer.ID, pk ic.PubKey) error {
	// check it's correct.
	if !p.MatchesPublicKey(pk) {
		return errors.New("peer ID does not match public key")
	}

	key := kbBase.ChildString(p.Pretty()).Child(pubSuffix)
	val, err := pk.Bytes()
	if err != nil {
		logger.Errorf("error while converting pubkey byte string for peer %s: %s", p.Pretty(), err)
		return err
	}
	err = kb.store.Put(key.Bytes(), val)
	if err != nil {
		logger.Errorf("error while updating pubkey in datastore for peer %s: %s", p.Pretty(), err)
	}
	return err
}

func (kb *keyBook) PrivKey(p peer.ID) ic.PrivKey {
	key := kbBase.ChildString(p.Pretty()).Child(privSuffix)
	value, err := kb.store.Get(key.Bytes())
	if err != nil {
		logger.Errorf("error while fetching privkey from datastore for peer %s: %s\n", p.Pretty(), err)
		return nil
	}
	sk, err := ic.UnmarshalPrivateKey(value)
	if err != nil {
		return nil
	}
	return sk
}

func (kb *keyBook) AddPrivKey(p peer.ID, sk ic.PrivKey) error {
	if sk == nil {
		return errors.New("private key is nil")
	}
	// check it's correct.
	if !p.MatchesPrivateKey(sk) {
		return errors.New("peer ID does not match private key")
	}

	key := kbBase.ChildString(p.Pretty()).Child(privSuffix)
	val, err := sk.Bytes()
	if err != nil {
		logger.Errorf("error while converting privkey byte string for peer %s: %s\n", p.Pretty(), err)
		return err
	}
	return kb.store.Put(key.Bytes(), val)
}

func (kb *keyBook) PeersWithKeys() peer.IDSlice {
	ids, err := uniquePeerIDs(kb.store, kbBase.Bytes(), func(k key.Key) string {
		return k.Parent().BaseName()
	})
	if err != nil {
		logger.Errorf("error while retrieving peers with keys: %v", err)
	}
	return ids
}
