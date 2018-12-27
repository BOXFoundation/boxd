// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package blacklist

import (
	"time"

	"github.com/BOXFoundation/boxd/boxd/eventbus"
	"github.com/BOXFoundation/boxd/core"
	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/crypto"
	"github.com/BOXFoundation/boxd/p2p"
	"github.com/BOXFoundation/boxd/util"
)

func (bl *BlackList) onBlacklistMsg(msg p2p.Message) error {

	blMsg := new(BlacklistMsg)
	if err := blMsg.Unmarshal(msg.Body()); err != nil {
		return err
	}

	ok, err := blMsg.validate()
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
			bl.bus.Send(eventbus.TopicBlacklistBlockConfirmResult, evidence.Block, core.DefaultMode, false, nil, resultCh)
		case TxEvidence:
			bl.bus.Send(eventbus.TopicBlacklistTxConfirmResult, evidence.Tx, resultCh)
		default:
			return core.ErrInvalidEvidenceType
		}
		if result := <-resultCh; result == nil || result.Error() != evidence.Err {
			return core.ErrInsufficientEvidence
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

	if bl.existConfirmedKey.Contains(confirmMsg.hash) {
		logger.Debugf("Enough confirmMsgs had been received.")
		return nil
	}

	now := time.Now().Unix()
	if confirmMsg.timestamp > now || now-confirmMsg.timestamp > MaxConfirmMsgCacheTime {
		return core.ErrIllegalMsg
	}

	if pubkey, ok := crypto.RecoverCompact(confirmMsg.hash[:], confirmMsg.signature); ok {
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
	}

	bl.mutex.Lock()
	if sigs, ok := bl.confirmMsgNote.Get(confirmMsg.hash); ok {
		sigSlice := sigs.([][]byte)
		if util.InArray(confirmMsg.signature, sigSlice) {
			return nil
		}
		sigSlice = append(sigSlice, confirmMsg.signature)
		if len(sigSlice) > 2*periodSize/3 {
			bl.existConfirmedKey.Add(confirmMsg.hash, struct{}{})
			// TODO: 上链
			go func() {

			}()
		} else {
			bl.confirmMsgNote.Add(confirmMsg.hash, sigSlice)
		}
	} else {
		bl.confirmMsgNote.Add(confirmMsg.hash, [][]byte{confirmMsg.signature})
	}
	bl.mutex.Unlock()

	return nil
}
