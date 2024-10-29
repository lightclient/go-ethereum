// Copyright 2024 The go-ethereum Authors
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

package state

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/stateless"
	"github.com/ethereum/go-ethereum/core/tracing"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/trie/utils"
	"github.com/holiman/uint256"
)

// witnessStateDB represents a statedb which emits calls to tracing-hooks
// on state operations.
type witnessStateDB struct {
	inner   *StateDB
	witness *stateless.Witness
}

// NewWitnessState wraps the given stateDB with the witness builder.
func NewWitnessState(stateDb *StateDB, witness *stateless.Witness) *witnessStateDB {
	return &witnessStateDB{stateDb, witness}
}

func (s *witnessStateDB) CreateAccount(addr common.Address) {
	s.inner.CreateAccount(addr)
}

func (s *witnessStateDB) CreateContract(addr common.Address) {
	s.inner.CreateContract(addr)
}

func (s *witnessStateDB) GetBalance(addr common.Address) *uint256.Int {
	return s.inner.GetBalance(addr)
}

func (s *witnessStateDB) GetNonce(addr common.Address) uint64 {
	return s.inner.GetNonce(addr)
}

func (s *witnessStateDB) GetCodeHash(addr common.Address) common.Hash {
	return s.inner.GetCodeHash(addr)
}

func (s *witnessStateDB) GetCode(addr common.Address) []byte {
	return s.inner.GetCode(addr)
}

func (s *witnessStateDB) GetCodeSize(addr common.Address) int {
	return s.inner.GetCodeSize(addr)
}

func (s *witnessStateDB) AddRefund(u uint64) {
	s.inner.AddRefund(u)
}

func (s *witnessStateDB) SubRefund(u uint64) {
	s.inner.SubRefund(u)
}

func (s *witnessStateDB) GetRefund() uint64 {
	return s.inner.GetRefund()
}

func (s *witnessStateDB) GetCommittedState(addr common.Address, hash common.Hash) common.Hash {
	return s.inner.GetCommittedState(addr, hash)
}

func (s *witnessStateDB) GetState(addr common.Address, hash common.Hash) common.Hash {
	return s.inner.GetState(addr, hash)
}

func (s *witnessStateDB) GetStorageRoot(addr common.Address) common.Hash {
	return s.inner.GetStorageRoot(addr)
}

func (s *witnessStateDB) GetTransientState(addr common.Address, key common.Hash) common.Hash {
	return s.inner.GetTransientState(addr, key)
}

func (s *witnessStateDB) SetTransientState(addr common.Address, key, value common.Hash) {
	s.inner.SetTransientState(addr, key, value)
}

func (s *witnessStateDB) HasSelfDestructed(addr common.Address) bool {
	return s.inner.HasSelfDestructed(addr)
}

func (s *witnessStateDB) Exist(addr common.Address) bool {
	return s.inner.Exist(addr)
}

func (s *witnessStateDB) Empty(addr common.Address) bool {
	return s.inner.Empty(addr)
}

func (s *witnessStateDB) AddressInAccessList(addr common.Address) bool {
	return s.inner.AddressInAccessList(addr)
}

func (s *witnessStateDB) SlotInAccessList(addr common.Address, slot common.Hash) (addressOk bool, slotOk bool) {
	return s.inner.SlotInAccessList(addr, slot)
}

func (s *witnessStateDB) AddAddressToAccessList(addr common.Address) {
	s.inner.AddAddressToAccessList(addr)
}

func (s *witnessStateDB) AddSlotToAccessList(addr common.Address, slot common.Hash) {
	s.inner.AddSlotToAccessList(addr, slot)
}

func (s *witnessStateDB) PointCache() *utils.PointCache {
	return s.inner.PointCache()
}

func (s *witnessStateDB) Prepare(rules params.Rules, sender, coinbase common.Address, dest *common.Address, precompiles []common.Address, txAccesses types.AccessList) {
	s.inner.Prepare(rules, sender, coinbase, dest, precompiles, txAccesses)
}

func (s *witnessStateDB) RevertToSnapshot(i int) {
	s.inner.RevertToSnapshot(i)
}

func (s *witnessStateDB) Snapshot() int {
	return s.inner.Snapshot()
}

func (s *witnessStateDB) AddPreimage(hash common.Hash, bytes []byte) {
	s.inner.Snapshot()
}

func (s *witnessStateDB) Witness() *stateless.Witness {
	return s.inner.Witness()
}

func (s *witnessStateDB) SubBalance(addr common.Address, amount *uint256.Int, reason tracing.BalanceChangeReason) uint256.Int {
	prev := s.inner.SubBalance(addr, amount, reason)
	return prev
}

func (s *witnessStateDB) AddBalance(addr common.Address, amount *uint256.Int, reason tracing.BalanceChangeReason) uint256.Int {
	prev := s.inner.AddBalance(addr, amount, reason)
	return prev
}

func (s *witnessStateDB) SetNonce(address common.Address, nonce uint64) {
	s.inner.SetNonce(address, nonce)
}

func (s *witnessStateDB) SetCode(address common.Address, code []byte) {
	s.inner.SetCode(address, code)
}

func (s *witnessStateDB) SetState(address common.Address, key common.Hash, value common.Hash) common.Hash {
	prev := s.inner.SetState(address, key, value)
	return prev
}

func (s *witnessStateDB) SelfDestruct(address common.Address) uint256.Int {
	prev := s.inner.SelfDestruct(address)
	return prev
}

func (s *witnessStateDB) SelfDestruct6780(address common.Address) (uint256.Int, bool) {
	prev, changed := s.inner.SelfDestruct6780(address)
	return prev, changed
}

func (s *witnessStateDB) AddLog(log *types.Log) {
	// The inner will modify the log (add fields), so invoke that first
	s.inner.AddLog(log)
}

func (s *witnessStateDB) Finalise(deleteEmptyObjects bool) {
	s.inner.Finalise(deleteEmptyObjects)
}
