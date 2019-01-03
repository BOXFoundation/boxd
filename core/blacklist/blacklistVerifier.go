// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package blacklist

import (
	"hash/crc32"
	"time"

	"github.com/BOXFoundation/boxd/boxd/eventbus"
	"github.com/BOXFoundation/boxd/core"
	"github.com/BOXFoundation/boxd/core/pb"
	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/crypto"
	"github.com/BOXFoundation/boxd/p2p"
	"github.com/BOXFoundation/boxd/util"
	peer "github.com/libp2p/go-libp2p-peer"
)

func (bl *BlackList) onBlacklistMsg(msg p2p.Message) error {

	blMsg := new(BlacklistMsg)
	if err := blMsg.Unmarshal(msg.Body()); err != nil {
		return err
	}

	ok, err := blMsg.validateHash()
	if err != nil {
		return err
	} else if !ok {
		return core.ErrInsufficientEvidence
	}

	var pubkeyChecksum uint32
	resultCh := make(chan error)
	for _, evidence := range blMsg.evidences {
		switch evidence.Type {
		case BlockEvidence:
			// TODO: params bug
			bl.bus.Send(eventbus.TopicBlacklistBlockConfirmResult, evidence.Block, peer.ID("nil"), resultCh)
		case TxEvidence:
			bl.bus.Send(eventbus.TopicBlacklistTxConfirmResult, evidence.Tx, resultCh)
		default:
			return core.ErrInvalidEvidenceType
		}
		if result := <-resultCh; result == nil || result.Error() != evidence.Err {
			return core.ErrEvidenceErrNotMatch
		}

		// TODO: 判断checksum是否match scene中的发送方（script/block）
		if pubkeyChecksum == 0 {
			pubkeyChecksum = evidence.PubKeyChecksum
		} else if pubkeyChecksum != evidence.PubKeyChecksum {
			return core.ErrSeparateSourceEvidences
		}
	}

	signCh := make(chan []byte)
	bl.bus.Send(eventbus.TopicSignature, blMsg.hash, signCh)
	signature := <-signCh

	if signature == nil {
		return core.ErrSign
	}

	confirmMsg := &BlacklistConfirmMsg{
		pubKeyChecksum: pubkeyChecksum,
		hash:           blMsg.hash,
		signature:      signature,
		timestamp:      time.Now().Unix(),
	}
	bl.notifiee.SendMessageToPeer(p2p.BlacklistConfirmMsg, confirmMsg, msg.From())
	return nil
}

func (bl *BlackList) onBlacklistConfirmMsg(msg p2p.Message) error {

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
			// TODO: 上链
			go func() {
				bl.createBlacklistTx(confirmMsg.hash, sigSlice)
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

func (bl *BlackList) createBlacklistTx(hash []byte, signs [][]byte) error {
	tx, err := CreateBlacklistTx(&BlacklistTxData{
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

	// pubkeyCh := make(chan []byte)
	// eventbus.Default().Send(eventbus.TopicMinerPubkey, pubkeyCh)
	// pubkey := <-pubkeyCh

	// var pkScript []byte
	// pkScript = *script.PayToPubKeyHashScript(pubkey)

	data, err := txData.Marshal()
	if err != nil {
		return nil, err
	}

	// tx := &types.Transaction{
	// 	Version: 1,
	// 	Vin: []*types.TxIn{
	// 		{
	// 			PrevOutPoint: types.OutPoint{
	// 				Hash:  zeroHash,
	// 				Index: 0,
	// 			},
	// 			ScriptSig: []byte{},
	// 			Sequence:  math.MaxUint32,
	// 		},
	// 	},
	// 	Vout: []*corepb.TxOut{
	// 		{
	// 			Value:        0,
	// 			ScriptPubKey: pkScript,
	// 		},
	// 	},
	// 	Data: &corepb.Data{
	// 		Type:    types.BlacklistTx,
	// 		Content: data,
	// 	},
	// }

	txpbData := &corepb.Data{
		Type:    types.BlacklistTx,
		Content: data,
	}

	txCh := make(chan *types.Transaction)
	eventbus.Default().Send(eventbus.TopicGenerateTx, txpbData, txCh)
	tx := <-txCh

	return tx, nil
}
