// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package contractcmd

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"strconv"
	"strings"

	"github.com/BOXFoundation/boxd/commands/box/common"
	"github.com/BOXFoundation/boxd/core"
	"github.com/BOXFoundation/boxd/core/abi"
	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/crypto"
	rpcpb "github.com/BOXFoundation/boxd/rpc/pb"
	"github.com/BOXFoundation/boxd/rpc/rpcutil"
	"github.com/BOXFoundation/boxd/util"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(
		&cobra.Command{
			Use: "list",
			Short: "list current sender, current abi,  all index revelant to imported" +
				" abi files and attached contracts",
			Run: list,
		},
		&cobra.Command{
			Use:   "encode [optional|index] [method] [args...]",
			Short: "Get an input string to send or call",
			Run:   encode,
		},
		&cobra.Command{
			Use:   "decode [optional|index] [topic/method] [optional|data]",
			Short: "decode",
			Run:   decode,
			Example: `  1 ./box contract decode index Method
    ./box contract decode 7 6dd7d8ea000000000000000000000000ce86056786e3415530f8cc739fb414a87435b4b6(data from method and args encode)
  2 ./box contract decode index topic
    ./box contract decode 7 2207c1818549bfd6420c96be05b47e8fbdd336a22cd20f069ecba206e474aa7a
  3 ./box contract decode index method return_value
    ./box contract decode 6 allowance 0000000000000000000000000000000000000000000000000000000000004e20
  4 ./box contract decode index topics data (currently only support topics of event name, not to support args indexed topic.)
    ./box contract decode 2207c1818549bfd6420c96be05b47e8fbdd336a22cd20f069ecba206e474aa7a 000000000000000000000000ae3e96d008658db64dd4f8df2d736edbc6be1c31000000000000000000000000000000000000000000000000000000012a15f790`,
		},
		&cobra.Command{
			Use:   "getlogs [hash] [from] [to] [address] [topics]",
			Short: "Get returns logs matching the given argument that are stored within the state",
			Run:   getLogs,
		},
		&cobra.Command{
			Use:   "getnonce [addr]",
			Short: "Get the nonce of address ",
			Run:   getNonce,
		},
		&cobra.Command{
			Use:   "getcode [contractaddress]",
			Short: "Get the code of contract_address",
			Run:   getCode,
		},
		&cobra.Command{
			Use:   "estimategas [from] [to] [data] [optinal|height]",
			Short: "Get estimategas about contract_transaction",
			Run:   estimateGas,
		},
		&cobra.Command{
			Use:   "getstorage [address] [position] [optinal|height]",
			Short: "Get the position of variable in storsge",
			Run:   getStorageAt,
		},
		&cobra.Command{
			Use:   "detailabi [optional|index/filename]",
			Short: "view contract details",
			Run:   detailAbi,
		},
	)
}

func encode(cmd *cobra.Command, args []string) {
	if len(args) < 1 {
		fmt.Println(cmd.Use)
		return
	}
	var (
		method    string
		arguments []string
	)
	// check abi index
	index, err := strconv.Atoi(args[0])
	if err != nil {
		index, err = currentAbi()
		if err != nil {
			fmt.Printf("abi index is not passed as argument or set before correctly(%s)\n", err)
			return
		}
		method = args[0]
		arguments = args[1:]
	} else {
		method = args[1]
		arguments = args[2:]
	}
	abiObj, err := newAbiObj(index)
	if err != nil {
		fmt.Println(err)
		return
	}
	data, err := encodeInput(abiObj, method, arguments...)
	if err != nil {
		fmt.Println("encode error:", err)
		return
	}
	fmt.Println(data)
}

