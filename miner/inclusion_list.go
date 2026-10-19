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

package miner

import (
	"context"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus/misc/eip1559"
	"github.com/ethereum/go-ethereum/core/txpool"
	"github.com/ethereum/go-ethereum/core/txpool/txorder"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
	"github.com/holiman/uint256"
)

// commitInclusionList appends the EIP-7805 inclusion list transactions that are
// not part of the block yet, as far as they can be included. A transaction that
// fails may become includable once another one went in (e.g. its sender gets
// funded, or a nonce gap gets filled), so the list is retried until no more
// progress is made. Blob transactions are exempt from inclusion lists.
func (miner *Miner) commitInclusionList(ctx context.Context, env *environment, inclusionList types.Transactions) {
	if len(inclusionList) == 0 {
		return
	}
	included := make(map[common.Hash]struct{}, len(env.txs)+len(inclusionList))
	for _, tx := range env.txs {
		included[tx.Hash()] = struct{}{}
	}
	pending := make(types.Transactions, 0, len(inclusionList))
	for _, tx := range inclusionList {
		if _, ok := included[tx.Hash()]; ok || tx.Type() == types.BlobTxType {
			continue
		}
		included[tx.Hash()] = struct{}{}
		pending = append(pending, tx)
	}
	for progress := true; progress && len(pending) > 0; {
		progress = false
		remaining := pending[:0]
		for _, tx := range pending {
			if !env.txFitsSize(tx) {
				remaining = append(remaining, tx)
				continue
			}
			env.state.SetTxContext(tx.Hash(), env.tcount, uint32(env.tcount+1))
			if err := miner.commitTransaction(ctx, env, tx); err != nil {
				log.Trace("Skipping inclusion list transaction", "hash", tx.Hash(), "err", err)
				remaining = append(remaining, tx)
				continue
			}
			progress = true
		}
		pending = remaining
	}
}

// BuildInclusionList returns an EIP-7805 inclusion list built from the pending
// transactions of the local pool on top of the current head. Transactions are
// picked in the order the payload builder would include them, skipping blob
// transactions, while they fit into MaxTransactionsBytesPerInclusionList.
func (miner *Miner) BuildInclusionList() [][]byte {
	head := miner.chain.CurrentBlock()
	if head.BaseFee == nil {
		return [][]byte{}
	}
	baseFee := eip1559.CalcBaseFee(miner.chainConfig, head, head.Time)
	filter := txpool.PendingFilter{
		MinTip:  new(uint256.Int),
		BaseFee: uint256.MustFromBig(baseFee),
	}
	pending, _ := miner.txpool.Pending(filter)

	var (
		signer = types.LatestSigner(miner.chainConfig)
		txs    = txorder.NewTransactionsByPriceAndNonce(signer, pending, baseFee)
		list   = [][]byte{}
		size   uint64
	)
	for {
		ltx, _ := txs.Peek()
		if ltx == nil {
			break
		}
		tx := ltx.Resolve()
		if tx == nil {
			txs.Pop()
			continue
		}
		enc, err := tx.MarshalBinary()
		if err != nil || size+uint64(len(enc)) > params.MaxTransactionsBytesPerInclusionList {
			// Later transactions of the same sender depend on this one.
			txs.Pop()
			continue
		}
		list = append(list, enc)
		size += uint64(len(enc))
		txs.Shift()
	}
	return list
}
