// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package client

import (
	"context"
	"fmt"
	"time"

	"github.com/BOXFoundation/boxd/core/pb"
	"github.com/BOXFoundation/boxd/core/txlogic"
	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/crypto"
	"github.com/BOXFoundation/boxd/rpc/pb"
	acc "github.com/BOXFoundation/boxd/wallet/account"
	"google.golang.org/grpc"
)

const (
	connTimeout = 30
	maxTokenFee = 1000
)

// GetBalance returns total amount of an address
func GetBalance(conn *grpc.ClientConn, addresses []string) ([]uint64, error) {
	c := rpcpb.NewTransactionCommandClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), connTimeout*time.Second)
	defer cancel()
	req, err := c.GetBalance(ctx, &rpcpb.GetBalanceReq{Addrs: addresses})
	if err != nil {
		return nil, err
	}
	return req.GetBalances(), nil
}

// FetchUtxos fetch utxos from chain
func FetchUtxos(conn *grpc.ClientConn, addr string, amount uint64) ([]*rpcpb.Utxo, error) {
	c := rpcpb.NewTransactionCommandClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), connTimeout*time.Second)
	defer cancel()
	req, err := c.FetchUtxos(ctx, &rpcpb.FetchUtxosReq{Addr: addr, Amount: amount})
	if err != nil {
		return nil, err
	}
	return req.GetUtxos(), nil
}

// GetFeePrice gets the recommended mining fee price according to recent packed transactions
func GetFeePrice(conn *grpc.ClientConn) (uint64, error) {
	c := rpcpb.NewTransactionCommandClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), connTimeout*time.Second)
	defer cancel()
	r, err := c.GetFeePrice(ctx, &rpcpb.GetFeePriceRequest{})
	return r.BoxPerByte, err
}

// NewIssueTokenTx new a issue token transaction
func NewIssueTokenTx(conn *grpc.ClientConn, acc *acc.Account, to string,
	tag *txlogic.TokenTag, supply uint64) (
	*types.Transaction, *txlogic.TokenID, *rpcpb.Utxo, error) {

	// fetch utxos for fee
	inputAmt := uint64(0)
	utxos, err := FetchUtxos(conn, acc.Addr(), maxTokenFee)
	if err != nil {
		return nil, nil, nil, err
	}
	for _, u := range utxos {
		inputAmt += u.GetTxOut().GetValue()
	}
	// fee
	fee := calcFee()
	//
	tx, tid, change, err := txlogic.NewIssueTokenTxWithUtxos(acc, utxos, to, tag, supply,
		inputAmt-fee)
	if err != nil {
		logger.Warnf("new issue token tx with utxos from %s to %s tag %+v "+
			"supply %d change %d with utxos: %+v error: %s", acc.Addr(), to, tag,
			supply, inputAmt-fee, utxos, err)
		return nil, nil, nil, err
	}
	return tx, tid, change, nil
}

// FundTokenTransaction gets the utxo of a public key containing a certain amount of box and token
func FundTokenTransaction(conn *grpc.ClientConn, addr types.Address,
	token *txlogic.TokenID, boxAmount, tokenAmount uint64) (
	*rpcpb.ListUtxosResponse, error) {
	p2pkScript, err := getScriptAddressFromPubKeyHash(addr.Hash())
	if err != nil {
		return nil, err
	}
	logger.Debugf("Script Value: %v", p2pkScript)
	c := rpcpb.NewTransactionCommandClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), connTimeout*time.Second)
	defer cancel()

	tokenBudges := make([]*rpcpb.TokenAmount, 0)
	if token != nil && tokenAmount > 0 {
		outPointMsg, err := token.ToProtoMessage()
		if err != nil {
			return nil, err
		}
		outPointPb, ok := outPointMsg.(*corepb.OutPoint)
		if !ok {
			return nil, fmt.Errorf("Invalid token outpoint")
		}
		tokenBudges = append(tokenBudges, &rpcpb.TokenAmount{
			Token:  outPointPb,
			Amount: tokenAmount,
		})
	}
	r, err := c.FundTransaction(ctx, &rpcpb.FundTransactionRequest{
		Addr:         addr.String(),
		Amount:       boxAmount,
		TokenBudgets: tokenBudges,
	})
	if err != nil {
		return nil, err
	}
	logger.Debugf("Result: %+v", r)
	return r, nil
}

// FundTransaction gets the utxo of a public key
func FundTransaction(conn *grpc.ClientConn, addr types.Address, amount uint64) (*rpcpb.ListUtxosResponse, error) {
	//p2pkScript, err := getScriptAddressFromPubKeyHash(addr.Hash())
	//if err != nil {
	//	return nil, err
	//}
	//logger.Debugf("Script Value: %v", p2pkScript)
	//c := rpcpb.NewTransactionCommandClient(conn)
	//ctx, cancel := context.WithTimeout(context.Background(), connTimeout*time.Second)
	//defer cancel()

	//r, err := c.FundTransaction(ctx, &rpcpb.FundTransactionRequest{
	//	Addr:   addr.String(),
	//	Amount: amount,
	//})
	//if err != nil {
	//	return nil, err
	//}
	//logger.Debugf("Result: %+v", r)
	//return r, nil
	return nil, nil
}

