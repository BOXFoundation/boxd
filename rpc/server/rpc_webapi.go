// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package rpc

import (
	"container/list"
	"encoding/hex"
	"fmt"
	"math"
	"math/big"
	"strings"
	"time"

	"github.com/BOXFoundation/boxd/core"
	"github.com/BOXFoundation/boxd/core/chain"
	"github.com/BOXFoundation/boxd/core/txlogic"
	"github.com/BOXFoundation/boxd/core/types"
	state "github.com/BOXFoundation/boxd/core/worldstate"
	"github.com/BOXFoundation/boxd/crypto"
	rpcpb "github.com/BOXFoundation/boxd/rpc/pb"
	"github.com/BOXFoundation/boxd/rpc/rpcutil"
	"github.com/BOXFoundation/boxd/script"
	"github.com/BOXFoundation/boxd/vm"
	"github.com/jbenet/goprocess"
	uuid "github.com/satori/go.uuid"
	"golang.org/x/net/context"
)

func init() {
	RegisterServiceWithGatewayHandler(
		"web",
		registerWebapi,
		rpcpb.RegisterWebApiHandlerFromEndpoint,
	)
}

func registerWebapi(s *Server) {
	was := newWebAPIServer(s)
	rpcpb.RegisterWebApiServer(s.server, was)
}

type webapiServer struct {
	ChainBlockReader
	TxPoolReader
	proc goprocess.Process

	endpoints      map[uint32]rpcutil.Endpoint
	connController map[string]map[string]chan<- bool
}

// ChainTxReader defines chain tx reader interface
type ChainTxReader interface {
	LoadBlockInfoByTxHash(crypto.HashType) (*types.Block, *types.Transaction, error)
	GetDataFromDB([]byte) ([]byte, error)
	GetTxReceipt(*crypto.HashType) (*types.Receipt, error)
}

// ChainBlockReader defines chain block reader interface
type ChainBlockReader interface {
	ChainTxReader
	// LoadBlockInfoByTxHash(crypto.HashType) (*types.Block, *types.Transaction, error)
	ReadBlockFromDB(*crypto.HashType) (*types.Block, int, error)
	EternalBlock() *types.Block
	NewEvmContextForLocalCallByHeight(msg types.Message, height uint32) (*vm.EVM, func() error, error)
	TailBlock() *types.Block
	GetLogs(from, to uint32, topicslist [][][]byte) ([]*types.Log, error)
	FilterLogs(logs []*types.Log, topicslist [][][]byte) ([]*types.Log, error)
	TailState() *state.StateDB
}

// TxPoolReader defines tx pool reader interface
type TxPoolReader interface {
	GetTxByHash(hash *crypto.HashType) (*types.TxWrap, bool)
}

func newWebAPIServer(s *Server) *webapiServer {
	server := &webapiServer{
		ChainBlockReader: s.GetChainReader(),
		TxPoolReader:     s.GetTxHandler(),
		proc:             s.Proc(),
		endpoints:        make(map[uint32]rpcutil.Endpoint),
		connController:   make(map[string]map[string]chan<- bool),
	}

	if s.cfg.SubScribeBlocks {
		server.endpoints[rpcutil.BlockEp] = rpcutil.NewBlockEndpoint(s.eventBus)
	}
	if s.cfg.SubScribeLogs {
		server.endpoints[rpcutil.LogEp] = rpcutil.NewLogEndpoint(s.eventBus)
	}

	return server
}

func (s *webapiServer) Closing() bool {
	select {
	case <-s.proc.Closing():
		return true
	default:
		return false
	}
}

func (s *webapiServer) CloseConn(uid string) {
	if subs, ok := s.connController[uid]; ok {
		for _, ch := range subs {
			if ch != nil {
				ch <- true
			}
		}
		s.connController[uid] = nil
		delete(s.connController, uid)
	}
}

func (s *webapiServer) subscribe(uids ...string) (string, chan bool, error) {
	ch := make(chan bool, 1)
	if len(uids) < 1 {
		return "", nil, fmt.Errorf("Invalid uids")
	}
	connuid := uids[0]
	var uid string

	if len(uids) > 1 {
		uid = uids[1]
	} else {
		uid = uuid.NewV1().String()
	}

	if subs, ok := s.connController[connuid]; ok {
		subs[uid] = chan<- bool(ch)
	} else {
		subs = make(map[string]chan<- bool)
		subs[uid] = ch
		s.connController[connuid] = subs
	}
	return uid, ch, nil
}

