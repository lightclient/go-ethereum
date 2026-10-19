// Copyright 2026 The go-ethereum Authors
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

package core

import (
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
	"github.com/holiman/uint256"
)

// inclusionListSatisfied reports whether a block satisfies the EIP-7805
// inclusion list constraints: every inclusion list transaction that is not part
// of the block must be impossible to append at the end of it. Blob transactions
// are exempt.
//
// The statedb must reflect the state after the last transaction of the block,
// before withdrawals and the post-execution system calls, and gp must be the
// block gas pool at that point. Neither is modified.
func inclusionListSatisfied(config *params.ChainConfig, header *types.Header, txs, inclusionList types.Transactions, statedb vm.StateDB, gp *GasPool) bool {
	if len(inclusionList) == 0 || !config.IsBogota(header.Number, header.Time) {
		return true
	}
	included := make(map[common.Hash]struct{}, len(txs))
	for _, tx := range txs {
		included[tx.Hash()] = struct{}{}
	}
	var (
		signer = types.MakeSigner(config, header.Number, header.Time)
		rules  = config.Rules(header.Number, true, header.Time)
	)
	for _, tx := range inclusionList {
		if _, ok := included[tx.Hash()]; ok {
			continue
		}
		if tx.Type() == types.BlobTxType {
			continue
		}
		if err := checkAppendable(tx, signer, rules, header, statedb, gp); err != nil {
			log.Trace("Inclusion list transaction not appendable", "number", header.Number, "tx", tx.Hash(), "err", err)
			continue
		}
		log.Debug("Block misses appendable inclusion list transaction", "number", header.Number, "hash", header.Hash(), "tx", tx.Hash())
		return false
	}
	return true
}

// checkAppendable runs the validity checks a transaction is subject to when it
// is included in a block, without executing it. It mirrors the checks of
// stateTransition.preCheck and the intrinsic gas checks of execute, under the
// Amsterdam rules every inclusion list fork builds on.
func checkAppendable(tx *types.Transaction, signer types.Signer, rules params.Rules, header *types.Header, statedb vm.StateDB, gp *GasPool) error {
	// Recover the sender, which also rejects transactions of other chains.
	msg, err := TransactionToMessage(tx, signer, header.BaseFee)
	if err != nil {
		return err
	}
	// Static checks of the transaction itself.
	if msg.GasLimit > params.MaxTxTotalGas {
		return fmt.Errorf("%w (cap: %d, tx: %d)", ErrGasLimitTooHigh, params.MaxTxTotalGas, msg.GasLimit)
	}
	if msg.GasFeeCap.Cmp(msg.GasTipCap) < 0 {
		return ErrTipAboveFeeCap
	}
	if tx.Type() == types.SetCodeTxType && len(msg.SetCodeAuthorizations) == 0 {
		return ErrEmptyAuthList
	}
	if msg.To == nil {
		if err := vm.CheckMaxInitCodeSize(&rules, uint64(len(msg.Data))); err != nil {
			return err
		}
	}
	intrinsicGas, err := IntrinsicGas(msg.Data, msg.AccessList, msg.SetCodeAuthorizations, msg.From, msg.To, msg.Value, rules)
	if err != nil {
		return err
	}
	if msg.GasLimit < intrinsicGas {
		return ErrIntrinsicGas
	}
	floorDataGas, err := FloorDataGas(rules, msg.From, msg.To, msg.Value, msg.Data, msg.AccessList)
	if err != nil {
		return err
	}
	if msg.GasLimit < floorDataGas {
		return ErrFloorDataGas
	}
	if max(intrinsicGas, floorDataGas) > params.MaxTxGas {
		return ErrFloorDataGas
	}
	// Checks against the block.
	if err := gp.CheckGasAmsterdam(min(msg.GasLimit, params.MaxTxGas), msg.GasLimit); err != nil {
		return err
	}
	if msg.GasFeeCap.CmpBig(header.BaseFee) < 0 {
		return ErrFeeCapTooLow
	}
	// Checks against the sender account.
	nonce := statedb.GetNonce(msg.From)
	switch {
	case nonce < msg.Nonce:
		return ErrNonceTooHigh
	case nonce > msg.Nonce:
		return ErrNonceTooLow
	case nonce+1 < nonce:
		return ErrNonceMax
	}
	cost := new(uint256.Int).SetUint64(msg.GasLimit)
	if _, overflow := cost.MulOverflow(cost, msg.GasFeeCap); overflow {
		return ErrInsufficientFunds
	}
	if _, overflow := cost.AddOverflow(cost, msg.Value); overflow {
		return ErrInsufficientFunds
	}
	if statedb.GetBalance(msg.From).Cmp(cost) < 0 {
		return ErrInsufficientFunds
	}
	if code := statedb.GetCode(msg.From); len(code) > 0 {
		if _, delegated := types.ParseDelegation(code); !delegated {
			return ErrSenderNoEOA
		}
	}
	return nil
}
