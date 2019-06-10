// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package abi

import (
	"encoding/hex"
	"fmt"
	"reflect"
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

	a := 100
	fmt.Println(reflect.TypeOf(a))
	fmt.Println(reflect.ValueOf(&a))
	fmt.Println(reflect.ValueOf(&a).Elem())

	fmt.Println("reflect b: ")
	b := reflect.Zero(reflect.TypeOf(a))
	b.Elem().Interface()
	fmt.Println(b)
	fmt.Println(b.Kind())

}

// TestUnpackEvent is based on this contract:
//    contract T {
//      event received(address sender, uint amount, bytes memo);
//      event receivedAddr(address sender);
//      function receive(bytes memo) external payable {
//        received(msg.sender, msg.value, memo);
//        receivedAddr(msg.sender);
//      }
//    }
// When receive("X") is called with sender 0x00... and value 1, it produces this tx receipt:
//   receipt{status=1 cgas=23949 bloom=00000000004000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000800000000000000000000000000000000000040200000000000000000000000000000000001000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000080000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000 logs=[log: b6818c8064f645cd82d99b59a1a267d6d61117ef [75fd880d39c1daf53b6547ab6cb59451fc6452d27caa90e5b6649dd8293b9eed] 000000000000000000000000376c47978271565f56deb45495afa69e59c16ab200000000000000000000000000000000000000000000000000000000000000010000000000000000000000000000000000000000000000000000000000000060000000000000000000000000000000000000000000000000000000000000000158 9ae378b6d4409eada347a5dc0c180f186cb62dc68fcc0f043425eb917335aa28 0 95d429d309bb9d753954195fe2d69bd140b4ae731b9b5b605c34323de162cf00 0]}
func TestUnpackEvent(t *testing.T) {
	const abiJSON = `[{"constant":false,"inputs":[{"name":"_greeting","type":"*string"}],"name":"setGreeting","outputs":[],"payable":false,"stateMutability":"nonpayable","type":"function"},{"constant":true,"inputs":[],"name":"greet","outputs":[{"name":"","type":"string"}],"payable":false,"stateMutability":"view","type":"function"},{"constant":true,"inputs":[],"name":"greeting","outputs":[{"name":"","type":"string"}],"payable":false,"stateMutability":"view","type":"function"},{"inputs":[],"payable":false,"stateMutability":"nonpayable","type":"constructor"},{"anonymous":false,"inputs":[{"indexed":true,"name":"_greeting","type":"string"}],"name":"setted","type":"event"}]`
	abi, err := JSON(strings.NewReader(abiJSON))
	if err != nil {
		t.Fatal(err)
	}

	hexdata := `0000000000000000000000000000000000000000000000000000000000000020000000000000000000000000000000000000000000000000000000000000000d6f6e65206d6f72652074696d6500000000000000000000000000000000000000`
	data, err := hex.DecodeString(hexdata)
	if err != nil {
		t.Fatal(err)
	}

	typ, err := NewType("string", nil)
	a := reflect.New(typ.Type)
	b := a.Elem().Interface()
	err = abi.Unpack(&b, "greet", data)

	fmt.Println(err)
	fmt.Println(b)

}

func TestUnpack1(t *testing.T) {

	var aaa string

	fmt.Println(reflect.TypeOf(aaa).AssignableTo(reflect.TypeOf(aaa)))

	fmt.Println(reflect.TypeOf(&aaa))
	fmt.Println(reflect.ValueOf(&aaa))
	fmt.Println(reflect.ValueOf(&aaa).Elem())
	fmt.Println(reflect.ValueOf(&aaa).Elem().Addr())
	fmt.Println(reflect.ValueOf(&aaa).Elem().Addr().Interface())
	fmt.Println(reflect.ValueOf(reflect.ValueOf(&aaa).Elem().Addr().Interface()))
	fmt.Println(reflect.ValueOf(reflect.ValueOf(&aaa).Elem().Addr().Interface()).Elem())
	fmt.Println(reflect.ValueOf(reflect.ValueOf(&aaa).Elem().Addr().Interface()).Elem().CanSet())
	fmt.Println(reflect.ValueOf(reflect.ValueOf(&aaa).Elem().Addr().Interface()).Elem().Type())

	fmt.Println("-----------------")

	a := reflect.New(reflect.TypeOf(aaa))
	b := a.Elem().Interface()

	fmt.Println(reflect.TypeOf(&b))
	fmt.Println(reflect.ValueOf(&b))
	fmt.Println(reflect.ValueOf(&b).Elem())
	fmt.Println(reflect.ValueOf(&b).Elem().Addr())
	fmt.Println(reflect.ValueOf(&b).Elem().Addr().Interface())
	fmt.Println(reflect.ValueOf(reflect.ValueOf(&b).Elem().Addr().Interface()))
	fmt.Println(reflect.ValueOf(reflect.ValueOf(&b).Elem().Addr().Interface()).Elem())
	fmt.Println(reflect.ValueOf(reflect.ValueOf(&b).Elem().Addr().Interface()).Elem().CanSet())
	fmt.Println(reflect.ValueOf(reflect.ValueOf(&b).Elem().Addr().Interface()).Elem().Type())
	fmt.Println(reflect.ValueOf(reflect.ValueOf(&b).Elem().Addr().Interface()).Elem().Elem().CanSet())

}

// TestUnpackEvent is based on this contract:
// pragma solidity >0.4.23 <0.7.0;
// contract C {
//    uint[] data;
//
//   function f() public pure returns (uint, bool, uint) {
//        return (7, true, 2);
//    }
//}
func TestTuple(t *testing.T) {
	const abiJSON = `[
		{
			"constant": true,
			"inputs": [],
			"name": "f",
			"outputs": [
				{
					"name": "",
					"type": "uint256"
				},
				{
					"name": "",
					"type": "bool"
				},
				{
					"name": "",
					"type": "uint256"
				}
			],
			"payable": false,
			"stateMutability": "pure",
			"type": "function"
		}
	]`
	abi, err := JSON(strings.NewReader(abiJSON))
	if err != nil {
		t.Fatal(err)
	}

	hexdata := `6080604052348015600f57600080fd5b5060a38061001e6000396000f3fe6080604052348015600f57600080fd5b506004361060285760003560e01c806326121ff014602d575b600080fd5b6033605b565b6040518084815260200183151515158152602001828152602001935050505060405180910390f35b600080600060076001600282925080905092509250925090919256fea165627a7a7230582086f654e5018d11f7f693c3672b483dcd494a1000d763a85ab594cc3f41476be00029`
	data, err := hex.DecodeString(hexdata)
	if err != nil {
		t.Fatal(err)
	}

	typ, err := NewType("string", nil)
	a := reflect.New(typ.Type)
	b := a.Elem().Interface()
	err = abi.Unpack(&b, "greet", data)

	fmt.Println(err)
	fmt.Println(b)

}
