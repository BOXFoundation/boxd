// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package script

import (
	"testing"

	"github.com/facebookgo/ensure"
)

func newScript(opCodes []OpCode) *Script {
	var script Script
	for _, op := range opCodes {
		script = append(script, byte(op))
	}
	return &script
}

func TestScriptEvaluation(t *testing.T) {
	script := newScript([]OpCode{OP8, OP6, OPADD, OP14, OPEQUAL})
	err := script.Evaluate()
	ensure.Nil(t, err)

	script = newScript([]OpCode{OP8, OP6, OPADD, OP11, OPEQUAL})
	err = script.Evaluate()
	ensure.NotNil(t, err)

	script = newScript([]OpCode{OP8, OP6, OPADD, OP11, OPEQUALVERIFY})
	err = script.Evaluate()
	ensure.NotNil(t, err)

	script = newScript([]OpCode{OP8, OP6, OPSUB, OP2, OPEQUAL})
	err = script.Evaluate()
	ensure.Nil(t, err)
}