type rpcTransactionHelper struct {
	conn *grpc.ClientConn
}

func (r *rpcTransactionHelper) GetFee() (uint64, error) {
	return GetFeePrice(r.conn)
}

// CreateSplitAddrTransaction creates a split address using input param addrs and weight
func CreateSplitAddrTransaction(conn *grpc.ClientConn, fromAddr types.Address, pubKeyBytes []byte,
	addrs []types.Address, weights []uint64, signer crypto.Signer) (*types.Transaction, error) {
	tx, err := txlogic.CreateSplitAddrTransaction(
		&rpcTransactionHelper{conn: conn}, fromAddr, pubKeyBytes, addrs, weights, signer)
	if err != nil {
		return nil, err
	}
	if err := SendTransaction(conn, tx); err != nil {
		return nil, err
	}
	return tx, nil
}

// CreateTransaction retrieves all the utxo of a public key, and use some of them to send transaction
func CreateTransaction(conn *grpc.ClientConn, fromAddress types.Address, targets map[types.Address]uint64, pubKeyBytes []byte,
	signer crypto.Signer, addrs []types.Address, weights []uint64) (*types.Transaction, error) {
	tx, err := txlogic.CreateTransaction(&rpcTransactionHelper{
		conn: conn,
	}, fromAddress, targets, pubKeyBytes, signer)
	if err != nil {
		return nil, err
	}
	if err := SendTransaction(conn, tx); err != nil {
		return nil, err
	}
	return tx, nil
}

// SendTransaction sends an signed transaction to node server through grpc connection
func SendTransaction(conn *grpc.ClientConn, tx *types.Transaction) error {
	txProtoMsg, err := tx.ToProtoMessage()
	if err != nil {
		return err
	}
	txPb, ok := txProtoMsg.(*corepb.Transaction)
	if !ok {
		return fmt.Errorf("can't convert transaction into protobuf")
	}
	txReq := &rpcpb.SendTransactionRequest{Tx: txPb}

	c := rpcpb.NewTransactionCommandClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), connTimeout*time.Second)
	defer cancel()

	_, err = c.SendTransaction(ctx, txReq)
	if err != nil {
		return err
	}
	return nil
}

// GetRawTransaction get the transaction info of given hash
func GetRawTransaction(conn *grpc.ClientConn, hash []byte) (*types.Transaction, error) {
	c := rpcpb.NewTransactionCommandClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), connTimeout*time.Second)
	defer cancel()
	logger.Debugf("Get transaction of hash: %x", hash)

	r, err := c.GetRawTransaction(ctx, &rpcpb.GetRawTransactionRequest{Hash: hash})
	if err != nil {
		return nil, err
	}
	tx := &types.Transaction{}
	err = tx.FromProtoMessage(r.Tx)
	return tx, err
}

// GetTransactionsInPool gets all transactions in memory pool
func GetTransactionsInPool(conn *grpc.ClientConn) ([]*types.Transaction, error) {
	c := rpcpb.NewTransactionCommandClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), connTimeout*time.Second)
	defer cancel()
	r, err := c.GetTransactionPool(ctx, &rpcpb.GetTransactionPoolRequest{})
	if err != nil {
		return nil, err
	}
	var txs []*types.Transaction
	for _, txMsg := range r.Txs {
		tx := &types.Transaction{}
		err := tx.FromProtoMessage(txMsg)
		if err != nil {
			return nil, err
		}
		txs = append(txs, tx)
	}
	return txs, nil
}

func (r *rpcTransactionHelper) Fund(fromAddr types.Address, amountRequired uint64) (map[types.OutPoint]*types.UtxoWrap, error) {
	resp, err := FundTransaction(r.conn, fromAddr, amountRequired)
	if err != nil {
		return nil, err
	}
	utxos := make(map[types.OutPoint]*types.UtxoWrap)
	for _, u := range resp.GetUtxos() {
		hash := &crypto.HashType{}
		if err := hash.SetBytes(u.OutPoint.Hash); err != nil {
			return nil, err
		}
		op := types.OutPoint{
			Hash:  *hash,
			Index: u.OutPoint.Index,
		}
		wrap := &types.UtxoWrap{
			Output:      u.TxOut,
			BlockHeight: u.BlockHeight,
			IsCoinBase:  u.IsCoinbase,
			IsSpent:     u.IsSpent,
			IsModified:  false,
		}
		utxos[op] = wrap
	}
	return utxos, nil
}
