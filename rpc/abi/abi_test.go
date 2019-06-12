// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package abi

import (
	"encoding/hex"
	"fmt"
	"math/big"
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
	c := reflect.ValueOf(&aaa).Elem().Addr().Interface()
	fmt.Println(c)
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

	var aaa = make([]interface{}, 3)
	err = abi.Unpack(&aaa, "f", data)

	fmt.Println(err)
	fmt.Println(aaa)
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
	fmt.Println(res)
}

func TestUnpack(t *testing.T) {

	type s struct {
		B *big.Int
	}

	var aaa interface{}

	aaa = s{}
	fmt.Println(reflect.TypeOf(aaa))

	var marshalledValue interface{}
	marshalledValue = big.NewInt(10)

	retval := reflect.New(reflect.TypeOf(s{})).Elem()
	retval.Field(0).Set(reflect.ValueOf(marshalledValue))
	aaa = retval.Interface()
	// aaa = reflect.ValueOf(retval).Interface()

	fmt.Println()
	fmt.Println(aaa)
}
