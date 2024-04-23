// TODO: move back into core/blockchain_test.go when ready to merge.

package core

import (
	"encoding/binary"
	"math/big"
	"os"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus/beacon"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/eth/tracers/logger"
	"github.com/ethereum/go-ethereum/params"
)

// TestEIP7002 verifies that withdrawal requests are processed correctly in the
// pre-deploy and parsed out correctly via the system call.
func TestEIP7002(t *testing.T) {
	var (
		engine = beacon.NewFaker()

		// A sender who makes transactions, has some funds
		key, _ = crypto.HexToECDSA("b71c71a67e1177ad4e901695e1b4b9ee17ae16c6668d313eac2f96dbcda3f291")
		addr   = crypto.PubkeyToAddress(key.PublicKey)
		funds  = new(big.Int).Mul(common.Big1, big.NewInt(params.Ether))
		config = *params.AllEthashProtocolChanges
		gspec  = &Genesis{
			Config: &config,
			Alloc: types.GenesisAlloc{
				addr:                             {Balance: funds},
				params.WithdrawalRequestsAddress: {Code: common.FromHex("3373fffffffffffffffffffffffffffffffffffffffe146090573615156028575f545f5260205ff35b366038141561012e5760115f54600182026001905f5b5f82111560595781019083028483029004916001019190603e565b90939004341061012e57600154600101600155600354806003026004013381556001015f3581556001016020359055600101600355005b6003546002548082038060101160a4575060105b5f5b81811460dd5780604c02838201600302600401805490600101805490600101549160601b83528260140152906034015260010160a6565b910180921460ed579060025560f8565b90505f6002555f6003555b5f548061049d141561010757505f5b60015460028282011161011c5750505f610122565b01600290035b5f555f600155604c025ff35b5f5ffd")},
			},
		}
	)
	gspec.Config.BerlinBlock = common.Big0
	gspec.Config.LondonBlock = common.Big0
	gspec.Config.TerminalTotalDifficulty = common.Big0
	gspec.Config.TerminalTotalDifficultyPassed = true
	gspec.Config.ShanghaiTime = u64(0)
	gspec.Config.CancunTime = u64(0)
	gspec.Config.PragueTime = u64(0)
	signer := types.LatestSigner(gspec.Config)

	// Withdrawal requests to send.
	wxs := types.WithdrawalRequests{
		{
			Source:    addr,
			PublicKey: [48]byte{42},
			Amount:    42,
		},
		{
			Source:    addr,
			PublicKey: [48]byte{13, 37},
			Amount:    1337,
		},
	}

	_, blocks, _ := GenerateChainWithGenesis(gspec, engine, 1, func(i int, b *BlockGen) {
		for i, wx := range wxs {
			data := make([]byte, 56)
			copy(data, wx.PublicKey[:])
			binary.BigEndian.PutUint64(data[48:], wx.Amount)
			txdata := &types.DynamicFeeTx{
				ChainID:    gspec.Config.ChainID,
				Nonce:      uint64(i),
				To:         &params.WithdrawalRequestsAddress,
				Value:      big.NewInt(1),
				Gas:        500000,
				GasFeeCap:  newGwei(5),
				GasTipCap:  big.NewInt(2),
				AccessList: nil,
				Data:       data,
			}
			tx := types.NewTx(txdata)
			tx, _ = types.SignTx(tx, signer, key)
			b.AddTx(tx)
		}
	})
	chain, err := NewBlockChain(rawdb.NewMemoryDatabase(), nil, gspec, nil, engine, vm.Config{Tracer: logger.NewMarkdownLogger(&logger.Config{}, os.Stderr).Hooks()}, nil, nil)
	if err != nil {
		t.Fatalf("failed to create tester chain: %v", err)
	}
	defer chain.Stop()
	if n, err := chain.InsertChain(blocks); err != nil {
		t.Fatalf("block %d: failed to insert into chain: %v", n, err)
	}
	block := chain.GetBlockByNumber(1)
	if block == nil {
		t.Fatalf("failed to retrieve block 1")
	}

	// Verify the withdrawal requests match.
	got := block.Requests()
	if len(got) != 2 {
		t.Fatalf("wrong number of withdrawal requests: wanted 2, got %d", len(wxs))
	}
	for i, want := range wxs {
		got, ok := got[i].Inner().(*types.WithdrawalRequest)
		if !ok {
			t.Fatalf("expected withdrawal request")
		}
		if want.Source != got.Source {
			t.Fatalf("wrong source address: want %s, got %s", want.Source, got.Source)
		}
		if want.PublicKey != got.PublicKey {
			t.Fatalf("wrong public key: want %s, got %s", common.Bytes2Hex(want.PublicKey[:]), common.Bytes2Hex(got.PublicKey[:]))
		}
		if want.Amount != got.Amount {
			t.Fatalf("wrong amount: want %d, got %d", want.Amount, got.Amount)
		}
	}
}