func (s *webapiServer) Unsubscribe(connuid, uid string) {
	logger.Debugf("Unsubscribe webapiServer connuid: %v, uid: %v", connuid, uid)
	if subs, ok := s.connController[connuid]; ok {
		if ch, ok := subs[uid]; ok {
			ch <- true
		}
		delete(subs, uid)
		if len(subs) == 0 {
			delete(s.connController, connuid)
		}
	}
}

func newViewTxDetailResp(code int32, msg string) *rpcpb.ViewTxDetailResp {
	return &rpcpb.ViewTxDetailResp{
		Code:    code,
		Message: msg,
	}
}

func (s *webapiServer) ViewTxDetail(
	ctx context.Context, req *rpcpb.ViewTxDetailReq,
) (*rpcpb.ViewTxDetailResp, error) {

	logger.Infof("view tx detail req: %+v", req)
	// fetch hash from request
	hash := new(crypto.HashType)
	if err := hash.SetString(req.Hash); err != nil {
		logger.Warn("view tx detail error: ", err)
		return newViewTxDetailResp(-1, err.Error()), nil
	}
	// new resp
	resp := new(rpcpb.ViewTxDetailResp)
	// fetch tx from chain and set status
	var tx *types.Transaction
	var block *types.Block
	var err error
	br, tr := s.ChainBlockReader, s.TxPoolReader
	if block, tx, err = br.LoadBlockInfoByTxHash(*hash); err == nil {
		// calc tx status
		if blockConfirmed(block, br) {
			resp.Status = rpcpb.TxStatus_confirmed
		} else {
			resp.Status = rpcpb.TxStatus_onchain
		}
		resp.BlockTime = block.Header.TimeStamp
		resp.BlockHeight = block.Header.Height
	} else {
		logger.Warnf("view tx detail load block by tx hash %s error: %s,"+
			" try get it from tx pool", hash, err)
		txWrap, _ := tr.GetTxByHash(hash)
		if txWrap == nil {
			return newViewTxDetailResp(-1, "tx not found"), nil
		}
		tx = txWrap.Tx
		resp.Status = rpcpb.TxStatus_pending
		resp.BlockTime = txWrap.AddedTimestamp
		resp.BlockHeight = txWrap.Height
	}
	resp.Version = tx.Version
	// fetch tx details
	detail, err := detailTx(tx, br, tr, req.GetSpreadSplit(), true)
	if err != nil {
		logger.Warn("view tx detail error: ", err)
		return newViewTxDetailResp(-1, err.Error()), nil
	}
	//
	resp.Detail = detail
	return resp, nil
}

func newViewBlockDetailResp(code int32, msg string) *rpcpb.ViewBlockDetailResp {
	return &rpcpb.ViewBlockDetailResp{
		Code:    code,
		Message: msg,
	}
}

func (s *webapiServer) ViewBlockDetail(
	ctx context.Context, req *rpcpb.ViewBlockDetailReq,
) (*rpcpb.ViewBlockDetailResp, error) {

	logger.Infof("view block detail req: %+v", req)
	hash := new(crypto.HashType)
	if err := hash.SetString(req.Hash); err != nil {
		logger.Warn("view block detail error: ", err)
		return newViewBlockDetailResp(-1, err.Error()), nil
	}

	br, tr := s.ChainBlockReader, s.TxPoolReader
	block, n, err := br.ReadBlockFromDB(hash)
	if err != nil {
		logger.Warn("view block detail error: ", err)
		return newViewBlockDetailResp(-1, err.Error()), nil
	}
	detail, err := detailBlock(block, br, tr, true)
	if err != nil {
		logger.Warn("view block detail error: ", err)
		return newViewBlockDetailResp(-1, err.Error()), nil
	}
	resp := newViewBlockDetailResp(0, "")
	resp.Detail = detail
	resp.Detail.Size_ = uint32(n)
	return resp, nil
}

