// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package rpc

import (
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"math/rand"
	"testing"
	"time"

	"github.com/BOXFoundation/boxd/core/txlogic"
	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/crypto"
	"github.com/BOXFoundation/boxd/rpc/pb"
	"github.com/BOXFoundation/boxd/rpc/rpcutil"
)

type TestWalletAgent struct {
}

func (twa *TestWalletAgent) Balance(addr string, tid *types.TokenID) (uint64, error) {
	return 0, nil
}

func (twa *TestWalletAgent) Utxos(
	addr string, tid *types.TokenID, amount uint64,
) (utxos []*rpcpb.Utxo, err error) {

	count := uint64(2)
	ave := amount / count

	for h, remain := uint32(0), amount; remain <= amount; h++ {
		value := ave/2 + uint64(rand.Intn(int(ave)/2))
		// outpoint
		hash := hashFromUint64(uint64(time.Now().UnixNano()))
		op := types.NewOutPoint(&hash, h%10)
		// utxo wrap
		uw := txlogic.NewUtxoWrap(addr, h, value)
		utxo := txlogic.MakePbUtxo(op, uw)
		remain -= value
		utxos = append(utxos, utxo)
	}
	return
}

func TestMakeUnsignedTx(t *testing.T) {
	from := "b1ndoQmEd83y4Fza5PzbUQDYpT3mV772J5o"
	to := []string{
		"b1jh8DSdB6kB7N7RanrudV1hzzMCCcoX6L7",
		"b1UP5pbfJgZrF1ezoSHLdvkxvgF2BYLtGva",
	}
	amounts := []uint64{24000, 38000}
	fee := uint64(1000)

	wa := new(TestWalletAgent)
	tx, utxos, err := rpcutil.MakeUnsignedTx(wa, from, to, amounts, fee)
	if err != nil {
		t.Fatal(err)
	}
	rawMsgs, err := MakeTxRawMsgsForSign(tx, utxos...)
	if err != nil {
		t.Fatal(err)
	}
	txBytes, _ := json.MarshalIndent(tx, "", "  ")
	t.Logf("tx:\n%s", string(txBytes))
	for _, m := range rawMsgs {
		t.Logf("%s", hex.EncodeToString(m))
	}
}

func hashFromUint64(n uint64) crypto.HashType {
	bs := make([]byte, 8)
	binary.LittleEndian.PutUint64(bs, n)
	return crypto.DoubleHashH(bs)
}
