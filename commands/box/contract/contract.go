// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package contract

import (
	"bufio"
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"io/ioutil"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/BOXFoundation/boxd/commands/box/common"
	"github.com/BOXFoundation/boxd/commands/box/root"
	"github.com/BOXFoundation/boxd/core"
	"github.com/BOXFoundation/boxd/core/abi"
	"github.com/BOXFoundation/boxd/core/types"
	rpcpb "github.com/BOXFoundation/boxd/rpc/pb"
	"github.com/BOXFoundation/boxd/rpc/rpcutil"
	"github.com/BOXFoundation/boxd/util"
	"github.com/BOXFoundation/boxd/wallet"
	"github.com/BOXFoundation/boxd/wallet/account"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/ripemd160"
)

const (
	baseDir    = ".cmd/contract/"
	senderFile = baseDir + "sender"
	abiIdxFile = baseDir + "idx"
	abiDir     = baseDir + "abi/"
	recordFile = abiDir + "record"
	abiSep     = ""
)

var (
	walletDir   string
	solFilePath string
	solcBinReg  = regexp.MustCompile("Binary:\\s?\n([0-9a-f]+)\\s")
	solcAbiReg  = regexp.MustCompile("ABI\\s?\n([\\pP0-9a-zA-Z]+)\\s")
	solNameReg  = regexp.MustCompile(solFilePath + ":([0-9a-zA-Z_]+)\\s")
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "contract [command]",
	Short: "The contract command line interface",
	Example: `
  1. set sender
    ./box contract setsender b1fc1Vzz73WvBtzNQNbBSrxNCUC1Zrbnq4m
    NOTE: account sender must be imported and unlocked to local wallet

  2. import abi file
    ./box contract importabi .cmd/contract/test/erc20_simple.abi "simple erc20 abi"

  3. list abi index and select one to continue:
    ./box contract list

    output e.g.:
      abi list:
        1: simple erc20 abi Sep 24 13:15:49
        2: bonus abi Sep 26 11:15:19
      contract attached list:
        1 > b5rEkkMtdp2LVNPyFem7w9sJthTuYvDFbiz
        2 > b5rEkkMtdp2LVNPyFem7w9sJthTuYvDFbiz

    index is the number "1, 2, ...", that is relevant to one abi from the last step

  4. set abi
    ./box contract setabi 1
    NOTE: 1 is the abi index in list from output of "./box contract list"

  5. deploy contract 
    ./box contract deploy .cmd/contract/test/erc20_simple.bin 0 --rpc-port=19191
    NOTE: if add index argument after amount argument, attach index to the deployed contract

  6. attach index to an contract address
    ./box contract attach  b5kcrqGMZZ8yrxYs8TcGuv9wqvBFYHBmDTd --rpc-port=19191

  7. send a contract call transaction
    ./box contract send  0 approve 5623d8b0dd0136197531fd86110d509ce0030d9e 20000 --rpc-port=19191

  8. call a contract to get state value revalent to method via DoCall a contract
    ./box contract call  allowance 816666b318349468f8146e76e4e3751d937c14cb 5623d8b0dd0136197531fd86110d509ce0030d9e --rpc-port=19111

  9. decode output
    ./box contract decode allowance 0000000000000000000000000000000000000000000000000000000000004e20
`,
}

func init() {
	root.RootCmd.AddCommand(rootCmd)
	rootCmd.AddCommand(
		&cobra.Command{
			Use:   "importabi [abi_file_name] [description]",
			Short: "import abi for a contract.",
			Run:   importAbi,
		},
		&cobra.Command{
			Use: "list",
			Short: "list current sender, current abi,  all index revelant to imported" +
				" abi files and attached contracts",
			Run: list,
		},
		&cobra.Command{
			Use:   "setabi [index]",
			Short: "set abi index as default abi, index must be in abi list. relevant command: importabi, list",
			Run:   setabi,
		},
		&cobra.Command{
			Use:   "setsender [optinal|wallet_dir] [address]",
			Short: "set sender address to deploy or send contract",
			Run:   setsender,
		},
		&cobra.Command{
			Use:   "attach [contract_address]",
			Short: "attach contract abi to a live contract on chain",
			RunE:  attach,
		},
		&cobra.Command{
			Use:   "deploy [contract] [amount] [optional|index] [optional|args...]",
			Short: "Deploy a contract, contract can be a binary file, a solidity file or a hex string",
			Long:  "The return value is a hex-encoded transaction sequence and a contract address",
			Run:   deploy,
		},
		&cobra.Command{
			Use:   "send [amount] [method] [args...]",
			Short: "calling a contract",
			Long:  "Successful call will return a transaction hash value",
			Run:   send,
		},
		&cobra.Command{
			Use:   "call [optional|block_height] [method] [args...]",
			Short: "call contract value via docall.",
			Run:   call,
		},
		&cobra.Command{
			Use:   "reset",
			Short: "reset abi setting",
			Run:   reset,
		},
		&cobra.Command{
			Use:   "detailabi [optional|index/filename]",
			Short: "view contract details",
			Run:   detailAbi,
		},
		&cobra.Command{
			Use:   "compile [sol_file_path] [optional|ouput_file_path]",
			Short: "Compile contract source file",
			Run:   compile,
		},
	)
}

