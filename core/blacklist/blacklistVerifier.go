// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package blacklist

import (
	"github.com/BOXFoundation/boxd/boxd/eventbus"
	"github.com/BOXFoundation/boxd/p2p"
)

func (bl *BlackList) processBlacklistMsg(msg p2p.Message) error {

	blMsg := new(BlacklistMsg)
	if err := blMsg.Unmarshal(msg.Body()); err != nil {
		return err
	}

	resultCh := make(chan error)
	for _, evidence := range blMsg.evidences {
		switch evidence.Type {
		case BlockEvidence:
			bl.bus.Send(eventbus.TopicBlacklistBlockConfirmResult, evidence.Block, p2p.DefaultMode, false, nil, resultCh)
			if result := <-resultCh; result == nil || result.Error() == 
		case TxEvidence:
			bl.bus.Send(eventbus.TopicBlacklistTxConfirmResult, )
		}
	}

	return nil
}
