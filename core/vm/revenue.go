// Copyright 2023 The go-ethereum Authors
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

package vm

import (
	"fmt"

	"github.com/ethereum/go-ethereum/common"
)

type RevenueEntry struct {
	Recipient common.Address
	Amount    uint64
}

type RevenueTracker struct {
	table map[common.Address]*RevenueEntry
}

func (t *RevenueTracker) SetRecipient(addr common.Address, recipient common.Address) {
	fmt.Println("setting recipient", "addr", addr, "recipient", recipient)
	if e, ok := t.table[addr]; ok {
		e.Recipient = recipient
	} else {
		t.table[addr] = &RevenueEntry{recipient, 0}
	}
}

func (t *RevenueTracker) AddGasUsed(addr common.Address, amount uint64) {
	if e, ok := t.table[addr]; ok {
		e.Amount += amount
	} else {
		t.table[addr] = &RevenueEntry{addr, amount}
	}
}

func (t *RevenueTracker) Entries() (entries []*RevenueEntry) {
	for _, e := range t.table {
		entries = append(entries, e)
	}
	return
}
