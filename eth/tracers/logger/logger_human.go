// Copyright 2021 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package logger

/*
import (
	"io"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/tracing"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
)

type humanLogger struct {
	encoder io.Writer
	cfg     *Config
	env     *tracing.VMContext
	hooks   *tracing.Hooks
}

// NewHumanLogger creates a new EVM tracer that prints execution steps as JSON objects
// into the provided stream.
func NewHumanLogger(cfg *Config, w io.Writer) *tracing.Hooks {
	l := &humanLogger{encoder: w, cfg: cfg}
	if l.cfg == nil {
		l.cfg = &Config{}
	}
	l.hooks = &tracing.Hooks{
		OnTxStart:         l.OnTxStart,
		OnSystemCallStart: l.onSystemCallStart,
		OnExit:            l.OnEnd,
		OnOpcode:          l.OnOpcode,
		OnFault:           l.OnFault,
	}
	return l.hooks
}

func (l *humanLogger) OnFault(pc uint64, op byte, gas uint64, cost uint64, scope tracing.OpContext, depth int, err error) {
	// TODO: Add rData to this interface as well
	l.OnOpcode(pc, op, gas, cost, scope, nil, depth, err)
}

func (l *humanLogger) OnOpcode(pc uint64, op byte, gas, cost uint64, scope tracing.OpContext, rData []byte, depth int, err error) {
	memory := scope.MemoryData()
	stack := scope.StackData()

	log := StructLog{
		Pc:            pc,
		Op:            vm.OpCode(op),
		Gas:           gas,
		GasCost:       cost,
		MemorySize:    len(memory),
		Depth:         depth,
		RefundCounter: l.env.StateDB.GetRefund(),
		Err:           err,
	}
	if l.cfg.EnableMemory {
		log.Memory = memory
	}
	if !l.cfg.DisableStack {
		log.Stack = stack
	}
	if l.cfg.EnableReturnData {
		log.ReturnData = rData
	}
	l.encoder.Write([]byte(log.String()))
}

func (l *humanLogger) onSystemCallStart() {
	// Process no events while in system call.
	hooks := *l.hooks
	*l.hooks = tracing.Hooks{
		OnSystemCallEnd: func() {
			*l.hooks = hooks
		},
	}
}

func (l *humanLogger) OnEnd(depth int, output []byte, gasUsed uint64, err error, reverted bool) {
	if depth > 0 {
		return
	}
	l.OnExit(depth, output, gasUsed, err, false)
}

func (l *humanLogger) OnExit(depth int, output []byte, gasUsed uint64, err error, reverted bool) {
	// type endLog struct {
	// 	Output  string              `json:"output"`
	// 	GasUsed math.HexOrDecimal64 `json:"gasUsed"`
	// 	Err     string              `json:"error,omitempty"`
	// }
	// var errMsg string
	// if err != nil {
	// 	errMsg = err.Error()
	// }
	// l.encoder.Write(endLog{common.Bytes2Hex(output), math.HexOrDecimal64(gasUsed), errMsg})
}

func (l *humanLogger) OnTxStart(env *tracing.VMContext, tx *types.Transaction, from common.Address) {
	l.env = env
}
*/
