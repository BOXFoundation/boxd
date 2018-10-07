// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package convert

import (
	proto "github.com/gogo/protobuf/proto"
)

// Convertible interface
type Convertible interface {
	ToProtoMessage() (proto.Message, error)
	FromProtoMessage(proto.Message) error
}