func importAbi(cmd *cobra.Command, args []string) {
	if len(args) != 2 {
		fmt.Println(cmd.Use)
		return
	}

	srcFile := args[0]
	if _, err := os.Stat(srcFile); err != nil {
		if os.IsNotExist(err) {
			fmt.Println("abi file is not exists")
		} else {
			fmt.Println("abi file status error:", err)
		}
		return
	}

	if err := util.MkDir(abiDir); err != nil {
		panic(err)
	}

	var iFiles []int
	err := filepath.Walk(abiDir, func(path string, info os.FileInfo, err error) error {
		if info.IsDir() {
			return nil
		}
		s := strings.TrimPrefix(path, abiDir)
		if i, err := strconv.Atoi(s); err == nil && i > 0 {
			iFiles = append(iFiles, i)
		}
		return nil
	})
	if err != nil {
		panic(err)
	}
	max := 0
	if len(iFiles) > 0 {
		for _, v := range iFiles {
			if max < v {
				max = v
			}
		}
	}
	newFile := abiDir + strconv.Itoa(max+1)
	data, err := ioutil.ReadFile(srcFile)
	if err != nil {
		panic(err)
	}
	if _, err := abi.JSON(bytes.NewBuffer(data)); err != nil {
		fmt.Println("illegal abi file, error:", err)
		return
	}
	// Write data to desc
	err = ioutil.WriteFile(newFile, data, 0644)
	if err != nil {
		panic(err)
	}
	f, err := os.OpenFile(recordFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	note := fmt.Sprintf("%d%s%s, %s%s%s\n", max+1, abiSep, args[1],
		time.Now().Format(time.Stamp), abiSep, newFile)
	if _, err := f.WriteString(note); err != nil {
		panic(err)
	}
	fmt.Println("index:", max+1)
	index, err := currentAbi()
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Printf("Current index: %d\n", index)
	fmt.Printf("Would you want to change current index into %d. [Y/n]\n", max+1)
	var input string
	fmt.Scanf("%s", &input)
	switch {
	case input == "n" || input == "N":
	default:
		setabi(&cobra.Command{}, []string{strconv.Itoa(max + 1)})
	}
}

func list(cmd *cobra.Command, args []string) {
	if len(args) > 0 {
		fmt.Println(cmd.Use)
		return
	}
	// show sender
	sender, _, _ := currentSender()
	fmt.Println("current sender:", sender)
	// show current index
	fmt.Printf("current index: ")
	index, err := currentAbi()
	if err == nil {
		fmt.Printf("%d", index)
	}
	fmt.Println()
	// show abi list
	abiInfo, attachedInfo, err := restoreRecord(recordFile)
	if err != nil {
		fmt.Println("restore record error:", err)
		return
	}
	if len(abiInfo) == 0 {
		fmt.Println("record is empty")
		return
	}
	fmt.Println("abi list:")
	for _, i := range abiInfo {
		fmt.Printf("\t%d: %s\n", i.index, i.note)
	}
	// show attached list
	if len(attachedInfo) == 0 {
		return
	}
	fmt.Println("contract attached list:")
	for i, c := range attachedInfo {
		fmt.Printf("\t%d > %s\n", i, c)
	}
}

func setsender(cmd *cobra.Command, args []string) {
	var (
		sender    string
		walletDir = common.DefaultWalletDir
	)
	if len(args) == 1 {
		sender = args[0]
	} else if len(args) == 2 {
		walletDir = args[0]
		sender = args[1]
	} else {
		fmt.Println(cmd.Use)
		return
	}
	// validate wallet dir
	if _, err := os.Stat(walletDir); err != nil {
		if os.IsNotExist(err) {
			fmt.Println("wallet directory is not exists")
		} else {
			fmt.Println("wallet directory status error:", err)
		}
		return
	}
	//validate address
	if _, err := types.NewAddress(sender); err != nil {
		fmt.Println("sender address is Invalid: ", err)
		return
	}
	wltMgr, err := wallet.NewWalletManager(walletDir)
	if err != nil {
		fmt.Println("open wallet failed:", err)
		return
	}
	var senderKeyFile string
	unlocked := false
	for _, acc := range wltMgr.ListAccounts() {
		if acc.Addr() == sender {
			senderKeyFile = acc.Path
			unlocked = true
			break
		}
	}
	if !unlocked {
		fmt.Printf("accound is unlocked, you need to import your account to the "+
			"wallet %s\n", walletDir)
		return
	}
	// write address to file
	if err := ioutil.WriteFile(senderFile, []byte(senderKeyFile+": "+sender), 0644); err != nil {
		panic(err)
	}
}

func setabi(cmd *cobra.Command, args []string) {
	if len(args) != 1 {
		fmt.Println(cmd.Use)
	}
	index, err := strconv.ParseUint(args[0], 10, 64)
	if err != nil {
		fmt.Println("invalid index: ", err)
		return
	}
	abiInfo, _, err := restoreRecord(recordFile)
	if err != nil {
		fmt.Println("restore record error:", err)
	}
	abiDesc := abiInfo.getItem(int(index))
	if abiDesc == nil {
		fmt.Printf("index %d is not in abi list\n", index)
		return
	}
	// write index to file
	if err := ioutil.WriteFile(abiIdxFile, []byte(args[0]), 0644); err != nil {
		panic(err)
	}
}

func attach(cmd *cobra.Command, args []string) error {
	// index
	var (
		contractAddr string
		index        int
		err          error
	)
	if len(args) == 1 {
		index, err = currentAbi()
		if err != nil {
			fmt.Println(err)
			return nil
		}
		contractAddr = args[0]
	} else if len(args) == 2 {
		index, err = strconv.Atoi(args[0])
		if err != nil {
			fmt.Println("invalid index:", err)
		}
		contractAddr = args[1]
	} else {
		fmt.Println(cmd.Use)
		return nil
	}
	// contract address
	address, err := types.NewContractAddress(contractAddr)
	if err != nil {
		fmt.Println("invalid contract address")
		return nil
	}
	// check whether index is record file
	abiInfos, _, err := restoreRecord(recordFile)
	if err != nil {
		panic(err)
	}
	abiInfo := abiInfos.getItem(index)
	if abiInfo == nil {
		fmt.Println("invalid index argument")
		return nil
	}
	// check contract existed
	resp, err := rpcutil.RPCCall(rpcpb.NewWebApiClient, "GetCode",
		&rpcpb.GetCodeReq{Address: address.String()}, common.GetRPCAddr())
	if err != nil {
		fmt.Printf("get code of contract %s error: %s\n", address, err)
		return err
	}
	getcodeResp := resp.(*rpcpb.GetCodeResp)
	if getcodeResp.Code != 0 {
		fmt.Printf("get code of contract %s error: %s\n", address, getcodeResp.Message)
		return errors.New(getcodeResp.Message)
	}
	if len(getcodeResp.Data) == 0 {
		fmt.Printf("get code of contract %s error: code is empty\n", address)
		return errors.New("contract code is empty")
	}
	// write record to file
	f, err := os.OpenFile(recordFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	note := fmt.Sprintf("%d> %s\n", index, address)
	if _, err := f.WriteString(note); err != nil {
		panic(err)
	}
	fmt.Printf("{%d \"%s\" %s} > %s\n", abiInfo.index, abiInfo.note, abiInfo.filepath, address)
	return nil
}

func deploy(cmd *cobra.Command, args []string) {
	if len(args) < 2 {
		fmt.Println(cmd.Use)
		return
	}
	amount, err := strconv.ParseUint(args[1], 10, 64)
	if err != nil {
		fmt.Println("invalid amount: ", err)
		return
	}
	data := args[0]
	//parase contract data
	bytecode, _, err := parseContractData(data)
	if err != nil {
		fmt.Println(err)
		return
	}
	// params
	var indexArg string
	if len(args) > 2 {
		index, err := strconv.Atoi(args[2])
		if err != nil {
			fmt.Println("invalid index:", err)
			return
		}
		indexArg = args[2]
		if len(args) > 3 {
			abiObj, err := newAbiObj(index)
			if err != nil {
				fmt.Println(err)
				return
			}
			data, err := encodeInput(abiObj, "", args[3:]...)
			if err != nil {
				fmt.Println("encode error:", err)
				return
			}
			bytecode += data
		}
	}
	// sender info
	sender, keyFile, bal, nonce, err := fetchSenderInfo()
	if err != nil {
		fmt.Println(err)
		return
	}
	if bal <= amount {
		fmt.Printf("balance of sender %s is %d that less than amount %d\n",
			sender, bal, amount)
		return
	}
	// do request
	req := &rpcpb.MakeContractTxReq{
		From:   sender,
		Amount: amount,
		// preserve extra fee for this tx
		GasLimit: calcGasLimit(bal, amount),
		Nonce:    nonce + 1,
		IsDeploy: true,
		Data:     bytecode,
	}
	acc, err := account.NewAccountFromFile(keyFile)
	if err != nil {
		fmt.Printf("unlock account: %s error: %s", sender, err)
		return
	}
	contractAddr, hash, err := signAndSendContractTx(req, acc)
	if err != nil {
		fmt.Println("sign and send transaction error:", err)
		return
	}
	fmt.Println("contract address:", contractAddr)
	fmt.Println("transaction hash:", hash)

	// if the index is given, attach it to the contract
	if indexArg != "" {
		t := time.NewTicker(500 * time.Millisecond)
		defer t.Stop()
		for i := 0; i < 10; i++ {
			select {
			case <-t.C:
				err := attach(&cobra.Command{}, []string{indexArg, contractAddr})
				if err == nil {
					return
				}
				fmt.Printf("attach %s to contract %s error: %s, try again[%d]\n",
					indexArg, contractAddr, err, i)
			}
		}
		fmt.Printf("attach %s to contract %s failed\n", indexArg, contractAddr)
	}
}

func send(cmd *cobra.Command, args []string) {
	if len(args) < 2 {
		fmt.Println(cmd.Use)
		return
	}
	// amount
	amount, err := strconv.ParseUint(args[0], 10, 64)
	if err != nil {
		fmt.Println("invalid amount: ", err)
		return
	}
	// index
	index, err := currentAbi()
	if err != nil {
		fmt.Println(err)
		fmt.Println("select an index in list via \"./box contract list\"")
		return
	}
	_, attachedInfo, err := restoreRecord(recordFile)
	if err != nil {
		fmt.Println("restore record error:", err)
		return
	}
	contractAddr, ok := attachedInfo[index]
	if !ok {
		fmt.Println("index is not attached to contract address")
		fmt.Println("attach abi to a contract via " +
			"(eg.) \"./box contract attach index contract_address --rpc-port 19111\"")
		return
	}
	// code
	abiObj, err := newAbiObj(index)
	if err != nil {
		fmt.Println(err)
		return
	}
	data, err := encodeInput(abiObj, args[1], args[2:]...)
	if err != nil {
		fmt.Println("encode error:", err)
		return
	}
	// sender info
	sender, keyFile, bal, nonce, err := fetchSenderInfo()
	if err != nil {
		fmt.Println(err)
		return
	}
	if bal <= amount {
		fmt.Printf("balance of sender %s is %d that less than amount %d\n",
			sender, bal, amount)
		return
	}
	// call
	req := &rpcpb.MakeContractTxReq{
		From:   sender,
		To:     contractAddr,
		Amount: amount,
		// preserve extra fee for this tx
		GasLimit: calcGasLimit(bal, amount),
		Nonce:    nonce + 1,
		Data:     data,
	}
	acc, err := account.NewAccountFromFile(keyFile)
	if err != nil {
		fmt.Printf("unlock account: %s error: %s", sender, err)
		return
	}
	_, hash, err := signAndSendContractTx(req, acc)
	if err != nil {
		fmt.Println("sign and send transaction error:", err)
		return
	}
	fmt.Println("send contract call successfully, transaction hash:", hash)
}

func call(cmd *cobra.Command, args []string) {
	if len(args) == 0 {
		fmt.Println(cmd.Use)
		return
	}
	var (
		index  int
		height uint64
		method string
		params []string
		err    error
	)
	// index
	index, err = currentAbi()
	if err != nil {
		fmt.Println(err)
		fmt.Println("select an index in list via \"./box contract list\"")
		return
	}
	// height
	height, err = strconv.ParseUint(args[0], 10, 64)
	if err != nil {
		method = args[0]
		params = args[1:]
	} else {
		method = args[1]
		params = args[2:]
	}
	// contract address
	_, attachedInfo, err := restoreRecord(recordFile)
	if err != nil {
		fmt.Println("restore record error:", err)
		return
	}
	contractAddr, ok := attachedInfo[index]
	if !ok {
		fmt.Println("index is not attached to contract address")
		fmt.Println("attach abi to a contract via " +
			"(eg.) \"./box contract attach index contract_address --rpc-port 19111\"")
		return
	}
	// code
	abiObj, err := newAbiObj(index)
	if err != nil {
		fmt.Println(err)
		return
	}
	data, err := encodeInput(abiObj, method, params...)
	if err != nil {
		fmt.Println("encode error:", err)
		return
	}
	// sender
	sender, _, err := currentSender()
	if err != nil {
		fmt.Println("get current sender error:", err)
		return
	}
	callReq := &rpcpb.CallReq{
		From: sender, To: contractAddr, Data: data, Height: uint32(height), Timeout: 2,
	}
	resp, err := rpcutil.RPCCall(rpcpb.NewWebApiClient, "DoCall", callReq, common.GetRPCAddr())
	if err != nil {
		fmt.Printf("rpc DoCall request %v error: %s\n", callReq, err)
		return
	}
	callResp := resp.(*rpcpb.CallResp)
	if callResp.Code != 0 {
		msg := fmt.Sprintf("rpc DoCall request %v error: %s", callReq, callResp.Message)
		output := resp.(*rpcpb.CallResp).GetOutput()
		if len(output) != 0 {
			msg += ", " + output
		}
		fmt.Println(msg)
		return
	}
	// decode output
	outputS, err := hex.DecodeString(callResp.GetOutput())
	if err != nil {
		fmt.Println("decode output to hex error:", err)
		return
	}
	output, err := abiObj.Methods[method].Outputs.UnpackValues(outputS)
	if err != nil {
		fmt.Println("decode output to values error:", err)
		return
	}
	fmt.Println("output:", output)
}

func signAndSendContractTx(
	req *rpcpb.MakeContractTxReq, acc *account.Account,
) (contractAddr, hash string, err error) {
	// make unsigned tx
	var resp interface{}
	resp, err = rpcutil.RPCCall(rpcpb.NewTransactionCommandClient,
		"MakeUnsignedContractTx", req, common.GetRPCAddr())
	if err != nil {
		err = fmt.Errorf("make unsigned contract tx req %+v error: %s", req, err)
		return
	}
	txResp := resp.(*rpcpb.MakeContractTxResp)
	if txResp.Code != 0 {
		err = fmt.Errorf("make unsigned contract tx req %+v error: %s", req, txResp.Message)
		return
	}
	hash, err = common.SignAndSendTx(txResp.GetTx(), txResp.GetRawMsgs(), acc)
	if err != nil {
		return
	}
	return txResp.GetContractAddr(), hash, nil
}

func reset(cmd *cobra.Command, args []string) {
	if err := os.RemoveAll(abiDir); err != nil {
		panic(err)
	}
	os.Remove(abiIdxFile)
}

type abiDesc struct {
	index    int
	note     string
	filepath string
}

type abiDesces []*abiDesc

func (abis abiDesces) getItem(idx int) *abiDesc {
	for _, abi := range abis {
		if abi.index == idx {
			return abi
		}
	}
	return nil
}

func restoreRecord(filepath string) (abiDesces, map[int]string, error) {
	// read file
	f, err := os.Open(filepath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, nil
		}
		return nil, nil, err
	}
	scanner := bufio.NewScanner(f)
	lines := make([]string, 0)
	for scanner.Scan() {
		l := scanner.Text()
		lines = append(lines, l)
	}
	if scanner.Err() != nil {
		return nil, nil, scanner.Err()
	}
	// parse
	abiInfo := make([]*abiDesc, 0)
	attachedInfo := make(map[int]string)
	for i, l := range lines {
		fields := strings.FieldsFunc(l, func(c rune) bool {
			if c == rune([]byte(abiSep)[0]) {
				return true
			}
			return false
		})
		if len(fields) == 3 {
			index, err := strconv.Atoi(fields[0])
			if err != nil {
				fmt.Printf("parse line %d abi num error: %s", i, err)
				continue
			}
			abiFile := strings.TrimSpace(fields[2])
			if _, err := os.Stat(abiFile); err != nil {
				fmt.Printf("parse line %d abi file stat error: %s", i, err)
				continue
			}
			abiInfo = append(abiInfo, &abiDesc{index: index, note: fields[1], filepath: abiFile})
			continue
		}
		//
		fields = strings.FieldsFunc(l, func(c rune) bool {
			if c == 62 { // 62 is the value of ">" in ascii
				return true
			}
			return false
		})
		if len(fields) == 2 {
			index, err := strconv.Atoi(fields[0])
			if err != nil {
				fmt.Printf("parse line %d attach num error: %s", i, err)
				continue
			}

			contractAddr := strings.TrimSpace(fields[1])

			if _, err := types.NewContractAddress(contractAddr); err != nil {
				fmt.Printf("parse line %d, parce contract address error: %s", i, err)
				continue
			}
			attachedInfo[index] = contractAddr
		}
	}

	return abiInfo, attachedInfo, nil
}

func encodeInput(abiObj *abi.ABI, method string, args ...string) (string, error) {
	var inputs abi.Arguments
	var m *abi.Method
	if method == "" {
		inputs = abiObj.Constructor.Inputs
	} else {
		methods := abiObj.MethodByName(method)
		// filter out functions with different number of args
		if len(methods) > 1 {
			i := 0
			for exit := false; !exit; {
				for ; ; i++ {
					m := methods[i]
					if len(m.Inputs) != len(args) {
						if i >= len(methods)-1 {
							exit = true
						}
						methods = append(methods[:i], methods[i+1:]...)
						break
					} else if i >= len(methods)-1 {
						exit = true
						break
					}
				}
			}
		}

		if len(methods) == 0 {
			// may method is an event name
			if event, exist := abiObj.Events[method]; exist {
				return event.ID().String(), nil
			}
			return "", fmt.Errorf("method or event %s is not found in abi file", method)
		} else if len(methods) == 1 {
			inputs = methods[0].Inputs
			m = methods[0]
		} else {
			m = chooseMethod(methods)
			if m != nil {
				inputs = m.Inputs
			}
		}
	}
	if len(inputs) != len(args) {
		return "", fmt.Errorf("argument count mismatch: %d for %d", len(args), len(inputs))
	}
	params := make([]interface{}, 0, len(args))
	for i, arg := range args {
		arg, err := parseAbiArg(inputs[i].Type, arg)
		if err != nil {
			return "", err
		}
		params = append(params, arg)
	}
	data, err := abiObj.Pack(m.Name, params...)
	if err == nil {
		return hex.EncodeToString(data), nil
	}
	// try event
	if event, ok := abiObj.Events[method]; ok {
		return hex.EncodeToString(event.ID().Bytes()), nil
	}
	return "", err
}

func chooseMethod(methods []*abi.Method) *abi.Method {

	if len(methods) == 0 {
		return nil
	}
	note := fmt.Sprintf("There are %d functions that you can use:\n", len(methods))
	for i, method := range methods {
		note += fmt.Sprintf("    %d. %s\n", i+1, method.String())
	}
	fmt.Print(note)

	fmt.Print("Input your choice:")
	for idx := -1; ; {
		fmt.Scanln(&idx)
		if idx < 1 || idx > len(methods) {
			fmt.Print("Please input the correct index: ")
		} else {
			return methods[idx-1]
		}
	}
}

func newAbiObj(index int) (*abi.ABI, error) {
	abiInfos, _, err := restoreRecord(recordFile)
	if err != nil {
		return nil, fmt.Errorf("parse abi record file error: %s", err)
	}
	abiInfo := abiInfos.getItem(index)
	if abiInfo == nil {
		return nil, errors.New("index not found in abi record")
	}
	return newAbiObjFromFile(abiInfo.filepath)
}

func fetchSenderInfo() (sender, keyFile string, bal, nonce uint64, err error) {
	// sender
	sender, keyFile, err = currentSender()
	if err != nil {
		return
	}
	// gaslimit
	var resp interface{}
	resp, err = rpcutil.RPCCall(rpcpb.NewTransactionCommandClient, "GetBalance",
		&rpcpb.GetBalanceReq{Addrs: []string{sender}}, common.GetRPCAddr())
	if err != nil {
		err = fmt.Errorf("get balance of sender %s error: %s", sender, err)
		return
	}
	balResp := resp.(*rpcpb.GetBalanceResp)
	if balResp.Code != 0 {
		err = fmt.Errorf("get balance of sender %s error: %s", sender, balResp.Message)
		return
	}
	bal = balResp.Balances[0]
	// nonce
	resp, err = rpcutil.RPCCall(rpcpb.NewWebApiClient, "Nonce",
		&rpcpb.NonceReq{Addr: sender}, common.GetRPCAddr())
	if err != nil {
		err = fmt.Errorf("get nonce of sender %s error: %s", sender, err)
		return
	}
	nonceResp := resp.(*rpcpb.NonceResp)
	if nonceResp.Code != 0 {
		err = fmt.Errorf("get nonce of sender %s error: %s", sender, nonceResp.Message)
		return
	}
	nonce = nonceResp.GetNonce()
	return
}

func currentSender() (sender, keyFile string, err error) {
	bytes, err := ioutil.ReadFile(senderFile)
	if err != nil {
		return "", "", errors.New("sender is not initialized, use \"setsender\" command to set")
	}
	fields := strings.FieldsFunc(string(bytes), func(c rune) bool {
		if c == 58 { // 58 is the value of ":" in ascii
			return true
		}
		return false
	})
	if len(fields) != 2 {
		return "", "", errors.New("damaged sender file")
	}
	keyFile, sender = strings.TrimSpace(fields[0]), strings.TrimSpace(fields[1])
	return
}

func currentAbi() (int, error) {
	data, err := ioutil.ReadFile(abiIdxFile)
	if err != nil {
		return 0, errors.New("abi index is not set, use \"setabi\" command to set")
	}
	index, err := strconv.Atoi(string(bytes.TrimSpace(data)))
	if err != nil {
		return 0, errors.New("get current abi error: " + err.Error())
	}
	abiInfo, _, err := restoreRecord(recordFile)
	if err != nil {
		return 0, errors.New("get current abi error: " + err.Error())
	}
	abiDesc := abiInfo.getItem(index)
	if abiDesc == nil {
		return 0, fmt.Errorf("current index %d set is not in abi list", index)
	}
	return index, nil
}

func parseAbiArg(typ abi.Type, arg string) (interface{}, error) {
	var (
		val interface{}
		err error
	)
	switch typ.T {
	case abi.BoolTy:
		val, err = strconv.ParseBool(arg)
		if err != nil {
			return nil, fmt.Errorf("encode abi: cannot use string as bool as argument, %s", err)
		}
	case abi.IntTy:
		i, err := strconv.ParseInt(arg, 0, 64)
		if err != nil {
			return nil, fmt.Errorf("encode abi: cannot use string as int as argument, %s", err)
		}
		val = big.NewInt(i)
	case abi.UintTy:
		i, err := strconv.ParseUint(arg, 0, 64)
		if err != nil {
			return nil, fmt.Errorf("encode abi: cannot use string as uint as argument, %s", err)
		}
		val = new(big.Int).SetUint64(i)
	case abi.AddressTy:
		if len(arg) != ripemd160.Size*2 {
			return nil, fmt.Errorf("encode abi: cannot use string as address as argument")
		}
		bytes, err := hex.DecodeString(arg)
		if err != nil {
			return nil, err
		}
		array := [ripemd160.Size]byte{}
		copy(array[:], bytes)
		val = array
	case abi.StringTy:
		val = arg
	case abi.BytesTy:
		val, err = hex.DecodeString(arg)
		if err != nil {
			return nil, fmt.Errorf("encode abi: cannot use string as address as argument, %s", err)
		}
	case abi.SliceTy, abi.ArrayTy:
		args, err := parseArrayArg(arg)
		if err != nil {
			return nil, err
		}
		var vals []interface{}
		for _, arg := range args {
			v, err := parseAbiArg(*typ.Elem, arg)
			if err != nil {
				return nil, err
			}
			vals = append(vals, v)
		}
		val = convertArrayType(typ.Elem.T, vals)
	}
	return val, nil
}

func parseArrayArg(arg string) ([]string, error) {
	arg = strings.TrimSpace(arg)
	if !strings.HasPrefix(arg, "[") || !strings.HasSuffix(arg, "]") {
		return nil, errors.New("invalid array format")
	}
	arg = arg[1 : len(arg)-1]
	args := strings.Split(arg, ",")
	for i := 0; i < len(args); i++ {
		args[i] = strings.TrimSpace(args[i])
	}
	return args, nil
}

func convertArrayType(typ byte, vals []interface{}) interface{} {
	switch typ {
	case abi.BoolTy:
		var val []bool
		for _, v := range vals {
			val = append(val, v.(bool))
		}
		return val
	case abi.IntTy, abi.UintTy:
		var val []*big.Int
		for _, v := range vals {
			val = append(val, v.(*big.Int))
		}
		return val
	case abi.AddressTy:
		var val [][ripemd160.Size]byte
		for _, v := range vals {
			val = append(val, v.([ripemd160.Size]byte))
		}
		return val
	case abi.StringTy:
		var val []string
		for _, v := range vals {
			val = append(val, v.(string))
		}
		return val
	case abi.BytesTy:
		var val [][]byte
		for _, v := range vals {
			val = append(val, v.([]byte))
		}
		return val
	case abi.SliceTy, abi.ArrayTy:
		panic(fmt.Errorf("convert array argument %+v type: %d error: "+
			"include multiple nested array", vals, typ))
	}
	panic(fmt.Errorf("parameters include unsupported type %d", typ))
}

func calcGasLimit(bal, amount uint64) uint64 {
	limit := (bal - amount - core.TransferFee) / core.FixedGasPrice
	if limit > 100000000 {
		limit = 100000000
	}
	return limit
}

func parseContractData(data string) (bytecode string, abi string, err error) {
	//parase data
	if _, err := os.Stat(data); err == nil {
		bytes, err := ioutil.ReadFile(data)
		if err != nil {
			return "", "", err
		}
		bytesStr := strings.TrimSpace(string(bytes))
		if common.IsHexFormat(bytesStr) {
			bytecode = bytesStr
		} else {
			binData, abiData, solName, err := compileSol(data)
			if err != nil {
				return "", "", err
			}
			if len(binData) == 1 {
				bytecode = string(binData[0])
				abi = string(abiData[0])
			} else if len(binData) > 1 {
				for i, v := range solName {
					fmt.Printf("%d:%s\n", i, v)
				}
				fmt.Println("Which one do you want to deploy?")
				var input string
				fmt.Scanf("%s", &input)
				for i, v := range solName {
					if input == v || input == strconv.Itoa(i) {
						bytecode = string(binData[i])
						abi = string(abiData[i])
					}
				}
				//if len(bytecode) == 0, there is no match in compiling sol file
				if len(bytecode) == 0 {
					return "", "", errors.New("Please make sure your input is correct")
				}
			}
		}
	} else {
		if !common.IsHexFormat(data) {
			return "", "", err
		}
		bytecode = data
	}
	return bytecode, abi, nil
}

func compileSol(filepath string) (binData [][]byte, abiData [][]byte, solName []string, err error) {
	cmd := exec.Command("solc", filepath, "--bin", "--abi")
	//CombinedOutput runs the command and returns its combined standard output and standard error.
	output, err := cmd.CombinedOutput()
	if err != nil {
		//an error is returned and the error message is the output.
		return nil, nil, nil, errors.New(string(output))
	}
	binMatches := solcBinReg.FindAllSubmatch(output, -1)
	for _, v := range binMatches {
		binData = append(binData, v[1])
	}
	abiMathes := solcAbiReg.FindAllSubmatch(output, -1)
	for _, v := range abiMathes {
		abiData = append(abiData, v[1])
	}
	solFilePath = filepath
	nameMatches := solNameReg.FindAllSubmatch(output, -1)
	for _, v := range nameMatches {
		solName = append(solName, string(v[1]))
	}
	if len(abiData) == 0 || len(binData) == 0 || len(solName) == 0 {
		return nil, nil, nil, errors.New("no matche in sol file")
	}
	return binData, abiData, solName, nil
}

func newAbiObjFromFile(filePath string) (*abi.ABI, error) {
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read file : %s, err: %s", filePath, err)
	}
	abi, err := abi.JSON(bytes.NewBuffer(data))
	if err != nil {
		return nil, fmt.Errorf("illegal abi file, error: %s", err)
	}
	return &abi, nil
}

