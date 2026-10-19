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

package catalyst

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/beacon/engine"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/params"
)

// newBogotaEnv starts a node on a chain with Amsterdam and Bogota active from
// genesis.
func newBogotaEnv(t *testing.T) (*node.Node, *ConsensusAPI) {
	t.Helper()
	config := *params.MergedTestChainConfig
	config.AmsterdamTime = new(uint64)
	config.BogotaTime = new(uint64)
	config.BlobScheduleConfig = &params.BlobScheduleConfig{
		Cancun: params.DefaultCancunBlobConfig,
		Prague: params.DefaultPragueBlobConfig,
		Bogota: params.DefaultBogotaBlobConfig,
	}
	gspec := &core.Genesis{
		Config: &config,
		Alloc: types.GenesisAlloc{
			testAddr:                         {Balance: testBalance},
			params.BeaconRootsAddress:        {Nonce: 1, Code: params.BeaconRootsCode, Balance: common.Big0},
			params.HistoryStorageAddress:     {Nonce: 1, Code: params.HistoryStorageCode, Balance: common.Big0},
			params.WithdrawalQueueAddress:    {Nonce: 1, Code: params.WithdrawalQueueCode, Balance: common.Big0},
			params.ConsolidationQueueAddress: {Nonce: 1, Code: params.ConsolidationQueueCode, Balance: common.Big0},
			params.BuilderDepositAddress:     {Nonce: 1, Code: params.BuilderDepositCode, Balance: common.Big0},
			params.BuilderExitAddress:        {Nonce: 1, Code: params.BuilderExitCode, Balance: common.Big0},
		},
		Difficulty: common.Big0,
		BaseFee:    big.NewInt(params.InitialBaseFee),
	}
	n, ethservice := startEthService(t, gspec, nil)
	return n, newConsensusAPIWithoutHeartbeat(ethservice)
}

// bogotaTransfer signs a value transfer from the test account.
func bogotaTransfer(t *testing.T, api *ConsensusAPI, nonce uint64, data []byte) (*types.Transaction, hexutil.Bytes) {
	t.Helper()
	config := api.config()
	tx := types.MustSignNewTx(testKey, types.LatestSigner(config), &types.DynamicFeeTx{
		ChainID:   config.ChainID,
		Nonce:     nonce,
		To:        &common.Address{0xaa},
		Value:     big.NewInt(1),
		Gas:       100_000,
		GasFeeCap: big.NewInt(10 * params.InitialBaseFee),
		Data:      data,
	})
	enc, err := tx.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	return tx, enc
}

