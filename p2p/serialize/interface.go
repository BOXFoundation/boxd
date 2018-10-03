// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package serialize

import (
	proto "github.com/gogo/protobuf/proto"
)

// Serializable Define serialize interface
type Serializable interface {
	Serialize() (proto.Message, error)
	Deserialize(proto.Message) error
}
