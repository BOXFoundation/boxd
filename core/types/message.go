// Copyright 2016 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package types

// Message is a fully derived transaction
//
// NOTE: In a future PR this will be removed.
// type VmMessage struct {
// 	from       AddressHash
// 	to         *AddressHash
// 	nonce      uint64
// 	amount     *big.Int
// 	gasLimit   uint64
// 	gasPrice   *big.Int
// 	data       []byte
// 	checkNonce bool
// }

// func NewTxMessage(from AddressHash, to *AddressHash, nonce uint64, amount *big.Int, gasLimit uint64, gasPrice *big.Int, data []byte, checkNonce bool) *VmMessage {
// 	return &VmMessage{
// 		from:       from,
// 		to:         to,
// 		nonce:      nonce,
// 		amount:     amount,
// 		gasLimit:   gasLimit,
// 		gasPrice:   gasPrice,
// 		data:       data,
// 		checkNonce: checkNonce,
// 	}
// }

// func (m VmMessage) From() AddressHash  { return m.from }
// func (m VmMessage) To() *AddressHash   { return m.to }
// func (m VmMessage) GasPrice() *big.Int { return m.gasPrice }
// func (m VmMessage) Value() *big.Int    { return m.amount }
// func (m VmMessage) Gas() uint64        { return m.gasLimit }
// func (m VmMessage) Nonce() uint64      { return m.nonce }
// func (m VmMessage) Data() []byte       { return m.data }
// func (m VmMessage) CheckNonce() bool   { return m.checkNonce }
