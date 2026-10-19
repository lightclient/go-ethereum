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
	"context"
	"crypto/ecdsa"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus/beacon"
	"github.com/ethereum/go-ethereum/consensus/ethash"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/params"
	"github.com/holiman/uint256"
)

// ilTestEnv is a Bogota chain with a single block, whose only transaction is
// sent by env.from with nonce 0.
type ilTestEnv struct {
	*balTestEnv
	block *types.Block

	// funded is a funded account that is not touched by the block.
	funded *ecdsa.PrivateKey
	// poor has no balance, contract has code and delegated has a delegation.
	poor, contract, delegated *ecdsa.PrivateKey
}

func newILTestEnv(t *testing.T) *ilTestEnv {
	t.Helper()
	var (
		funded, _    = crypto.HexToECDSA("8a1f9a8f95be41cd7ccb6168179afb4504aefe388d1e14474d32c45c72ce7b7a")
		poor, _      = crypto.HexToECDSA("49a7b37aa6f6645917e7b807e9d1c00d4fa71f18343b0d4122a4d2df64dd6fee")
		contract, _  = crypto.HexToECDSA("45a915e4d060149eb4365960e6a7a45f334393093061116b197e3240065ff2d8")
		delegated, _ = crypto.HexToECDSA("0202020202020202020202020202020202020202020202020202020202020202")
		balance      = newGwei(1_000_000_000)
	)
	env := newBALTestEnv(types.GenesisAlloc{
		crypto.PubkeyToAddress(funded.PublicKey):    {Balance: balance},
		crypto.PubkeyToAddress(contract.PublicKey):  {Balance: balance, Code: []byte{0x00}},
		crypto.PubkeyToAddress(delegated.PublicKey): {Balance: balance, Code: types.AddressToDelegation(common.Address{0xaa})},
	})
	env.cfg.BogotaTime = new(uint64)
	env.cfg.BlobScheduleConfig = &params.BlobScheduleConfig{
		Cancun: params.DefaultCancunBlobConfig,
		Prague: params.DefaultPragueBlobConfig,
		Bogota: params.DefaultBogotaBlobConfig,
	}
	_, blocks, _ := GenerateChainWithGenesis(env.gspec, beacon.New(ethash.NewFaker()), 1, func(_ int, b *BlockGen) {
		b.AddTx(types.MustSignNewTx(env.key, env.signer, &types.DynamicFeeTx{
			ChainID:   env.cfg.ChainID,
			Nonce:     0,
			To:        &common.Address{0xbb},
			Value:     big.NewInt(1),
			Gas:       100_000,
			GasFeeCap: new(big.Int).Mul(b.BaseFee(), big.NewInt(2)),
		}))
	})
	if blocks[0].AccessList() == nil {
		t.Fatal("expected a block access list")
	}
	return &ilTestEnv{balTestEnv: env, block: blocks[0], funded: funded, poor: poor, contract: contract, delegated: delegated}
}

// transfer signs a value transfer that pays twice the block base fee, unless
// overridden by edit.
func (e *ilTestEnv) transfer(key *ecdsa.PrivateKey, nonce uint64, edit func(*types.DynamicFeeTx)) *types.Transaction {
	inner := &types.DynamicFeeTx{
		ChainID:   e.cfg.ChainID,
		Nonce:     nonce,
		To:        &common.Address{0xcc},
		Value:     big.NewInt(1),
		Gas:       100_000,
		GasFeeCap: new(big.Int).Mul(e.block.BaseFee(), big.NewInt(2)),
	}
	if edit != nil {
		edit(inner)
	}
	signer := types.LatestSignerForChainID(inner.ChainID)
	return types.MustSignNewTx(key, signer, inner)
}

