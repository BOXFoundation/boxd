// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package controller

import (
	"bytes"
	"fmt"
	"hash/crc32"
	"sync"
	"time"

	"github.com/BOXFoundation/boxd/boxd/eventbus"
	"github.com/BOXFoundation/boxd/core"
	corepb "github.com/BOXFoundation/boxd/core/pb"
	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/crypto"
	"github.com/BOXFoundation/boxd/log"
	"github.com/BOXFoundation/boxd/p2p"
	"github.com/BOXFoundation/boxd/script"
	"github.com/BOXFoundation/boxd/storage"
	"github.com/BOXFoundation/boxd/storage/key"
	"github.com/BOXFoundation/boxd/util"
	lru "github.com/hashicorp/golang-lru"
	"github.com/jbenet/goprocess"
	peer "github.com/libp2p/go-libp2p-peer"
)

var logger = log.NewLogger("blacklist") // logger

// TODO: add into core config
const (
	TxEvidence uint32 = iota
	BlockEvidence

	// evidence useful life
	blackListPeriod    = 3 * time.Second
	blackListThreshold = 100

	txEvidenceChMaxSize    = 100
	blockEvidenceChMaxSize = 3
	BlMsgChBufferSize      = 5
	MaxConfirmMsgCacheTime = 5
)

var (
	blacklistBase = key.NewKey("/bl")

	blackListWrap *BlackListWrap

	// periodSize is a clone of consensus.periodSize
	periodSize int
)

// BlackListWrap represents the black list of public keys
type BlackListWrap struct {
	// Main part of blacklist.
	// checksumIEEE(pubKey) -> int64(expire time)
	Details *sync.Map

	// Collecting evidence from various sources.
	// All scene named variables in blacklist logic represent
	// the invalid transaction or block that caused the evidence.
	SceneCh chan *Evidence

	// Each entry contains two chs now, first is for txs and second for blocks.
	// When one of them reach a certain len, the blacklist judgment logic will start.
	// checksumIEEE(pubKey) -> []ch
	evidenceNote *lru.Cache

	// Save the signatures of each blacklist msg hash received from all pbs.
	// checkousum(hash) -> [][]byte([]signature)
	confirmMsgNote *lru.Cache

	// Preserve recent signed blacklist msg.
	// checkousum(hash) -> struct{}{}
	existConfirmedKey *lru.Cache

	db       storage.Table
	bus      eventbus.Bus
	notifiee p2p.Net
	msgCh    chan p2p.Message
	proc     goprocess.Process
	mutex    *sync.Mutex
}

// Evidence can help bp to restore error scene
type Evidence struct {
	PubKey []byte
	Tx     *types.Transaction
	Block  *types.Block
	Type   uint32
	Err    string
	Ts     int64
}

// Default returns the default BlackListWrap.
func Default() *BlackListWrap {
	return blackListWrap
}

// SetPeriodSize get a clone from consensus
func SetPeriodSize(size int) {
	periodSize = size
}

// NewBlacklistWrap return a singleton wrap
func NewBlacklistWrap(notifiee p2p.Net, bus eventbus.Bus, db storage.Table, parent goprocess.Process) *BlackListWrap {

	if blackListWrap != nil {
		return blackListWrap
	}

	blackListWrap = &BlackListWrap{
		Details: new(sync.Map),
		SceneCh: make(chan *Evidence, 4096),
		msgCh:   make(chan p2p.Message, BlMsgChBufferSize),
		mutex:   &sync.Mutex{},
	}
	blackListWrap.evidenceNote, _ = lru.New(4096)
	blackListWrap.confirmMsgNote, _ = lru.New(1024)
	blackListWrap.existConfirmedKey, _ = lru.New(1024)

	blackListWrap.bus = bus
	blackListWrap.notifiee = notifiee
	blackListWrap.db = db

	blackListWrap.loadBlacklistFromDb()
	blackListWrap.subscribeMessageNotifiee()

	blackListWrap.proc = parent.Go(func(p goprocess.Process) {
		logger.Info("Start blacklist loop")
		for {
			select {
			case msg := <-blackListWrap.msgCh:
				switch msg.Code() {
				case p2p.BlacklistMsg:
					if err := blackListWrap.onBlacklistMsg(msg); err != nil {
						logger.Errorf("Process blacklist msg fail. %v", err)
					}
				case p2p.BlacklistConfirmMsg:
					if err := blackListWrap.onBlacklistConfirmMsg(msg); err != nil {
						logger.Errorf("Process blacklist confirm msg fail. %v", err)
					}
				}
			case evidence := <-blackListWrap.SceneCh:
				if err := blackListWrap.processEvidence(evidence); err != nil {
					logger.Errorf("Process evidence fail. %v", err)
				}
			case <-parent.Closing():
				logger.Info("Stopped black list loop.")
				return
			}
		}
	})
	return blackListWrap
}

