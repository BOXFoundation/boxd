// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package convert

import (
	proto "github.com/gogo/protobuf/proto"
)

// Convertible is the interface implemented by an object that can conver to/from proto message representation of itself.
type Convertible interface {
	ToProtoMessage() (proto.Message, error)
	FromProtoMessage(proto.Message) error
}

// Serializable is the interface implemented by an object that can marshal/unmarshal a binary representation of itself.
type Serializable interface {
	Marshal() (data []byte, err error)
	Unmarshal(data []byte) error
}

////////////////////////////////////////////////////////////////////////////////

// MarshalConvertible marshals a Convertible object to binary data.
func MarshalConvertible(c Convertible) (data []byte, err error) {
	msg, err := c.ToProtoMessage()
	if err != nil {
		return nil, err
	}
	return proto.Marshal(msg)
}