func (s *webapiServer) Connect(
	stream rpcpb.WebApi_ConnectServer,
) error {

	connuid := uuid.NewV1().String()

	defer func() {
		s.CloseConn(connuid)
		logger.Info("Close the persistent conn.")
	}()

	resultCh := make(chan *rpcpb.ListenedData)
	// register endpoints.
	p := s.proc.Go(func(p goprocess.Process) {
		for {
			select {
			case <-p.Closing():
				logger.Debug("Closing rpc stream")
				return
			default:
			}

			registerReq, err := stream.Recv()
			if err != nil {
				logger.Warn(err)
				break
			}
			if registerReq.Cancel {
				switch registerReq.Type {
				case rpcutil.BlockEp:
					s.unsubscribeBlockEndpoint(connuid)
				case rpcutil.LogEp:
					uid := registerReq.Info.(*rpcpb.RegisterReq_LogsReq).LogsReq.Uid
					s.unsubscribeLogEndpoint(connuid, uid)
				}
				continue
			}

			endpoint, ok := s.endpoints[registerReq.Type]
			if !ok {
				logger.Errorf("Api not supported: %d", registerReq.Type)
				continue
			}

			var ch chan bool
			var uid string
			switch registerReq.Type {
			case rpcutil.BlockEp:
				if _, ch, err = s.subscribeBlockEndpoint(connuid); err != nil {
					logger.Error(err)
					continue
				}
				go s.listenBlocks(endpoint, resultCh, ch)
			case rpcutil.LogEp:
				if uid, ch, err = s.subscribeLogEndpoint(connuid); err != nil {
					logger.Error(err)
					continue
				}
				go s.listenLogs(registerReq.Info.(*rpcpb.RegisterReq_LogsReq), endpoint, resultCh, ch)
				if err := stream.Send(&rpcpb.ListenedData{Type: rpcutil.RegisterResp, Data: &rpcpb.ListenedData_Info{Info: &rpcpb.RegisterDetails{Uid: uid}}}); err != nil {
					logger.Warnf("Webapi send uid data error %v", err)
				}
			default:
				logger.Errorf("Register type not found: %d", registerReq.Type)
			}

		}
	})

	for {
		select {
		case data := <-resultCh:
			if err := stream.Send(data); err != nil {
				logger.Warnf("Webapi send data error %v", err)
			}
		case <-s.proc.Closing():
			return nil
		case <-p.Closing():
			return nil
		}
	}
}

func (s *webapiServer) listenBlocks(endpoint rpcutil.Endpoint, resultCh chan *rpcpb.ListenedData, cancelCh <-chan bool) {
	logger.Info("start listen new blocks")
	defer func() {
		if err := endpoint.Unsubscribe(); err != nil {
			logger.Error(err)
		}
	}()

	var (
		elm  *list.Element
		exit bool
	)
	for {
		select {
		case <-cancelCh:
			return
		default:
		}

		if s.Closing() {
			logger.Info("receive closing signal, exit ListenAndReadNewBlock ...")
			return
		}
		rwmtx := endpoint.GetEventMutex()
		rwmtx.RLock()
		if endpoint.GetQueue().Len() != 0 {
			elm = endpoint.GetQueue().Front()
			rwmtx.RUnlock()
			break
		}
		rwmtx.RUnlock()
		time.Sleep(100 * time.Millisecond)
	}
	for {
		select {
		case <-cancelCh:
			logger.Info("Unsubscribe Blocks.")
			return
		default:
		}

		// get block
		block := elm.Value.(*types.Block)

		blockDetail, err := detailBlock(block, s.ChainBlockReader, s.TxPoolReader, true)
		if err == nil {
			br := s.ChainBlockReader
			_, n, err := br.ReadBlockFromDB(block.BlockHash())
			if err != nil {
				logger.Warn(err)
			}
			blockDetail.Size_ = uint32(n)

			resultCh <- &rpcpb.ListenedData{Type: rpcutil.BlockEp, Data: &rpcpb.ListenedData_Block{Block: blockDetail}}

			logger.Debugf("webapi server sent a block, hash: %s, height: %d",
				blockDetail.Hash, blockDetail.Height)
		} else {
			logger.Warnf("detail block %s height %d error: %s",
				block.BlockHash(), block.Header.Height, err)
		}

		if elm, exit = s.moveToNextElem(endpoint, elm); exit {
			return
		}
	}
}

