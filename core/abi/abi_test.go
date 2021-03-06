// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package abi

import (
	"encoding/hex"
	"fmt"
	"strings"
	"testing"
)

func TestPack(t *testing.T) {
	const definition = `[{"constant":true,"inputs":[],"name":"minter","outputs":[{"name":"","type":"address"}],"payable":false,"stateMutability":"view","type":"function"},{"constant":true,"inputs":[{"name":"","type":"address"}],"name":"balances","outputs":[{"name":"","type":"uint256"}],"payable":false,"stateMutability":"view","type":"function"},{"constant":false,"inputs":[{"name":"receiver","type":"address"},{"name":"amount","type":"uint256"}],"name":"mint","outputs":[],"payable":false,"stateMutability":"nonpayable","type":"function"},{"constant":false,"inputs":[{"name":"receiver","type":"address"},{"name":"amount","type":"uint256"}],"name":"send","outputs":[],"payable":false,"stateMutability":"nonpayable","type":"function"},{"inputs":[],"payable":false,"stateMutability":"nonpayable","type":"constructor"},{"anonymous":false,"inputs":[{"indexed":false,"name":"from","type":"address"},{"indexed":false,"name":"to","type":"address"},{"indexed":false,"name":"amount","type":"uint256"}],"name":"Sent","type":"event"}]`

	aaa, err := JSON(strings.NewReader(definition))
	if err != nil {
		t.Error(err)
	}

	minter := aaa.Methods["balances"]
	outputs := minter.Outputs

	ret := outputs[0]

	fmt.Println("ret.Type.Type: ", ret.Type.Type)
}

// TestUnpackEvent is based on this contract:
// pragma solidity ^0.5.6;
// contract Coin {
//
//     event Sent(string amount);
//     event Sent2(string amount, uint num);
//
//     function send(string memory amount) public {
//         emit Sent(amount);
//         emit Sent2(amount, 12345);
//     }
// }
func TestUnpackEvent(t *testing.T) {
	const abiJSON = `[{"constant":false,"inputs":[{"name":"amount","type":"string"}],"name":"send","outputs":[],"payable":false,"stateMutability":"nonpayable","type":"function"},{"anonymous":false,"inputs":[{"indexed":false,"name":"amount","type":"string"}],"name":"Sent","type":"event"},{"anonymous":false,"inputs":[{"indexed":false,"name":"amount","type":"string"},{"indexed":false,"name":"num","type":"uint256"}],"name":"Sent2","type":"event"}]`
	abi, err := JSON(strings.NewReader(abiJSON))
	if err != nil {
		t.Fatal(err)
	}

	hexdata := `0000000000000000000000000000000000000000000000000000000000000020000000000000000000000000000000000000000000000000000000000000000a6162636428292829282900000000000000000000000000000000000000000000`
	data, err := hex.DecodeString(hexdata)
	if err != nil {
		t.Fatal(err)
	}

	var a string
	err = abi.Unpack(&a, "Sent", data)
	fmt.Println(err)
	fmt.Println(a)

	fmt.Println("--------------------")

	hexdata = `00000000000000000000000000000000000000000000000000000000000000400000000000000000000000000000000000000000000000000000000000003039000000000000000000000000000000000000000000000000000000000000000a6162636428292829282900000000000000000000000000000000000000000000`
	data, err = hex.DecodeString(hexdata)

	res, err := abi.Events["Sent2"].Inputs.UnpackValues(data)
	fmt.Println(err)
	fmt.Println(res)

}

// TestTuple is based on this contract:
// pragma solidity >0.4.23 <0.7.0;
// contract C {
//    uint[] data;
//
//   function f() public pure returns (uint, bool, uint) {
//        return (7, true, 2);
//    }
//}
func TestTuple(t *testing.T) {
	const abiJSON = `[{"constant":true,"inputs":[],"name":"f","outputs":[{"name":"","type":"uint256"},{"name":"","type":"bool"},{"name":"","type":"uint256"}],"payable":false,"stateMutability":"pure","type":"function"}]`
	abi, err := JSON(strings.NewReader(abiJSON))
	if err != nil {
		t.Fatal(err)
	}

	hexdata := `000000000000000000000000000000000000000000000000000000000000000700000000000000000000000000000000000000000000000000000000000000010000000000000000000000000000000000000000000000000000000000000002`
	data, err := hex.DecodeString(hexdata)
	if err != nil {
		t.Fatal(err)
	}

	res, _ := abi.Methods["f"].Outputs.UnpackValues(data)
	fmt.Println(res)
}

// TestStruct is based on this contract:
// pragma solidity ^0.5.9;
// pragma experimental ABIEncoderV2;
//
// contract A{
//     struct S{
//         string para1;
//         M para2;
//     }
//     struct M{
//         string para1;
//         int para2;
//     }
//
//     mapping (uint32=>S) public aaa;
//     constructor() public {
//         aaa[0] = S("Test", M("M", 10));
//     }
// }
func TestStruct(t *testing.T) {
	const abiJSON = `[{"inputs":[],"payable":false,"stateMutability":"nonpayable","type":"constructor"},{"constant":true,"inputs":[{"name":"","type":"uint32"}],"name":"aaa","outputs":[{"name":"para1","type":"string"},{"components":[{"name":"para1","type":"string"},{"name":"para2","type":"int256"}],"name":"para2","type":"tuple"}],"payable":false,"stateMutability":"view","type":"function"}]`
	abi, err := JSON(strings.NewReader(abiJSON))
	if err != nil {
		t.Fatal(err)
	}

	hexdata := `00000000000000000000000000000000000000000000000000000000000000400000000000000000000000000000000000000000000000000000000000000080000000000000000000000000000000000000000000000000000000000000000454657374000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000040000000000000000000000000000000000000000000000000000000000000000a00000000000000000000000000000000000000000000000000000000000000014d00000000000000000000000000000000000000000000000000000000000000`
	data, err := hex.DecodeString(hexdata)
	if err != nil {
		t.Fatal(err)
	}

	res, _ := abi.Methods["aaa"].Outputs.UnpackValues(data)
	a := fmt.Sprint(res)
	fmt.Println(a)
}
