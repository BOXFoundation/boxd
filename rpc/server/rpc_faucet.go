// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package rpc

import (
	"context"
	"os"
	"sync/atomic"
	"time"

	"github.com/BOXFoundation/boxd/boxd/service"
	"github.com/BOXFoundation/boxd/core"
	"github.com/BOXFoundation/boxd/core/txlogic"
	"github.com/BOXFoundation/boxd/core/types"
	rpcpb "github.com/BOXFoundation/boxd/rpc/pb"
	"github.com/BOXFoundation/boxd/rpc/rpcutil"
	acc "github.com/BOXFoundation/boxd/wallet/account"

	"google.golang.org/grpc/metadata"
)

func init() {
	RegisterServiceWithGatewayHandler(
		"faucet",
		registerFaucet,
		rpcpb.RegisterFaucetHandlerFromEndpoint,
	)
}

func registerFaucet(s *Server) {
	keyFile := s.cfg.Faucet.Keyfile
	if _, err := os.Stat(keyFile); os.IsNotExist(err) {
		return
	}
	account, err := acc.NewAccountFromFile(keyFile)
	if err != nil {
		logger.Warnf("rpc register faucet new account error: %s", err)
		return
	}
	if err := account.UnlockWithPassphrase("1"); err != nil {
		logger.Warnf("rpc register faucet unlock account error: %s", err)
		return
	}
	amountPerSec := s.cfg.Faucet.AmountPerSec
	if amountPerSec == 0 {
		amountPerSec = 10000
	}
	f := newFaucet(s.cfg.Faucet.WhiteList, s.GetTxHandler(), s.GetWalletAgent(),
		account, amountPerSec)
	rpcpb.RegisterFaucetServer(s.server, f)
}

type faucet struct {
	service.WalletAgent
	service.TxHandler
	refreshTimer  *time.Ticker
	whiteList     []string
	account       *acc.Account
	amountPerSec  uint64
	remainBalance uint64
}

func newFaucet(
	whiteLists []string, handler service.TxHandler, wa service.WalletAgent,
	account *acc.Account, amountPerSec uint64,
) *faucet {
	return &faucet{
		refreshTimer:  time.NewTicker(time.Second),
		whiteList:     whiteLists,
		TxHandler:     handler,
		WalletAgent:   wa,
		account:       account,
		amountPerSec:  amountPerSec,
		remainBalance: amountPerSec,
	}
}

func newClaimResp(code int32, msg string) *rpcpb.ClaimResp {
	return &rpcpb.ClaimResp{
		Code:    code,
		Message: msg,
	}
}

func (f *faucet) Claim(
	ctx context.Context, req *rpcpb.ClaimReq,
) (resp *rpcpb.ClaimResp, err error) {

	defer func() {
		if resp.Code != 0 {
			logger.Warnf("faucet claim %+v error: %s", req, resp.Message)
		} else {
			logger.Infof("faucet claim: %+v succeeded, response: %+v", req, resp)
		}
	}()

	inWhiteList := false
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return newClaimResp(-1, "unable to parse ip from context"), err
	}
	cliIPs := md["x-forwarded-for"]
Loop:
	for _, cliIP := range cliIPs {
		for _, v := range f.whiteList {
			if cliIP == v {
				inWhiteList = true
				break Loop
			}
		}
	}
	if !inWhiteList {
		return newClaimResp(-1, "unauthorized IP!"), err
	}
	if req.Amount == 0 {
		return newClaimResp(-1, "Amount must be more than 0 "), err
	}

	select {
	case <-f.refreshTimer.C:
		atomic.StoreUint64(&f.remainBalance, f.amountPerSec)
	default:
	}
	remain := atomic.LoadUint64(&f.remainBalance)
	if remain-req.Amount > remain {
		return newClaimResp(-1, "exceed max amount this second"), err
	}
	atomic.AddUint64(&f.remainBalance, ^uint64(req.Amount-1))
	addrPubHash, err := types.NewAddressFromPubKey(f.account.PrivateKey().PubKey())
	if err != nil {
		return newClaimResp(-1, err.Error()), nil
	}
	from, toAddr, amount := addrPubHash.Hash160(), req.Addr, req.Amount
	toAddress, err := types.NewAddress(toAddr)
	if err != nil {
		return newClaimResp(-1, "invalid receiver address"), err
	}
	tx, utxos, err := rpcutil.MakeUnsignedTx(f.WalletAgent,
		from, []*types.AddressHash{toAddress.Hash160()}, []uint64{amount}, core.TransferFee)
	if err != nil {
		return newClaimResp(-1, err.Error()), err
	}
	if err := txlogic.SignTxWithUtxos(tx, utxos, f.account); err != nil {
		return newClaimResp(-1, err.Error()), err
	}
	if err := f.ProcessTx(tx, core.BroadcastMode); err != nil {
		return newClaimResp(-1, err.Error()), err
	}
	resp = newClaimResp(0, "success")
	hash, _ := tx.TxHash()
	resp.Hash = hash.String()
	return resp, nil
}
