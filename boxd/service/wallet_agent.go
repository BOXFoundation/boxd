package service

import "github.com/BOXFoundation/boxd/core/types"

type WalletAgent interface {
	Balance(address types.Address) (uint64, error)
	Utxos(address types.Address) (map[types.OutPoint]*types.UtxoWrap, error)
}