func (bl *BlackListWrap) processEvidence(evidence *Evidence) error {

	key := crc32.ChecksumIEEE(evidence.PubKey)
	// get personal note
	personalNote, ok := bl.evidenceNote.Get(key)
	if !ok {
		newNote := make([]chan *Evidence, 2)
		// store invalid txs
		newNote[0] = make(chan *Evidence, txEvidenceChMaxSize+1)
		// store invalid blocks
		newNote[1] = make(chan *Evidence, blockEvidenceChMaxSize+1)
		bl.evidenceNote.Add(key, newNote)
		personalNote, _ = bl.evidenceNote.Get(key)
	}

	// get pioneer
	var first *Evidence
	var evidenceCh chan *Evidence
	switch evidence.Type {
	case TxEvidence:
		evidenceCh = personalNote.([]chan *Evidence)[0]
		if len(evidenceCh) >= txEvidenceChMaxSize {
			first = <-evidenceCh
		} else {
			evidenceCh <- evidence
			return nil
		}
	case BlockEvidence:
		evidenceCh = personalNote.([]chan *Evidence)[1]
		if len(evidenceCh) >= blockEvidenceChMaxSize {
			first = <-evidenceCh
		} else {
			evidenceCh <- evidence
			return nil
		}
	default:
		logger.Errorf("invalid evidence type: %v", evidence.Type)
		return nil
	}
	evidenceCh <- evidence
	if first == nil || time.Unix(int64(first.Ts), 0).Add(blackListPeriod).Before(time.Unix(int64(evidence.Ts), 0)) {
		return nil
	}
	eviPackege := bl.packageEvidences(first, evidenceCh)
	blm := &BlacklistMsg{evidences: eviPackege}
	hash, err := blm.calcHash()
	if err != nil {
		return err
	}
	blm.hash = hash
	bl.notifiee.BroadcastToMiners(p2p.BlacklistMsg, blm)
	return nil
}

func (bl *BlackListWrap) packageEvidences(first *Evidence, evidenceCh chan *Evidence) []*Evidence {
	evidences := []*Evidence{first}
	for len(evidenceCh) != 0 {
		evidences = append(evidences, <-evidenceCh)
	}
	return evidences
}

func (bl *BlackListWrap) subscribeMessageNotifiee() {
	bl.notifiee.Subscribe(p2p.NewNotifiee(p2p.BlacklistMsg, bl.msgCh))
	bl.notifiee.Subscribe(p2p.NewNotifiee(p2p.BlacklistConfirmMsg, bl.msgCh))
}

// StoreContext save new blacklist item
func (bl *BlackListWrap) StoreContext(block *types.Block, batch storage.Batch) error {
	for _, tx := range block.Txs {
		if err := bl.processBlacklistTx(tx, batch); err != nil {
			return err
		}
	}
	return nil
}

func (bl *BlackListWrap) processBlacklistTx(tx *types.Transaction, batch storage.Batch) error {

	if tx.Data == nil || tx.Data.Type != types.BlacklistTx {
		return nil
	}

	blacklistContent := new(BlacklistTxData)
	if err := blacklistContent.Unmarshal(tx.Data.Content); err != nil {
		return err
	}
	expire := time.Now().Add(24 * time.Hour).Unix()
	bl.Details.Store(crc32.ChecksumIEEE(blacklistContent.pubkey), expire)
	batch.Put(BlacklistKey(blacklistContent.pubkey), util.FromInt64(expire))

	return nil
}

func (bl *BlackListWrap) loadBlacklistFromDb() {
	keys := bl.db.KeysWithPrefix(blacklistBase.Bytes())
	for _, k := range keys {
		slice := key.NewKeyFromBytes(k).List()
		if len(slice) == 2 {
			val, err := bl.db.Get(k)
			if err != nil {
				logger.Errorf("db get fail. Err: %v", err)
				continue
			}
			expire := util.Int64(val)
			if expire <= time.Now().Unix() {
				continue
			}
			pubkey := slice[1]
			bl.Details.Store(crc32.ChecksumIEEE([]byte(pubkey)), expire)
		}
	}
}

func (bl *BlackListWrap) onBlacklistMsg(msg p2p.Message) error {
	blMsg := new(BlacklistMsg)
	if err := blMsg.Unmarshal(msg.Body()); err != nil {
		return err
	}
	if err := blMsg.validateHash(); err != nil {
		return err
	}

	pubkey, err := bl.validateEvidences(blMsg)
	if err != nil {
		return err
	}

	signCh := make(chan []byte)
	bl.bus.Send(eventbus.TopicSignature, blMsg.hash, signCh)
	signature := <-signCh
	if signature == nil {
		return core.ErrSign
	}

	confirmMsg := &BlacklistConfirmMsg{
		pubkey:    pubkey,
		hash:      blMsg.hash,
		signature: signature,
		timestamp: time.Now().Unix(),
	}

	bl.notifiee.SendMessageToPeer(p2p.BlacklistConfirmMsg, confirmMsg, msg.From())
	return nil
}

