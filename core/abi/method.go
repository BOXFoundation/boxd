// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package abi

import (
	"fmt"
	"strings"

	"github.com/BOXFoundation/boxd/vm/crypto"
)

// Method represents a callable given a `Name` and whether the method is a constant.
// If the method is `Const` no transaction needs to be created for this
// particular Method call. It can easily be simulated using a local VM.
// For example a `Balance()` method only needs to retrieve something
// from the storage and therefore requires no Tx to be send to the
// network. A method such as `Transact` does require a Tx and thus will
// be flagged `false`.
// Input specifies the required input parameters for this gives method.
type Method struct {
	// Name is the method name used for internal representation. It's derived from
	// the raw name and a suffix will be added in the case of a function overload.
	//
	// e.g.
	// There are two functions have same name:
	// * foo(int,int)
	// * foo(uint,uint)
	// The method name of the first one will be resolved as foo while the second one
	// will be resolved as foo0.
	Name string
	// RawName is the raw method name parsed from ABI.
	RawName string
	Const   bool
	Inputs  Arguments
	Outputs Arguments
}

// Sig returns the methods string signature according to the ABI spec.
//
// Example
//
//     function foo(uint32 a, int b) = "foo(uint32,int256)"
//
// Please note that "int" is substitute for its canonical representation "int256"
func (method Method) Sig() string {
	types := make([]string, len(method.Inputs))
	for i, input := range method.Inputs {
		types[i] = input.Type.String()
	}
	return fmt.Sprintf("%v(%v)", method.RawName, strings.Join(types, ","))
}

func (method Method) String() string {
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
		constant = "constant "
	}
	return fmt.Sprintf("function %v(%v) %sreturns(%v)", method.RawName, strings.Join(inputs, ", "), constant, strings.Join(outputs, ", "))
}

// ID returns the canonical representation of the method's signature used by the
// abi definition to identify method names and types.
func (method Method) ID() []byte {
	return crypto.Keccak256([]byte(method.Sig()))[:4]
}