func (s *webapiServer) listenLogs(logsreq *rpcpb.RegisterReq_LogsReq, endpoint rpcutil.Endpoint, resultCh chan *rpcpb.ListenedData, cancelCh <-chan bool) error {

	if logsreq == nil {
		return fmt.Errorf("Invalid params")
	}
	req := logsreq.LogsReq

	logger.Info("start listen new logs")
	defer func() {
		if err := endpoint.Unsubscribe(); err != nil {
			logger.Error(err)
		}
	}()
	var (
		elm  *list.Element
		exit bool
	)
	for {
		select {
		case <-cancelCh:
			return nil
		default:
		}

		if s.Closing() {
			logger.Info("receive closing signal, exit ListenAndReadNewLog ...")
			return nil
		}
		rwmtx := endpoint.GetEventMutex()
		rwmtx.RLock()
		if endpoint.GetQueue().Len() != 0 {
			elm = endpoint.GetQueue().Front()
			rwmtx.RUnlock()
			break
		}
		rwmtx.RUnlock()
		time.Sleep(100 * time.Millisecond)
	}

	for {
		select {
		case <-cancelCh:
			return nil
		default:
		}
		// get logs
		logs := elm.Value.(map[string][]*types.Log)

		filteredLogs := []*types.Log{}
		topicslist := [][][]byte{[][]byte{}}
		for _, addr := range req.Addresses {
			if l, ok := logs[addr]; ok {
				filteredLogs = append(filteredLogs, l...)
			}
			contractAddress, err := types.NewContractAddress(addr)
			if err != nil {
				logger.Error(err)
				return err
			}
			topicslist[0] = append(topicslist[0], contractAddress.Hash())
		}

		for i, topiclist := range req.Topics {
			topicslist = append(topicslist, [][]byte{})
			for _, topic := range topiclist.Topics {
				t, err := hex.DecodeString(topic)
				if err != nil {
					logger.Error(err)
					return err
				}
				topicslist[i+1] = append(topicslist[i+1], t)
			}
		}

		filteredLogs, err := s.ChainBlockReader.FilterLogs(filteredLogs, topicslist)
		if err != nil {
			logger.Error(err)
			return err
		}

		resultLogs := rpcutil.ToPbLogs(filteredLogs)
		if len(resultLogs) != 0 {
			resultCh <- &rpcpb.ListenedData{Type: rpcutil.LogEp, Data: &rpcpb.ListenedData_Logs{Logs: &rpcpb.LogDetails{Logs: resultLogs}}}
			logger.Debugf("webapi server sent a log, data: %v", resultLogs)
		}
		if elm, exit = s.moveToNextElem(endpoint, elm); exit {
			return nil
		}
	}
}

func (s *webapiServer) moveToNextElem(endpoint rpcutil.Endpoint, elm *list.Element) (elmA *list.Element, exit bool) {
	// move to next element
	for {
		if s.Closing() {
			logger.Info("receive closing signal, exit ListenAndReadNewBlock ...")
			return nil, true
		}
		rwmtx := endpoint.GetEventMutex()
		rwmtx.RLock()
		if next := elm.Next(); next != nil {
			elmA = next
		} else if elm.Prev() == nil {
			// if this element is removed, move to the front of list
			elmA = endpoint.GetQueue().Front()
		}
		// occur when elm is the front
		if elmA != nil && elm != elmA {
			rwmtx.RUnlock()
			return elmA, false
		}
		rwmtx.RUnlock()
		time.Sleep(100 * time.Millisecond)
	}
}

func (s *webapiServer) subscribeBlockEndpoint(connuid string) (string, chan bool, error) {
	uid, ch, err := s.subscribe([]string{connuid, "blocks"}...)
	if err != nil {
		return "", nil, err
	}
	err = s.endpoints[rpcutil.BlockEp].Subscribe()
	if err != nil {
		return "", nil, err
	}
	return uid, ch, nil
}

func (s *webapiServer) subscribeLogEndpoint(connuid string) (string, chan bool, error) {
	uid, ch, err := s.subscribe(connuid)
	if err != nil {
		return "", nil, err
	}
	err = s.endpoints[rpcutil.LogEp].Subscribe()
	if err != nil {
		return "", nil, err
	}
	return uid, ch, nil
}

func (s *webapiServer) unsubscribeBlockEndpoint(connuid string) {
	s.Unsubscribe(connuid, "blocks")
	// Infrequent unsubscribe
	// return s.endpoints[rpcutil.BlockEp].Unsubscribe()
}

func (s *webapiServer) unsubscribeLogEndpoint(connuid, uid string) {
	s.Unsubscribe(connuid, uid)
	// Infrequent unsubscribe
	// return s.endpoints[rpcutil.LogEp].Unsubscribe()
}

func newCallResp(code int32, msg string) *rpcpb.CallResp {
	return &rpcpb.CallResp{
		Code:    code,
		Message: msg,
	}
}

