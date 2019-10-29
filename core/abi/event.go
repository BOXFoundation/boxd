// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package abi

import (
	"fmt"
	"strings"

	corecrypto "github.com/BOXFoundation/boxd/crypto"
	"github.com/BOXFoundation/boxd/vm/crypto"
)

// Event is an event potentially triggered by the EVM's LOG mechanism. The Event
// holds type information (inputs) about the yielded output. Anonymous events
// don't get the signature canonical representation as the first LOG topic.
type Event struct {
	// Name is the event name used for internal representation. It's derived from
	// the raw name and a suffix will be added in the case of a event overload.
	//
	// e.g.
	// There are two events have same name:
	// * foo(int,int)
	// * foo(uint,uint)
	// The event name of the first one wll be resolved as foo while the second one
	// will be resolved as foo0.
	Name string
	// RawName is the raw event name parsed from ABI.
	RawName   string
	Anonymous bool
	Inputs    Arguments
}

func (e Event) String() string {
	inputs := make([]string, len(e.Inputs))
	for i, input := range e.Inputs {
		inputs[i] = fmt.Sprintf("%v %v", input.Type, input.Name)
		if input.Indexed {
			inputs[i] = fmt.Sprintf("%v indexed %v", input.Type, input.Name)
		}
	}
	return fmt.Sprintf("event %v(%v)", e.RawName, strings.Join(inputs, ", "))
}

// Sig returns the event string signature according to the ABI spec.
//
// Example
//
//     event foo(uint32 a, int b) = "foo(uint32,int256)"
//
// Please note that "int" is substitute for its canonical representation "int256"
func (e Event) Sig() string {
	types := make([]string, len(e.Inputs))
	for i, input := range e.Inputs {
		types[i] = input.Type.String()
	}
	return fmt.Sprintf("%v(%v)", e.RawName, strings.Join(types, ","))
}

// ID returns the canonical representation of the event's signature used by the
// abi definition to identify event names and types.
func (e Event) ID() corecrypto.HashType {
	return crypto.Keccak256Hash([]byte(e.Sig()))
}
