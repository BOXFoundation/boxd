// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package common

import (
	"encoding/hex"
	"fmt"
	"path"

	"github.com/BOXFoundation/boxd/config"
	corepb "github.com/BOXFoundation/boxd/core/pb"
	"github.com/BOXFoundation/boxd/crypto"
	rpcpb "github.com/BOXFoundation/boxd/rpc/pb"
	"github.com/BOXFoundation/boxd/rpc/rpcutil"
	"github.com/BOXFoundation/boxd/script"
	"github.com/BOXFoundation/boxd/util"
	"github.com/BOXFoundation/boxd/wallet"
	"github.com/BOXFoundation/boxd/wallet/account"
	"github.com/spf13/viper"
)

//
var (
	DefaultWalletDir = path.Join(util.HomeDir(), ".box_keystore")
)

// GetRPCAddr gets rpc addr
func GetRPCAddr() string {
	var cfg config.Config
	viper.Unmarshal(&cfg)
	return fmt.Sprintf("%s:%d", cfg.RPC.Address, cfg.RPC.Port)
}

// SignAndSendTx sign tx and then send this tx to a server node
func SignAndSendTx(
	tx *corepb.Transaction, rawMsgs []string, acc *account.Account,
) (hash string, err error) {
	sigHashes := make([]*crypto.HashType, 0, len(rawMsgs))
	for _, msg := range rawMsgs {
		msgHash := new(crypto.HashType)
		if err = msgHash.SetString(msg); err != nil {
			return
		}
		for i, j := 0, 0; i < j; i, j = i+1, j-1 {
			msgHash[i], msgHash[j] = msgHash[j], msgHash[i]
		}
		sigHashes = append(sigHashes, msgHash)
	}
	// sign msg
	passphrase, err := wallet.ReadPassphraseStdin()
	if err = acc.UnlockWithPassphrase(passphrase); err != nil {
		fmt.Println("Fail to unlock account: ")
		return
	}
	var scriptSigs [][]byte
	for _, sigHash := range sigHashes {
		var sig *crypto.Signature
		sig, err = acc.Sign(sigHash)
		if err != nil {
			fmt.Println("sign error: ")
			return
		}
		scriptSig := script.SignatureScript(sig, acc.PublicKey())
		scriptSigs = append(scriptSigs, *scriptSig)
	}
	// make tx with sig
	for i, sig := range scriptSigs {
		tx.Vin[i].ScriptSig = sig
	}
	// send tx
	resp, err := rpcutil.RPCCall(rpcpb.NewTransactionCommandClient,
		"SendTransaction", &rpcpb.SendTransactionReq{Tx: tx}, GetRPCAddr())
	if err != nil {
		err = fmt.Errorf("send tx %+v error: %s", tx, err)
		return
	}
	sendResp := resp.(*rpcpb.SendTransactionResp)
	if sendResp.Code != 0 {
		err = fmt.Errorf("send tx %+v error: %s", tx, sendResp.Message)
		return
	}
	return sendResp.GetHash(), nil
}

//IsHexFormat judge whether str is hex code
func IsHexFormat(str string) bool {
	if len(str) == 0 {
		return false
	}
	if _, err := hex.DecodeString(str); err != nil {
		return false
	}
	return true
}
