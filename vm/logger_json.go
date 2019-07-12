// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package vm

import (
	"encoding/json"
	"io"
	"math/big"
	"time"

	coretypes "github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/vm/common"
	"github.com/BOXFoundation/boxd/vm/common/math"
)

// JSONLogger is an Encoder for logs.
type JSONLogger struct {
	encoder *json.Encoder
	cfg     *LogConfig
}

// NewJSONLogger creates a new EVM tracer that prints execution steps as JSON objects
// into the provided stream.
func NewJSONLogger(cfg *LogConfig, writer io.Writer) *JSONLogger {
	l := &JSONLogger{json.NewEncoder(writer), cfg}
	if l.cfg == nil {
		l.cfg = &LogConfig{}
	}
	return l
}

// CaptureStart start capture.
func (l *JSONLogger) CaptureStart(from coretypes.AddressHash, to coretypes.AddressHash, create bool, input []byte, gas uint64, value *big.Int) error {
	return nil
}

// CaptureState outputs state information on the logger.
func (l *JSONLogger) CaptureState(env *EVM, pc uint64, op OpCode, gas, cost uint64, memory *Memory, stack *Stack, contract *Contract, depth int, err error) error {
	log := StructLog{
		Pc:            pc,
		Op:            op,
		Gas:           gas,
		GasCost:       cost,
		MemorySize:    memory.Len(),
		Storage:       nil,
		Depth:         depth,
		RefundCounter: env.StateDB.GetRefund(),
		Err:           err,
	}
	if !l.cfg.DisableMemory {
		log.Memory = memory.Data()
	}
	if !l.cfg.DisableStack {
		log.Stack = stack.Data()
	}
	return l.encoder.Encode(log)
}

// CaptureFault outputs state information on the logger.
func (l *JSONLogger) CaptureFault(env *EVM, pc uint64, op OpCode, gas, cost uint64, memory *Memory, stack *Stack, contract *Contract, depth int, err error) error {
	return nil
}

// CaptureEnd is triggered at end of execution.
func (l *JSONLogger) CaptureEnd(output []byte, gasUsed uint64, t time.Duration, err error) error {
	type endLog struct {
		Output  string              `json:"output"`
		GasUsed math.HexOrDecimal64 `json:"gasUsed"`
		Time    time.Duration       `json:"time"`
		Err     string              `json:"error,omitempty"`
	}
	if err != nil {
		return l.encoder.Encode(endLog{common.Bytes2Hex(output), math.HexOrDecimal64(gasUsed), t, err.Error()})
	}
	return l.encoder.Encode(endLog{common.Bytes2Hex(output), math.HexOrDecimal64(gasUsed), t, ""})
}