func (s *webapiServer) DoCall(
	ctx context.Context, req *rpcpb.CallReq,
) (resp *rpcpb.CallResp, err error) {

	from, to := req.GetFrom(), req.GetTo()
	fromHash, err := types.ParseAddress(from)
	if err != nil || !strings.HasPrefix(from, types.AddrTypeP2PKHPrefix) {
		return newCallResp(-1, "invalid from address"), nil
	}
	contractAddr, err := types.ParseAddress(to)
	if err != nil || !strings.HasPrefix(to, types.AddrTypeContractPrefix) {
		return newCallResp(-1, "invalid contract address"), nil
	}

	data, err := hex.DecodeString(req.GetData())
	if err != nil {
		return newCallResp(-1, "invalid contract data"), nil
	}

	msg := types.NewVMTransaction(new(big.Int), big.NewInt(1), math.MaxUint64/2,
		0, nil, types.ContractCallType, data).
		WithFrom(fromHash.Hash160()).WithTo(contractAddr.Hash160())

	// Setup context so it may be cancelled the call has completed
	// or, in case of unmetered gas, setup a context with a timeout.
	var cancel context.CancelFunc
	if req.Timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, time.Duration(req.Timeout))
	} else {
		ctx, cancel = context.WithCancel(ctx)
	}
	// Make sure the context is cancelled when the call has completed
	// this makes sure resources are cleaned up.
	defer cancel()

	// Get a new instance of the EVM.
	evm, vmErr, err := s.NewEvmContextForLocalCallByHeight(msg, req.Height)
	if err != nil {
		return newCallResp(-1, err.Error()), nil
	}
	// Wait for the context to be done and cancel the evm. Even if the
	// EVM has finished, cancelling may be done (repeatedly)
	go func() {
		<-ctx.Done()
		evm.Cancel()
	}()

	// Setup the gas pool (also for unmetered requests)
	// and apply the message.
	output, _, _, _, _, err := chain.ApplyMessage(evm, msg)
	if err := vmErr(); err != nil {
		return newCallResp(-1, err.Error()), nil
	}

	// Make sure we have a contract to operate on, and bail out otherwise.
	if err == nil && len(output) == 0 {
		conHash := msg.To()
		// Make sure we have a contract to operate on, and bail out otherwise.
		if code := evm.StateDB.GetCode(*conHash); len(code) == 0 {
			return newCallResp(-1, core.ErrContractNotFound.Error()), nil
		}
	}

	resp = newCallResp(0, "")
	resp.Output = hex.EncodeToString(output)
	return resp, nil
}

func newNonceResp(code int32, msg string, nonce uint64) *rpcpb.NonceResp {
	return &rpcpb.NonceResp{
		Code:    code,
		Message: msg,
		Nonce:   nonce,
	}
}

func (s *webapiServer) Nonce(
	ctx context.Context, req *rpcpb.NonceReq,
) (*rpcpb.NonceResp, error) {

	address, err := types.ParseAddress(req.GetAddr())
	if err != nil {
		return newNonceResp(-1, err.Error(), 0), nil
	}
	switch address.(type) {
	default:
		return newNonceResp(-1, "only allow eoa and contract address", 0), nil
	case *types.AddressContract, *types.AddressPubKeyHash:
	}
	return newNonceResp(0, "", s.TailState().GetNonce(*address.Hash160())), nil
}

func newLogsResp(code int32, msg string, logs []*types.Log) *rpcpb.Logs {

	return &rpcpb.Logs{
		Code:    code,
		Message: msg,
		Logs:    rpcutil.ToPbLogs(logs),
	}
}