func (bl *BlackListWrap) validateEvidences(blMsg *BlacklistMsg) ([]byte, error) {
	var pubkey []byte
	resultCh := make(chan error)
	for _, evidence := range blMsg.evidences {
		switch evidence.Type {
		case BlockEvidence:
			// Param peer.ID in processBlock is used for light sync.
			// So we neednt to provide it.
			bl.bus.Send(eventbus.TopicBlacklistBlockConfirmResult, evidence.Block, peer.ID(""), resultCh)
		case TxEvidence:
			bl.bus.Send(eventbus.TopicBlacklistTxConfirmResult, evidence.Tx, resultCh)
		default:
			return nil, core.ErrInvalidEvidenceType
		}
		if result := <-resultCh; result == nil || result.Error() != evidence.Err {
			return nil, core.ErrEvidenceErrNotMatch
		}

		if pubkey == nil || len(pubkey) == 0 {
			pubkey = make([]byte, len(evidence.PubKey))
			copy(pubkey[:], evidence.PubKey)
		} else if !bytes.Equal(pubkey, evidence.PubKey) {
			return nil, core.ErrSeparateSourceEvidences
		}

		// Guarantee that all evidence is of the same origin.
		switch evidence.Type {
		case BlockEvidence:
			if scenePubkey, ok := crypto.RecoverCompact(evidence.Block.BlockHash()[:], evidence.Block.Signature); !ok || !bytes.Equal(pubkey, scenePubkey.Serialize()) {
				return nil, core.ErrSeparateSourceEvidences
			}
		case TxEvidence:
			if scenePubkey, ok := script.NewScriptFromBytes(evidence.Tx.Vout[0].ScriptPubKey).GetPubKey(); !ok || !bytes.Equal(pubkey, scenePubkey) {
				return nil, core.ErrSeparateSourceEvidences
			}
		}
	}
	return pubkey, nil
}

func (bl *BlackListWrap) onBlacklistConfirmMsg(msg p2p.Message) error {

	confirmMsg := new(BlacklistConfirmMsg)
	if err := confirmMsg.Unmarshal(msg.Body()); err != nil {
		return err
	}

	if bl.existConfirmedKey.Contains(crc32.ChecksumIEEE(confirmMsg.hash)) {
		logger.Debugf("Enough confirmMsgs had been received.")
		return nil
	}

	now := time.Now().Unix()
	if confirmMsg.timestamp > now || now-confirmMsg.timestamp > MaxConfirmMsgCacheTime {
		return core.ErrIllegalMsg
	}

	pubkey, ok := crypto.RecoverCompact(confirmMsg.hash[:], confirmMsg.signature)
	if !ok {
		return core.ErrSign
	}
	addrPubKeyHash, err := types.NewAddressFromPubKey(pubkey)
	if err != nil {
		return err
	}
	addr := *addrPubKeyHash.Hash160()

	minersValidateCh := make(chan bool)
	bl.bus.Send(eventbus.TopicValidateMiner, msg.From().Pretty(), addr, minersValidateCh)
	if !<-minersValidateCh {
		return core.ErrIllegalMsg
	}

	hashChecksum := crc32.ChecksumIEEE(confirmMsg.hash)
	bl.mutex.Lock()
	if sigs, ok := bl.confirmMsgNote.Get(hashChecksum); ok {
		sigSlice := sigs.([][]byte)
		if util.InArray(confirmMsg.signature, sigSlice) {
			return nil
		}
		sigSlice = append(sigSlice, confirmMsg.signature)
		if len(sigSlice) > 2*periodSize/3 {
			bl.existConfirmedKey.Add(hashChecksum, struct{}{})
			go func() {
				bl.doBlacklistTx(confirmMsg.pubkey, confirmMsg.hash, sigSlice)
				bl.confirmMsgNote.Remove(hashChecksum)
			}()
		} else {
			bl.confirmMsgNote.Add(hashChecksum, sigSlice)
		}
	} else {
		bl.confirmMsgNote.Add(hashChecksum, [][]byte{confirmMsg.signature})
	}
	bl.mutex.Unlock()
	return nil
}

func (bl *BlackListWrap) doBlacklistTx(pubkey, hash []byte, signs [][]byte) error {
	tx, err := CreateBlacklistTx(&BlacklistTxData{
		pubkey:     pubkey,
		hash:       hash,
		signatures: signs,
	})
	if err != nil {
		return err
	}

	resultCh := make(chan error)
	bl.bus.Send(eventbus.TopicBlacklistTxConfirmResult, tx, resultCh)
	if err = <-resultCh; err != nil {
		logger.Errorf("tx send fail %v", err)
		return err
	}
	logger.Errorf("tx send success %v", tx)
	return nil
}

// CreateBlacklistTx creates blacklist type tx
func CreateBlacklistTx(txData *BlacklistTxData) (*types.Transaction, error) {

	data, err := txData.Marshal()
	if err != nil {
		return nil, err
	}
	txpbData := &corepb.Data{
		Type:    types.BlacklistTx,
		Content: data,
	}

	txCh := make(chan *types.Transaction)
	eventbus.Default().Send(eventbus.TopicGenerateTx, txpbData, txCh)
	tx := <-txCh

	return tx, nil
}

// BlacklistKey returns the db key to store blacklist pubkey
func BlacklistKey(pubkey []byte) []byte {
	return blacklistBase.ChildString(fmt.Sprintf("%x", pubkey)).Bytes()
}