func TestInclusionListSatisfied(t *testing.T) {
	env := newILTestEnv(t)
	var (
		appendable   = env.transfer(env.key, 1, nil)
		nonceLow     = env.transfer(env.key, 0, nil)
		nonceHigh    = env.transfer(env.key, 2, nil)
		poor         = env.transfer(env.poor, 0, nil)
		contract     = env.transfer(env.contract, 0, nil)
		delegated    = env.transfer(env.delegated, 0, nil)
		belowBase    = env.transfer(env.funded, 0, func(tx *types.DynamicFeeTx) { tx.GasFeeCap = new(big.Int).Sub(env.block.BaseFee(), common.Big1) })
		atBase       = env.transfer(env.funded, 0, func(tx *types.DynamicFeeTx) { tx.GasFeeCap = env.block.BaseFee() })
		otherChain   = env.transfer(env.funded, 0, func(tx *types.DynamicFeeTx) { tx.ChainID = big.NewInt(1337) })
		lowGas       = env.transfer(env.funded, 0, func(tx *types.DynamicFeeTx) { tx.Gas = 1000 })
		overLimit    = env.transfer(env.funded, 0, func(tx *types.DynamicFeeTx) { tx.Gas = env.block.GasLimit() + 1 })
		unaffordable = env.transfer(env.funded, 0, func(tx *types.DynamicFeeTx) { tx.Value = newGwei(1_000_000_000) })
		blob         = types.MustSignNewTx(env.funded, env.signer, &types.BlobTx{
			ChainID:    uint256.MustFromBig(env.cfg.ChainID),
			Gas:        100_000,
			GasFeeCap:  uint256.MustFromBig(new(big.Int).Mul(env.block.BaseFee(), big.NewInt(2))),
			BlobFeeCap: uint256.NewInt(1 << 30),
			BlobHashes: []common.Hash{{0x01}},
		})
	)
	tests := []struct {
		name string
		list types.Transactions
		want bool
	}{
		{"empty", types.Transactions{}, true},
		{"included", types.Transactions{env.block.Transactions()[0]}, true},
		{"appendable", types.Transactions{appendable}, false},
		{"nonce too low", types.Transactions{nonceLow}, true},
		{"nonce too high", types.Transactions{nonceHigh}, true},
		{"no balance", types.Transactions{poor}, true},
		{"value unaffordable", types.Transactions{unaffordable}, true},
		{"sender with code", types.Transactions{contract}, true},
		{"sender with delegation", types.Transactions{delegated}, false},
		{"fee cap below base fee", types.Transactions{belowBase}, true},
		{"fee cap at base fee", types.Transactions{atBase}, false},
		{"other chain", types.Transactions{otherChain}, true},
		{"intrinsic gas too low", types.Transactions{lowGas}, true},
		{"over block gas limit", types.Transactions{overLimit}, true},
		{"blob", types.Transactions{blob}, true},
		{"unappendable before appendable", types.Transactions{belowBase, nonceLow, appendable}, false},
	}
	bc, err := NewBlockChain(rawdb.NewMemoryDatabase(), env.gspec, beacon.New(ethash.NewFaker()), nil)
	if err != nil {
		t.Fatalf("new blockchain: %v", err)
	}
	defer bc.Stop()

	for _, tt := range tests {
		for _, sequential := range []bool{false, true} {
			statedb, err := bc.State()
			if err != nil {
				t.Fatalf("state: %v", err)
			}
			cfg := vm.Config{DisableParallelExecution: sequential}
			res, err := NewStateProcessor(bc).Process(context.Background(), env.block.WithInclusionList(tt.list), statedb, nil, nil, cfg, nil)
			if err != nil {
				t.Fatalf("%s (sequential %t): process: %v", tt.name, sequential, err)
			}
			if res.InclusionListSatisfied != tt.want {
				t.Errorf("%s (sequential %t): satisfied %t, want %t", tt.name, sequential, res.InclusionListSatisfied, tt.want)
			}
		}
	}
}

// TestInclusionListImport checks that the verdict computed during block import
// is handed back, and that an imported block can be checked against another
// inclusion list afterwards.
func TestInclusionListImport(t *testing.T) {
	env := newILTestEnv(t)
	bc, err := NewBlockChain(rawdb.NewMemoryDatabase(), env.gspec, beacon.New(ethash.NewFaker()), nil)
	if err != nil {
		t.Fatalf("new blockchain: %v", err)
	}
	defer bc.Stop()

	appendable := env.transfer(env.key, 1, nil)
	res, err := bc.InsertBlockWithoutSetHead(context.Background(), env.block.WithInclusionList(types.Transactions{appendable}), false)
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	if res.InclusionListSatisfied() {
		t.Error("import reported a satisfied inclusion list for a block missing an appendable transaction")
	}
	for _, tt := range []struct {
		list types.Transactions
		want bool
	}{
		{nil, true},
		{types.Transactions{env.block.Transactions()[0]}, true},
		{types.Transactions{appendable}, false},
	} {
		satisfied, err := bc.InclusionListSatisfied(context.Background(), env.block, tt.list)
		if err != nil {
			t.Fatalf("check known block: %v", err)
		}
		if satisfied != tt.want {
			t.Errorf("known block with %d list entries: satisfied %t, want %t", len(tt.list), satisfied, tt.want)
		}
	}
}