func detailAbi(cmd *cobra.Command, args []string) {
	if len(args) > 1 {
		fmt.Println(cmd.Use)
		return
	}
	var (
		abiObj  *abi.ABI
		abiFile string
		index   = -1
		err     error
	)
	if len(args) == 0 {
		if index, err = currentAbi(); err == nil {
			abiObj, err = newAbiObj(index)
			if err != nil {
				fmt.Println("get new abi object from index error:", err)
				return
			}
		} else {
			fmt.Println("abi index is not set, use \"setabi\" command to set")
			return
		}
	}
	if len(args) == 1 {
		fileName := args[0]
		if err := util.FileExists(fileName); err == nil {
			abiObj, err = newAbiObjFromFile(fileName)
			if err != nil {
				fmt.Printf("new abi object from file %s error:%s\n", fileName, err)
				return
			}
			abiFile = fileName
		} else {
			index, err = strconv.Atoi(args[0])
			if err != nil {
				fmt.Println("invaild index:", err)
				return
			}
			abiObj, err = newAbiObj(index)
			if err != nil {
				fmt.Println("get new abi object from index error:", err)
				return
			}
		}
	}
	if index >= 0 {
		abiInfo, _, _ := restoreRecord(recordFile)
		abiDesc := abiInfo.getItem(index)
		abiFile = abiDesc.filepath
	}
	payableMap, err := parseAbiPayable(abiFile)
	if err != nil {
		fmt.Println("parse payable info error:", err)
		return
	}
	fmt.Println("methods:")
	for _, method := range abiObj.Methods {
		inputs := make([]string, len(method.Inputs))
		for i, input := range method.Inputs {
			inputs[i] = fmt.Sprintf("%v %v", input.Type, input.Name)
		}
		outputs := make([]string, len(method.Outputs))
		for i, output := range method.Outputs {
			outputs[i] = output.Type.String()
			if len(output.Name) > 0 {
				outputs[i] += fmt.Sprintf(" %v", output.Name)
			}
		}
		constant := ""
		if method.Const {
			constant = " constant"
		}
		payable := ""
		if _, ok := payableMap[method.Name]; ok {
			payable = " payable"
		}

		fmt.Printf("  %x:  %v(%v)%s%s", method.ID(), method.Name,
			strings.Join(inputs, ", "), constant, payable)
		if len(outputs) > 0 {
			fmt.Printf(" returns (%v)", strings.Join(outputs, ", "))
		}
		fmt.Printf("\n")
	}

	fmt.Println()
	fmt.Println("events:")
	for _, e := range abiObj.Events {
		inputs := make([]string, len(e.Inputs))
		for i, input := range e.Inputs {
			inputs[i] = fmt.Sprintf("%v %v", input.Type, input.Name)
			if input.Indexed {
				inputs[i] = fmt.Sprintf("%v indexed %v", input.Type, input.Name)
			}
		}
		fmt.Printf("  %s:  %v(%v)\n", e.ID(), e.Name, strings.Join(inputs, ", "))
	}
}