func decode(cmd *cobra.Command, args []string) {
	if len(args) == 0 || len(args) > 3 {
		fmt.Println(cmd.Use)
		return
	}
	var priArg, subArg string
	// check abi index
	index, err := strconv.Atoi(args[0])
	if err != nil {
		index, err = currentAbi()
		if err != nil {
			fmt.Printf("abi index is not passed as argument or set before correctly(%s)\n", err)
			return
		}
		priArg = args[0]
		if len(args) == 2 {
			subArg = args[1]
		}
	} else {
		priArg = args[1]
		if len(args) == 3 {
			subArg = args[2]
		}
	}
	abiObj, err := newAbiObj(index)
	if err != nil {
		fmt.Println(err)
		return
	}
	// for cases: 1) method code as input, 2) topic code from event name
	if subArg == "" {
		hash := new(crypto.HashType)
		if err = hash.SetString(priArg); err == nil {
			//topic code from event name
			for _, event := range abiObj.Events {
				if event.ID().String() == priArg {
					fmt.Println("event name:", event.Name)
					return
				}
			}
		} else {
			//method code as input
			if len(priArg) < 4 {
				fmt.Printf("method code %s must be not less than 4 bytes\n", priArg)
				return
			}
			data, err := hex.DecodeString(priArg)
			if err != nil {
				fmt.Println("hex format data is needed for method code", priArg)
				return
			}
			method, err := abiObj.MethodByID(data)
			if err != nil {
				fmt.Printf("%s may be method code, but the method cannot be found in "+
					"%d abi file\n", priArg, index)
				return
			}
			fmt.Printf("method name: \"%s\"\n", method.Name)
			arguments, err := method.Inputs.UnpackValues(data[4:])
			if err != nil {
				fmt.Printf("decode arguments of method %s failed: %s\n", data, err)
				return
			}
			for i, value := range arguments {
				input := method.Inputs[i]
				if addr, ok := value.(types.AddressHash); ok {
					fmt.Printf("argument: \"%s\", type: \"%s\", value: \"%x\"\n",
						input.Name, input.Type, addr[:])
				} else {
					fmt.Printf("argument: \"%s\", type: \"%s\", value: \"%v\"\n",
						input.Name, input.Type, value)
				}
			}
		}
	} else {
		// for cases: 3) method name and return value, 4) topic code and event argument
		data, err := hex.DecodeString(subArg)
		if err != nil {
			fmt.Println("hex format data is needed for argument", subArg)
			return
		}
		code := priArg
		hash := new(crypto.HashType)
		//if args[1] is the type of hash, decode log_topics
		if err = hash.SetString(code); err == nil {
			var event *abi.Event
			//topic code and event argument
			for _, e := range abiObj.Events {
				if e.ID().String() == code {
					event = &e
					break
				}
			}
			if event == nil {
				fmt.Printf("%s may be topic hash, but the method cannot be found in %d"+
					" abi file\n", code, index)
				return
			}
			fmt.Println("event name:", event.Name)
			// decode log_data
			arguments, err := abiObj.Events[event.Name].Inputs.UnpackValues(data)
			if err != nil {
				fmt.Printf("decode arguments of event %s failed: %s\n", data, err)
				return
			}
			for i, value := range arguments {
				input := event.Inputs[i]
				if addr, ok := value.(types.AddressHash); ok {
					fmt.Printf("argument: \"%s\", type: \"%s\", value: \"%x\"\n",
						input.Name, input.Type, addr[:])
				} else {
					fmt.Printf("argument: \"%s\", type: \"%s\", value: \"%v\"\n",
						input.Name, input.Type, value)
				}
			}
		} else {
			//method name and return value
			method, ok := abiObj.Methods[code]
			if !ok {
				fmt.Printf("%s: cannot be found in %d abi file\n", priArg, index)
				return
			}
			output, err := method.Outputs.UnpackValues(data)
			if err != nil {
				fmt.Printf("decode return value of method %s failed: %s\n", data, err)
				return
			}
			fmt.Println("return value:", output)
		}
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

func getLogs(cmd *cobra.Command, args []string) {
	//arg[0]represents block hash , arg[1] "from"andarg[2"to " represent log between from and to
	//arg[3]reprensents address arg[4]represents topics
	if len(args) != 5 {
		fmt.Println(cmd.Use)
		return
	}
	hash := new(crypto.HashType)
	if err := hash.SetString(args[0]); err != nil {
		fmt.Println("invalid hash")
		return
	}
	from, err := strconv.ParseUint(args[1], 10, 32)
	if err != nil {
		fmt.Println(err)
		return
	}
	to, err := strconv.ParseUint(args[2], 10, 32)
	if err != nil {
		fmt.Println(err)
		return
	}
	address := strings.Split(args[3], ",")
	topicsStr := strings.Split(args[4], ",")
	topics := []*rpcpb.LogsReqTopiclist{&rpcpb.LogsReqTopiclist{Topics: topicsStr}}

	req := &rpcpb.LogsReq{
		Uid:       "",
		Hash:      hash.String(),
		From:      uint32(from),
		To:        uint32(to),
		Addresses: address,
		Topics:    topics,
	}
	respRPC, err := rpcutil.RPCCall(rpcpb.NewWebApiClient, "GetLogs", req, common.GetRPCAddr())
	if err != nil {
		fmt.Println(err)
		return
	}
	resp := respRPC.(*rpcpb.Logs)
	if resp.Code != 0 {
		fmt.Println(resp.Message)
		return
	}
	for _, log := range resp.Logs {
		fmt.Println(log)
	}
}

func getNonce(cmd *cobra.Command, args []string) {
	if len(args) != 1 {
		fmt.Println(cmd.Use)
		return
	}
	addr := args[0]
	//validate address
	if address, err := types.ParseAddress(addr); err != nil {
		_, ok1 := address.(*types.AddressPubKeyHash)
		_, ok2 := address.(*types.AddressContract)
		if !ok1 && !ok2 {
			fmt.Printf("invaild address for %s, err: %s\n", args[0], err)
			return
		}
	}
	respRPC, err := rpcutil.RPCCall(rpcpb.NewWebApiClient, "Nonce",
		&rpcpb.NonceReq{Addr: addr}, common.GetRPCAddr())
	if err != nil {
		fmt.Println(err)
		return
	}
	resp := respRPC.(*rpcpb.NonceResp)
	if resp.Code != 0 {
		fmt.Println(resp.Message)
		return
	}
	fmt.Println("Nonce:", resp.Nonce)
}

func getCode(cmd *cobra.Command, args []string) {
	if len(args) != 1 {
		fmt.Println(cmd.Use)
		return
	}
	//validate address
	if _, err := types.NewContractAddress(args[0]); err != nil {
		fmt.Println("invalid contract address")
		return
	}
	respRPC, err := rpcutil.RPCCall(rpcpb.NewWebApiClient, "GetCode",
		&rpcpb.GetCodeReq{Address: args[0]}, common.GetRPCAddr())
	if err != nil {
		fmt.Println(err)
		return
	}
	resp := respRPC.(*rpcpb.GetCodeResp)
	if resp.Code != 0 {
		fmt.Println(resp.Message)
		return
	}
	fmt.Println(resp.Data)
}

func estimateGas(cmd *cobra.Command, args []string) {
	if len(args) < 3 {
		fmt.Println(cmd.Use)
		return
	}
	from := args[0]
	toAddr := args[1]
	//check address
	if _, err := types.NewAddress(args[0]); err != nil {
		fmt.Println("invalid address")
		return
	}
	if _, err := types.NewContractAddress(args[1]); err != nil {
		fmt.Println("invalid contract address")
		return
	}
	var height uint64
	if len(args) == 3 {
		height = 0
	} else {
		var err error
		height, err = strconv.ParseUint(args[3], 10, 64)
		if err != nil {
			fmt.Println(err)
			return
		}
	}
	req := &rpcpb.CallReq{
		From:    from,
		To:      toAddr,
		Data:    args[2],
		Height:  uint32(height),
		Timeout: 0,
	}
	respRPC, err := rpcutil.RPCCall(rpcpb.NewWebApiClient, "EstimateGas",
		req, common.GetRPCAddr())
	if err != nil {
		fmt.Println(err)
		return
	}
	resp, ok := respRPC.(*rpcpb.EstimateGasResp)
	if !ok {
		fmt.Println("Conversion to rpcpb.EstimateGasResp failed")
		return
	}
	if resp.Code != 0 {
		fmt.Println(resp.Message)
		return
	}
	fmt.Println("Estimate box:", uint64(resp.Gas)*core.FixedGasPrice/core.DuPerBox)
}

func getStorageAt(cmd *cobra.Command, args []string) {
	if len(args) != 3 {
		fmt.Println(cmd.Use)
		return
	}
	_, err := types.NewContractAddress(args[0])
	if err != nil {
		fmt.Println("invalid contract address")
		return
	}
	height, err := strconv.ParseUint(args[2], 10, 64)
	if err != nil {
		fmt.Println("Conversion the type of height failed:", err)
		return
	}
	req := &rpcpb.StorageReq{
		Address:  args[0],
		Position: args[1],
		Height:   uint32(height),
	}
	respRPC, err := rpcutil.RPCCall(rpcpb.NewWebApiClient, "GetStorageAt", req, common.GetRPCAddr())
	if err != nil {
		fmt.Println("RPC call failed:", err)
		return
	}
	resp, ok := respRPC.(*rpcpb.StorageResp)
	if !ok {
		fmt.Println("Conversion to rpcpb.StorageResp failed")
		return
	}
	if resp.Code != 0 {
		fmt.Println(resp.Message)
		return
	}
	fmt.Println(resp.Data)
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

func parseAbiPayable(file string) (map[string]bool, error) {
	var payableInfo []struct {
		Name    string `json:"name"`
		Type    string `json:"type"`
		Payable bool   `json:"payable"`
	}
	data, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(data, &payableInfo); err != nil {
		return nil, err
	}
	m := make(map[string]bool)
	for _, v := range payableInfo {
		if v.Type == "function" {
			m[v.Name] = v.Payable
		}
	}
	return m, nil
}
