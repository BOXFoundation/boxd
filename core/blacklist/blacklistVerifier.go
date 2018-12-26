// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package blacklist

import (
	"github.com/BOXFoundation/boxd/boxd/eventbus"
	"github.com/BOXFoundation/boxd/core"
	"github.com/BOXFoundation/boxd/p2p"
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
	}
	bl.notifiee.SendMessageToPeer(p2p.BlacklistConfirmMsg, confirmMsg, msg.From())
	return nil
}