func (s *webapiServer) GetLogs(ctx context.Context, req *rpcpb.LogsReq) (logs *rpcpb.Logs, err error) {

	from, to := req.From, req.To

	if len(req.Hash) != 0 {
		hash, err := hex.DecodeString(req.Hash)
		if err != nil {
			return newLogsResp(-1, err.Error(), nil), nil
		}
		hh := crypto.BytesToHash(hash)
		b, _, err := s.ChainBlockReader.ReadBlockFromDB(&hh)
		if err != nil {
			return newLogsResp(-1, err.Error(), nil), nil
		}
		from, to = b.Header.Height, b.Header.Height
	} else {
		tail := s.ChainBlockReader.TailBlock()
		if from > tail.Header.Height {
			return newLogsResp(0, "", nil), nil
		}
		if to > tail.Header.Height {
			to = tail.Header.Height
		}
		if from > to {
			return newLogsResp(-1, fmt.Sprint("From not allowed to be greater than To"), nil), nil
		}
	}

	topicslist := [][][]byte{[][]byte{}}
	for _, addr := range req.Addresses {

		contractAddress, err := types.NewContractAddress(addr)
		if err != nil {
			return newLogsResp(-1, err.Error(), nil), nil
		}
		topicslist[0] = append(topicslist[0], contractAddress.Hash())
	}

	for i, topiclist := range req.Topics {
		topicslist = append(topicslist, [][]byte{})
		for _, topic := range topiclist.Topics {
			t, err := hex.DecodeString(topic)
			if err != nil {
				return newLogsResp(-1, err.Error(), nil), nil
			}
			topicslist[i+1] = append(topicslist[i+1], t)
		}
	}

	blocklogs, err := s.ChainBlockReader.GetLogs(from, to, topicslist)
	if err != nil {
		return newLogsResp(-1, err.Error(), nil), nil
	}
	return newLogsResp(0, "", blocklogs), nil
}

func detailTx(
	tx *types.Transaction, br ChainTxReader, tr TxPoolReader, spread bool, detailVin bool,
) (*rpcpb.TxDetail, error) {

	detail := new(rpcpb.TxDetail)
	needSpread := false
	// parse vout
	txHash, _ := tx.TxHash()
	for i := range tx.Vout {
		outDetail, err := detailTxOut(txHash, tx.Vout[i], uint32(i), tx.Data, br)
		if err != nil {
			logger.Warnf("detail tx vout for %s %d %+v error: %s", txHash, i, tx.Vout[i], err)
			return nil, err
		}
		if strings.HasPrefix(outDetail.Addr, types.AddrTypeSplitAddrPrefix) &&
			outDetail.Type != rpcpb.TxOutDetail_new_split_addr {
			needSpread = true
			if spread {
				break
			}
		}
		detail.Vout = append(detail.Vout, outDetail)
	}
	if spread && needSpread {
		logger.Infof("spread split addr original tx: %s", txHash)
		buf, err := br.GetDataFromDB(chain.SplitTxHashKey(txHash))
		if err != nil {
			return nil, err
		}
		if len(buf) == 0 {
			return nil, fmt.Errorf("split tx for %s not found in db", txHash)
		}
		tx = new(types.Transaction)
		if err := tx.Unmarshal(buf); err != nil {
			return nil, err
		}
		detail.Vout = nil
		hash, _ := tx.TxHash()
		logger.Infof("spread split addr tx: %s", hash)
		for i := range tx.Vout {
			outDetail, err := detailTxOut(hash, tx.Vout[i], uint32(i), tx.Data, br)
			if err != nil {
				logger.Warnf("detail split tx vout for %s %d %+v error: %s", hash, i, tx.Vout[i], err)
				return nil, err
			}
			detail.Vout = append(detail.Vout, outDetail)
		}
	}
	// hash
	detail.Hash = txHash.String()
	// parse vin
	for i, in := range tx.Vin {
		inDetail, err := detailTxIn(in, br, tr, detailVin)
		if err != nil {
			logger.Warnf("detail tx vin for %s %d %+v error: %s", txHash, i, in, err)
			return nil, err
		}
		detail.Vin = append(detail.Vin, inDetail)
	}
	return detail, nil
}

func detailBlock(
	block *types.Block, r ChainBlockReader, tr TxPoolReader, detailVin bool,
) (*rpcpb.BlockDetail, error) {

	if block == nil || block.Header == nil {
		return nil, fmt.Errorf("detailBlock error for block: %+v", block)
	}
	detail := new(rpcpb.BlockDetail)
	detail.Version = block.Header.Version
	detail.Height = block.Header.Height
	detail.TimeStamp = block.Header.TimeStamp
	detail.Hash = block.BlockHash().String()
	detail.PrevBlockHash = block.Header.PrevBlockHash.String()
	coinBase, err := types.NewAddressPubKeyHash(block.Header.BookKeeper[:])
	if err != nil {
		return nil, err
	}
	detail.CoinBase = coinBase.String()
	detail.Confirmed = blockConfirmed(block, r)
	detail.Signature = hex.EncodeToString(block.Signature)
	for _, tx := range block.Txs {
		txDetail, err := detailTx(tx, r, tr, false, detailVin)
		if err != nil {
			hash, _ := tx.TxHash()
			logger.Warnf("detail tx %s error: %s", hash, err)
			return nil, err
		}
		detail.Txs = append(detail.Txs, txDetail)
	}
	for _, tx := range block.InternalTxs {
		txDetail, err := detailTx(tx, r, tr, false, detailVin)
		if err != nil {
			hash, _ := tx.TxHash()
			logger.Warnf("detail tx %s error: %s", hash, err)
			return nil, err
		}
		detail.InternalTxs = append(detail.InternalTxs, txDetail)
	}
	return detail, nil
}