func TestInclusionListEngineAPI(t *testing.T) {
	n, api := newBogotaEnv(t)
	defer n.Close()

	var (
		ctx     = context.Background()
		genesis = api.eth.BlockChain().CurrentBlock()
		slot    = uint64(1)

		// tx0 is only known through the inclusion list, tx1 is appendable
		// to any block including tx0.
		tx0, enc0 = bogotaTransfer(t, api, 0, nil)
		_, enc1   = bogotaTransfer(t, api, 1, nil)
	)
	attrs := &engine.PayloadAttributes{
		Timestamp:                 genesis.Time + 12,
		Random:                    common.Hash{0x01},
		SuggestedFeeRecipient:     common.Address{0x02},
		Withdrawals:               []*types.Withdrawal{},
		BeaconRoot:                &common.Hash{0x03},
		SlotNumber:                &slot,
		InclusionListTransactions: [][]byte{enc0},
	}
	fcState := engine.ForkchoiceStateV1{HeadBlockHash: genesis.Hash()}

	// The previous fork methods reject Bogota payloads.
	if _, err := api.ForkchoiceUpdatedV4(ctx, fcState, attrs, nil); !isEngineError(err, engine.UnsupportedFork) {
		t.Fatalf("fcuV4 with bogota attributes: want unsupported fork, got %v", err)
	}
	// The payload built for an inclusion list contains its transactions, even
	// if the local pool does not know them.
	resp, err := api.ForkchoiceUpdatedV5(ctx, fcState, attrs, nil)
	if err != nil {
		t.Fatalf("fcuV5: %v", err)
	}
	if resp.PayloadStatus.Status != engine.VALID || resp.PayloadID == nil {
		t.Fatalf("fcuV5: unexpected response %+v", resp)
	}
	envelope, err := api.GetPayloadV6(*resp.PayloadID)
	if err != nil {
		t.Fatalf("getPayloadV6: %v", err)
	}
	payload := envelope.ExecutionPayload
	if len(payload.Transactions) != 1 || !bytes.Equal(payload.Transactions[0], enc0) {
		t.Fatalf("built payload does not contain the inclusion list transaction %x", tx0.Hash())
	}
	requests := make([]hexutil.Bytes, len(envelope.Requests))
	for i, req := range envelope.Requests {
		requests[i] = req
	}
	newPayload := func(il []hexutil.Bytes) engine.PayloadStatusV2 {
		t.Helper()
		status, err := api.NewPayloadV6(ctx, *payload, []common.Hash{}, attrs.BeaconRoot, requests, il)
		if err != nil {
			t.Fatalf("newPayloadV6: %v", err)
		}
		if status.Status != engine.VALID {
			t.Fatalf("newPayloadV6: unexpected status %s", status.Status)
		}
		return status
	}
	if _, err := api.NewPayloadV5(ctx, *payload, []common.Hash{}, attrs.BeaconRoot, requests); !isEngineError(err, engine.UnsupportedFork) {
		t.Fatalf("newPayloadV5 with bogota payload: want unsupported fork, got %v", err)
	}
	if _, err := api.NewPayloadV6(ctx, *payload, []common.Hash{}, attrs.BeaconRoot, requests, nil); !isEngineError(err, engine.InvalidParams) {
		t.Fatalf("newPayloadV6 without inclusion list: want invalid params, got %v", err)
	}
	// Import the payload, then deliver it again with a list it fails to
	// satisfy, which re-evaluates the already known block.
	if status := newPayload([]hexutil.Bytes{enc0}); status.InclusionListSatisfied == nil || !*status.InclusionListSatisfied {
		t.Fatalf("payload including the list: want satisfied, got %v", status.InclusionListSatisfied)
	}
	if status := newPayload([]hexutil.Bytes{enc0, enc1}); status.InclusionListSatisfied == nil || *status.InclusionListSatisfied {
		t.Fatalf("payload missing an appendable transaction: want unsatisfied, got %v", status.InclusionListSatisfied)
	}
	// The forkchoice update reports the verdict on the list the head was last
	// delivered with.
	resp, err = api.ForkchoiceUpdatedV5(ctx, engine.ForkchoiceStateV1{HeadBlockHash: payload.BlockHash}, nil, nil)
	if err != nil {
		t.Fatalf("fcuV5: %v", err)
	}
	if resp.PayloadStatus.Status != engine.VALID || resp.PayloadStatus.InclusionListSatisfied == nil || *resp.PayloadStatus.InclusionListSatisfied {
		t.Fatalf("fcuV5 to unsatisfying head: unexpected status %+v", resp.PayloadStatus)
	}
	// Only the Bogota methods expose the verdict, and always carry the field.
	v4, err := api.ForkchoiceUpdatedV4(ctx, engine.ForkchoiceStateV1{HeadBlockHash: payload.BlockHash}, nil, nil)
	if err != nil {
		t.Fatalf("fcuV4: %v", err)
	}
	if enc, _ := json.Marshal(v4); bytes.Contains(enc, []byte("inclusionListSatisfied")) {
		t.Fatalf("fcuV4 response exposes the inclusion list verdict: %s", enc)
	}
	if enc, _ := json.Marshal(engine.STATUS_SYNCING.V2()); !bytes.Contains(enc, []byte(`"inclusionListSatisfied":null`)) {
		t.Fatalf("PayloadStatusV2 without verdict lacks an explicit null: %s", enc)
	}
}