func compile(cmd *cobra.Command, args []string) {
	if len(args) == 0 || len(args) > 2 {
		fmt.Println(cmd.Use)
		return
	}
	filepath := args[0]
	if err := util.FileExists(filepath); err != nil {
		fmt.Println(err)
		return
	}
	var outputDir string
	if len(args) == 1 {
		currentFilePath, err := os.Getwd()
		if err != nil {
			fmt.Println(err)
			return
		}
		outputDir = currentFilePath + "/"
	} else if len(args) == 2 {
		outputDir = args[1] + "/"
		if _, err := os.Stat(outputDir); err != nil {
			fmt.Printf("output dir %s error: %s\n", outputDir, err)
			return
		}
	}
	binData, abiData, solName, err := compileSol(filepath)
	if err != nil {
		fmt.Println(err)
		return
	}
	for i := 0; i < len(binData); i++ {
		binFile := outputDir + solName[i] + ".bin"
		// write bin data to file
		if err := ioutil.WriteFile(binFile, binData[i], 0644); err != nil {
			fmt.Println(err)
			return
		}
		fmt.Println("generate bin file:", binFile)
		abiFile := outputDir + solName[i] + ".abi"
		// write abi data to file
		if err := ioutil.WriteFile(abiFile, abiData[i], 0644); err != nil {
			fmt.Println(err)
			return
		}
		fmt.Println("generate abi file:", abiFile)
	}
}