func detailTxIn(
	txIn *types.TxIn, r ChainTxReader, tr TxPoolReader, detailVin bool,
) (*rpcpb.TxInDetail, error) {

	detail := new(rpcpb.TxInDetail)
	// if tx in is coin base
	if txIn.PrevOutPoint.Hash == crypto.ZeroHash {
		return detail, nil
	}
	//
	detail.ScriptSig = hex.EncodeToString(txIn.ScriptSig)
	detail.Sequence = txIn.Sequence
	detail.PrevOutPoint = txlogic.EncodeOutPoint(txlogic.ConvOutPoint(&txIn.PrevOutPoint))

	if detailVin {
		hash := &txIn.PrevOutPoint.Hash
		_, prevTx, err := r.LoadBlockInfoByTxHash(*hash)
		if err != nil {
			logger.Infof("load tx by hash %s from chain error: %s, try tx pool", hash, err)
			txWrap, _ := tr.GetTxByHash(hash)
			if txWrap == nil {
				return nil, fmt.Errorf("tx not found for detail txIn %+v", txIn.PrevOutPoint)
			}
			prevTx = txWrap.Tx
		}
		index := txIn.PrevOutPoint.Index
		prevTxHash, _ := prevTx.TxHash()
		detail.PrevOutDetail, err = detailTxOut(prevTxHash, prevTx.Vout[index],
			index, prevTx.Data, r)
		if err != nil {
			logger.Warnf("detail prev tx vout for %s %d %+v error: %s", prevTxHash,
				index, prevTx.Vout[index], err)
			return nil, err
		}
	}
	return detail, nil
}

func detailTxOut(
	txHash *crypto.HashType, txOut *types.TxOut, index uint32, data *types.Data,
	br ChainTxReader,
) (*rpcpb.TxOutDetail, error) {

	detail := new(rpcpb.TxOutDetail)
	sc := script.NewScriptFromBytes(txOut.ScriptPubKey)
	// addr
	addr, err := ParseAddrFrom(sc, txHash, index, br)
	if err != nil {
		return nil, err
	}
	detail.Addr = addr
	// value
	op := types.NewOutPoint(txHash, index)
	// height 0 is unused
	wrap := types.NewUtxoWrap(txOut.Value, txOut.ScriptPubKey, 0)
	utxo := txlogic.MakePbUtxo(op, wrap)
	amount, _, err := txlogic.ParseUtxoAmount(utxo)
	if err != nil {
		return nil, err
	}
	detail.Value = amount
	// script pubic key
	detail.ScriptPubKey = hex.EncodeToString(txOut.ScriptPubKey)
	// script disasm
	detail.ScriptDisasm = sc.Disasm()
	// type
	detail.Type = parseVoutType(txOut)
	//  appendix
	switch detail.Type {
	default:
	case rpcpb.TxOutDetail_token_issue:
		param, err := sc.GetIssueParams()
		if err != nil {
			return nil, err
		}
		issueInfo := &rpcpb.TxOutDetail_TokenIssueInfo{
			TokenIssueInfo: &rpcpb.TokenIssueInfo{
				TokenTag: &rpcpb.TokenTag{
					Name:    param.Name,
					Symbol:  param.Symbol,
					Supply:  param.TotalSupply,
					Decimal: uint32(param.Decimals),
				},
			},
		}
		detail.Appendix = issueInfo
	case rpcpb.TxOutDetail_token_transfer:
		param, err := sc.GetTransferParams()
		if err != nil {
			return nil, err
		}
		transferInfo := &rpcpb.TxOutDetail_TokenTransferInfo{
			TokenTransferInfo: &rpcpb.TokenTransferInfo{
				TokenId: txlogic.EncodeOutPoint(txlogic.ConvOutPoint(&param.TokenID.OutPoint)),
			},
		}
		detail.Appendix = transferInfo
	case rpcpb.TxOutDetail_new_split_addr:
		addresses, weights, err := sc.ParseSplitAddrScript()
		if err != nil {
			return nil, err
		}
		addrs := make([]string, 0, len(addresses))
		for _, a := range addresses {
			addrs = append(addrs, a.String())
		}
		splitContractInfo := &rpcpb.TxOutDetail_SplitContractInfo{
			SplitContractInfo: &rpcpb.SplitContractInfo{
				Addrs:   addrs,
				Weights: weights,
			},
		}
		detail.Appendix = splitContractInfo
	case rpcpb.TxOutDetail_contract_call:
		var (
			params *types.VMTxParams
			typ    types.ContractType
		)
		if data == nil || data.Type != int32(types.ContractDataType) ||
			len(data.Content) == 0 {
			return nil, core.ErrContractDataNotFound
		}
		sc := script.NewScriptFromBytes(txOut.ScriptPubKey)
		params, typ, err = sc.ParseContractParams()
		if err != nil {
			return nil, err
		}
		// get tx reveipt.
		receipt, err := br.GetTxReceipt(txHash)
		if err != nil {
			logger.Warn(err)
			return nil, err
		}
		if receipt == nil {
			return nil, fmt.Errorf("receipt for tx %s not found", txHash)
		}

		contractInfo := &rpcpb.TxOutDetail_ContractInfo{
			ContractInfo: &rpcpb.ContractInfo{
				Fee:      uint32(receipt.GasUsed * params.GasPrice),
				Failed:   receipt.Failed,
				GasLimit: params.GasLimit,
				GasUsed:  receipt.GasUsed,
				Nonce:    params.Nonce,
				Data:     hex.EncodeToString(data.Content),
				Logs:     rpcutil.ToPbLogs(receipt.Logs),
			},
		}
		if typ == types.ContractCreationType {
			detail.Type = rpcpb.TxOutDetail_contract_create
		}
		detail.Appendix = contractInfo
	}
	//
	return detail, nil
}

