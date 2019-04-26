package test

import (
	"math/big"

	"github.com/BOXFoundation/boxd/core/types"
)

type Message struct {
	from     *types.AddressHash
	gasPrice *big.Int
}

func NewMessage(from *types.AddressHash, gasPrice *big.Int) Message {
	return Message{
		from:     from,
		gasPrice: gasPrice,
	}
}

func (msg Message) From() *types.AddressHash { return msg.from }
func (msg Message) To() *types.AddressHash   { return &types.AddressHash{} }

func (msg Message) GasPrice() *big.Int { return msg.gasPrice }
func (msg Message) Gas() uint64        { return 0 }
func (msg Message) Value() *big.Int    { return big.NewInt(0) }

func (msg Message) Nonce() uint64    { return 0 }
func (msg Message) CheckNonce() bool { return true }
func (msg Message) Data() []byte     { return []byte{} }

func (msg Message) Type() types.ContractType { return types.ContractCreationType }
