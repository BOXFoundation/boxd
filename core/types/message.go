// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

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
