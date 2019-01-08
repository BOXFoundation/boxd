// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package txgenerator

import (
	"fmt"
	"math"

	"github.com/BOXFoundation/boxd/boxd/eventbus"
	"github.com/BOXFoundation/boxd/core/chain"
	"github.com/BOXFoundation/boxd/core/pb"
	"github.com/BOXFoundation/boxd/core/txpool"
	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/crypto"
	"github.com/BOXFoundation/boxd/log"
	"github.com/BOXFoundation/boxd/p2p"
	"github.com/BOXFoundation/boxd/script"
	"github.com/BOXFoundation/boxd/storage"
	"github.com/BOXFoundation/boxd/util"
	"github.com/BOXFoundation/boxd/wallet"
	"github.com/BOXFoundation/boxd/wallet/utxo"
)

var logger = log.NewLogger("tx_generator")

var (
	txGenerator *TxGenerator
	zeroHash    = crypto.HashType{}
)

// TxGenerator can generate tx automatically
type TxGenerator struct {
	db     storage.Table
	chain  *chain.BlockChain
	peer   *p2p.BoxPeer
	txpool *txpool.TransactionPool
}

// Default return the default TxGenerator
func Default() *TxGenerator {
	return txGenerator
}

// New return a new TxGenerator
func New(s storage.Storage, blockchain *chain.BlockChain, txpool *txpool.TransactionPool, peer *p2p.BoxPeer) (*TxGenerator, error) {
	if txGenerator == nil {
		txGenerator = &TxGenerator{}
	}
	table, err := s.Table(chain.WalletTableName)
	if err != nil {
		return nil, err
	}
	txGenerator.db = table
	txGenerator.chain = blockchain
	txGenerator.txpool = txpool
	txGenerator.peer = peer
	return txGenerator, nil
}

// Run tx generator
func (tg *TxGenerator) Run() {
	eventbus.Default().Reply(eventbus.TopicGenerateTx, func(data *corepb.Data, out chan<- *types.Transaction) {
		logger.Errorf("newTx start!!!!")
		tx, err := tg.NewTx(data)
		if err != nil {
			logger.Errorf("new tx failed: %v", err)
			out <- nil
			return
		}
		logger.Errorf("new tx success: %v", tx)
		out <- tx
	}, false)
}

// FundTransaction asdf
func (tg *TxGenerator) FundTransaction(minerAddr string, requiredAmount uint64) (map[types.OutPoint]*types.UtxoWrap, error) {

	addr, err := types.NewAddress(minerAddr)
	payToPubKeyHashScript := *script.PayToPubKeyHashScript(addr.Hash())
	if err != nil {
		return nil, nil
	}
	wallet := utxo.NewWalletUtxoWithAddress(payToPubKeyHashScript, tg.db)
	utxos, err := wallet.Utxos(addr)
	if err != nil {
		return nil, err
	}

	nextHeight := tg.chain.GetBlockHeight() + 1

	// apply mempool txs as if they were mined into a block with 0 confirmation
	utxoSet := chain.NewUtxoSetFromMap(utxos)
	memPoolTxs, _ := tg.txpool.GetTransactionsInPool()
	// Note: we add utxo first and spend them later to maintain tx topological order within mempool. Since memPoolTxs may not
	// be topologically ordered, if tx1 spends tx2 but tx1 comes after tx2, tx1's output is mistakenly marked as unspent
	// Add utxos first
	for _, tx := range memPoolTxs {
		for txOutIdx, txOut := range tx.Vout {
			// utxo for this address
			if util.IsPrefixed(txOut.ScriptPubKey, payToPubKeyHashScript) {
				if err := utxoSet.AddUtxo(tx, uint32(txOutIdx), nextHeight); err != nil {
					return nil, nil
				}
			}
		}
	}
	// Then spend
	for _, tx := range memPoolTxs {
		for _, txIn := range tx.Vin {
			utxoSet.SpendUtxo(txIn.PrevOutPoint)
		}
	}
	utxos = utxoSet.GetUtxos()

	var current uint64
	tokenAmount := make(map[types.OutPoint]uint64)
	choosenUtxos := make(map[types.OutPoint]*types.UtxoWrap)
	for out, utxo := range utxos {
		token, amount, isToken := tg.getTokenInfo(out, utxo)
		if isToken {
			if val, ok := tokenAmount[token]; ok && val > 0 {
				if val > amount {
					tokenAmount[token] = val - amount
				} else {
					delete(tokenAmount, token)
				}
				current += utxo.Value()
				choosenUtxos[out] = utxo
			} else {
				// Do not include token utxos not needed
				continue
			}
		} else if current < requiredAmount {
			choosenUtxos[out] = utxo
			current += utxo.Value()
		}
		if current >= requiredAmount && len(tokenAmount) == 0 {
			break
		}
	}
	if current < requiredAmount || len(tokenAmount) > 0 {
		errMsg := "Not enough balance"
		return nil, fmt.Errorf(errMsg)
	}
	return choosenUtxos, nil
}