// TestInclusionListPayloadBuilding checks that the payload builder includes
// inclusion list transactions that only become valid once another one of the
// list went in, here a sender's nonces listed in reverse order.
func TestInclusionListPayloadBuilding(t *testing.T) {
	n, api := newBogotaEnv(t)
	defer n.Close()

	var (
		ctx       = context.Background()
		genesis   = api.eth.BlockChain().CurrentBlock()
		slot      = uint64(1)
		tx0, enc0 = bogotaTransfer(t, api, 0, nil)
		tx1, enc1 = bogotaTransfer(t, api, 1, nil)
	)
	attrs := &engine.PayloadAttributes{
		Timestamp:                 genesis.Time + 12,
		Withdrawals:               []*types.Withdrawal{},
		BeaconRoot:                &common.Hash{},
		SlotNumber:                &slot,
		InclusionListTransactions: [][]byte{enc1, enc0},
	}
	resp, err := api.ForkchoiceUpdatedV5(ctx, engine.ForkchoiceStateV1{HeadBlockHash: genesis.Hash()}, attrs, nil)
	if err != nil {
		t.Fatalf("fcuV5: %v", err)
	}
	envelope, err := api.GetPayloadV6(*resp.PayloadID)
	if err != nil {
		t.Fatalf("getPayloadV6: %v", err)
	}
	payload := envelope.ExecutionPayload
	if len(payload.Transactions) != 2 || !bytes.Equal(payload.Transactions[0], enc0) || !bytes.Equal(payload.Transactions[1], enc1) {
		t.Fatalf("built payload does not contain %x and %x in nonce order", tx0.Hash(), tx1.Hash())
	}
	requests := make([]hexutil.Bytes, len(envelope.Requests))
	for i, req := range envelope.Requests {
		requests[i] = req
	}
	status, err := api.NewPayloadV6(ctx, *payload, []common.Hash{}, attrs.BeaconRoot, requests, []hexutil.Bytes{enc1, enc0})
	if err != nil {
		t.Fatalf("newPayloadV6: %v", err)
	}
	if status.Status != engine.VALID || status.InclusionListSatisfied == nil || !*status.InclusionListSatisfied {
		t.Fatalf("built payload: want valid and satisfied, got %s %v", status.Status, status.InclusionListSatisfied)
	}
}

func TestGetInclusionListV1(t *testing.T) {
	n, api := newBogotaEnv(t)
	defer n.Close()

	// Fill the pool with more than an inclusion list can hold.
	var txs []*types.Transaction
	for nonce := range uint64(100) {
		tx, _ := bogotaTransfer(t, api, nonce, make([]byte, 100))
		txs = append(txs, tx)
	}
	for _, err := range api.eth.TxPool().Add(txs, true) {
		if err != nil {
			t.Fatalf("add tx: %v", err)
		}
	}
	list := api.GetInclusionListV1()
	if len(list) == 0 || len(list) == len(txs) {
		t.Fatalf("unexpected inclusion list length %d", len(list))
	}
	var size int
	for i, enc := range list {
		size += len(enc)
		if !bytes.Equal(enc, mustEncode(t, txs[i])) {
			t.Fatalf("entry %d is not the pool transaction with nonce %d", i, i)
		}
	}
	if size > int(params.MaxTransactionsBytesPerInclusionList) {
		t.Fatalf("inclusion list is %d bytes, limit %d", size, params.MaxTransactionsBytesPerInclusionList)
	}
	if next := len(mustEncode(t, txs[len(list)])); size+next <= int(params.MaxTransactionsBytesPerInclusionList) {
		t.Fatalf("inclusion list stops at %d bytes although the next transaction (%d bytes) fits", size, next)
	}
}

func TestInclusionListQueue(t *testing.T) {
	q := newInclusionListQueue()
	hash := func(i int) common.Hash { return common.Hash{byte(i), byte(i >> 8)} }
	yes, no := true, false

	// Redelivering a payload replaces its entry instead of taking a new slot.
	for i := range maxTrackedInclusionLists {
		q.put(hash(i), nil, nil)
	}
	for range 2 * maxTrackedInclusionLists {
		q.put(hash(0), nil, &yes)
	}
	for i := range maxTrackedInclusionLists {
		if q.get(hash(i)) == nil {
			t.Fatalf("entry %d evicted by redeliveries of another payload", i)
		}
	}
	if item := q.get(hash(0)); item.satisfied == nil || !*item.satisfied {
		t.Fatal("redelivery did not replace the entry")
	}
	// A new payload evicts the oldest one.
	q.put(hash(maxTrackedInclusionLists), nil, nil)
	if q.get(hash(1)) != nil || q.get(hash(0)) == nil {
		t.Fatal("wrong entry evicted")
	}
	// A verdict is only recorded on the entry it was computed for.
	stale := q.get(hash(2))
	q.put(hash(2), types.Transactions{}, &no)
	q.resolve(stale, true)
	if item := q.get(hash(2)); item.satisfied == nil || *item.satisfied {
		t.Fatal("verdict on a replaced entry overwrote the current one")
	}
	current := q.get(hash(3))
	q.resolve(current, true)
	if item := q.get(hash(3)); item.satisfied == nil || !*item.satisfied {
		t.Fatal("verdict on the current entry not recorded")
	}
}

func isEngineError(err error, want *engine.EngineAPIError) bool {
	var apiErr *engine.EngineAPIError
	return errors.As(err, &apiErr) && apiErr.ErrorCode() == want.ErrorCode()
}

func mustEncode(t *testing.T, tx *types.Transaction) []byte {
	t.Helper()
	enc, err := tx.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	return enc
}
