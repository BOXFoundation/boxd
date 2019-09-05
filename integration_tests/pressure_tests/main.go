package main

import (
	"fmt"

	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/integration_tests/utils"
	"github.com/BOXFoundation/boxd/log"
	"github.com/BOXFoundation/boxd/rpc/rpcutil"
	"google.golang.org/grpc"
)

var logger = log.NewLogger("pressure") // logger

func main() {

	conn, err := grpc.Dial("localhost:19141", grpc.WithInsecure())
	if err != nil {
		logger.Panic(err)
	}
	defer conn.Close()

	preKeyStore := "../.devconfig/keyfile/pre.keystore"
	preAddrs, preAccs := utils.LoadAccounts(preKeyStore)

	acc := preAccs[0]
	addr, _ := types.ParseAddress(preAddrs[0])
	txss, transfer, fee, count, err := rpcutil.NewTxs(acc, addr.(*types.AddressPubKeyHash).Hash160(), 300, conn)

	fmt.Printf("txs count: %v %v, transfer: %v, fee: %v, count: %v, err: %v", len(txss), len(txss[0]), transfer, fee, count, err)
	for _, txs := range txss {
		for _, tx := range txs {
			if _, err := rpcutil.SendTransaction(conn, tx); err != nil {
				logger.Panic(err)
			}
		}
	}
}