func (tg *TxGenerator) getTokenInfo(outpoint types.OutPoint, wrap *types.UtxoWrap) (types.OutPoint, uint64, bool) {
	s := script.NewScriptFromBytes(wrap.Output.ScriptPubKey)
	if s.IsTokenIssue() {
		if issueParam, err := s.GetIssueParams(); err == nil {
			return outpoint, issueParam.TotalSupply * uint64(math.Pow10(int(issueParam.Decimals))), true
		}
	}
	if s.IsTokenTransfer() {
		if transferParam, err := s.GetTransferParams(); err == nil {
			return transferParam.OutPoint, transferParam.Amount, true
		}
	}
	return types.OutPoint{}, 0, false
}

// NewTx new a tx
func (tg *TxGenerator) NewTx(data *corepb.Data) (*types.Transaction, error) {

	// 获取发送方地址
	accCh := make(chan *wallet.Account)
	eventbus.Default().Send(eventbus.TopicMiner, accCh)
	acc := <-accCh

	amount := uint64(1)
	// 获取utxo
	utxos, err := tg.FundTransaction(acc.Addr(), amount)
	if err != nil {
		return nil, err
	}

	tx, err := tg.NewTxWithUtxo(acc, utxos, []string{zeroHash.String()}, []uint64{amount}, 0, data)
	if err != nil {
		return nil, err
	}

	return tx, nil
}

// NewTxWithUtxo new a transaction
func (tg *TxGenerator) NewTxWithUtxo(fromAcc *wallet.Account, utxos map[types.OutPoint]*types.UtxoWrap, toAddrs []string,
	amounts []uint64, changeAmt uint64, data *corepb.Data) (*types.Transaction, error) {

	logger.Errorf("fromAcc = %v, utxos = %v, toAddrs = %v, amounts = %v, changeAmt = %v", fromAcc.Addr(), utxos, toAddrs, amounts, changeAmt)

	utxoValue := uint64(0)
	for _, u := range utxos {
		utxoValue += u.Output.Value
	}
	amount := uint64(0)
	for _, a := range amounts {
		amount += a
	}
	if utxoValue < amount+changeAmt {
		return nil, fmt.Errorf("input %d is less than output %d", utxoValue, amount+changeAmt)
	}

	// vin
	vins := make([]*types.TxIn, 0)
	for outpoint, utxo := range utxos {
		vins = append(vins, makeVin(outpoint, utxo, 0))
	}

	// vout for toAddrs
	vouts := make([]*corepb.TxOut, 0, len(toAddrs))
	for i, toAddr := range toAddrs {
		vouts = append(vouts, makeVout(toAddr, amounts[i]))
	}

	// vout for change of fromAddress
	fromAddrOut := makeVout(fromAcc.Addr(), changeAmt)

	// construct transaction
	tx := new(types.Transaction)
	tx.Vin = append(tx.Vin, vins...)
	tx.Vout = append(tx.Vout, vouts...)
	tx.Vout = append(tx.Vout, fromAddrOut)
	tx.Data = data

	// sign vin
	if err := tg.signTx(tx, utxos, fromAcc); err != nil {
		return nil, err
	}

	// create change utxo
	tx.TxHash()

	return tx, nil
}

func makeVin(outpoint types.OutPoint, utxo *types.UtxoWrap, seq uint32) *types.TxIn {
	var hash crypto.HashType
	copy(hash[:], outpoint.Hash[:])
	return &types.TxIn{
		PrevOutPoint: types.OutPoint{
			Hash:  hash,
			Index: outpoint.Index,
		},
		ScriptSig: []byte{},
		Sequence:  seq,
	}
}

func makeVout(acc string, amount uint64) *corepb.TxOut {
	address, _ := types.NewAddress(acc)
	addrPkh, _ := types.NewAddressPubKeyHash(address.Hash())
	addrScript := *script.PayToPubKeyHashScript(addrPkh.Hash())
	return &corepb.TxOut{
		Value:        amount,
		ScriptPubKey: addrScript,
	}
}

func (tg *TxGenerator) signTx(tx *types.Transaction, utxos map[types.OutPoint]*types.UtxoWrap, acc *wallet.Account) error {
	i := 0
	for _, utxo := range utxos {
		scriptPkBytes := utxo.Output.ScriptPubKey
		sigHash, err := script.CalcTxHashForSig(scriptPkBytes, tx, i)
		if err != nil {
			return err
		}

		sig, err := crypto.Sign(acc.PrivateKey(), sigHash)
		if err != nil {
			return err
		}
		scriptSig := script.SignatureScript(sig, acc.PublicKey())
		tx.Vin[i].ScriptSig = *scriptSig
		i++
	}
	return nil
}