func parseVoutType(txOut *types.TxOut) rpcpb.TxOutDetail_TxOutType {
	sc := script.NewScriptFromBytes(txOut.ScriptPubKey)
	if sc.IsPayToPubKeyHash() {
		return rpcpb.TxOutDetail_pay_to_pubkey_hash
	} else if sc.IsPayToPubKeyHashCLTVScript() {
		return rpcpb.TxOutDetail_pay_to_pubkey_hash_cltv
	} else if sc.IsTokenIssue() {
		return rpcpb.TxOutDetail_token_issue
	} else if sc.IsTokenTransfer() {
		return rpcpb.TxOutDetail_token_transfer
	} else if sc.IsSplitAddrScript() {
		return rpcpb.TxOutDetail_new_split_addr
	} else if sc.IsPayToScriptHash() {
		return rpcpb.TxOutDetail_pay_to_script_hash
	} else if sc.IsContractPubkey() {
		// distinguish create and call after this.
		return rpcpb.TxOutDetail_contract_call
	}

	return rpcpb.TxOutDetail_unknown
}

func blockConfirmed(b *types.Block, r ChainBlockReader) bool {
	if b == nil {
		return false
	}
	eternalHeight := r.EternalBlock().Header.Height
	return eternalHeight >= b.Header.Height
}

// ParseAddrFrom parse addr from scriptPubkey
func ParseAddrFrom(
	sc *script.Script, txHash *crypto.HashType, idx uint32, tr ChainTxReader,
) (string, error) {
	var (
		address types.Address
		err     error
	)
	switch {
	case sc.IsContractPubkey():
		address, err = sc.ParseContractAddr()
		if err == nil && (address == nil || len(address.Hash()) == 0) {
			// smart contract deploy
			from, err := sc.ParseContractFrom()
			if err != nil {
				return "", err
			}
			nonce, err := sc.ParseContractNonce()
			if err != nil {
				return "", err
			}
			address, _ = types.MakeContractAddress(from, nonce)
		}
	case sc.IsSplitAddrScript():
		addrs, weights, err := sc.ParseSplitAddrScript()
		if err != nil {
			return "", err
		}
		address = txlogic.MakeSplitAddress(txHash, idx, addrs, weights)
	default:
		address, err = sc.ExtractAddress()
	}
	if err != nil {
		return "", err
	}
	return address.String(), nil
}
