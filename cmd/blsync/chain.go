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

package main

import (
	"context"
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum/beacon/engine"
	"github.com/ethereum/go-ethereum/beacon/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/log"
)

// LightClient tracks the head of the chain using the light client protocol,
// which assumes the majority of beacon chain sync committe is honest.
type LightClient struct {
	beacon *BeaconClient
	store  *store

	chainHeadFeed event.Feed
}

// bootstrap retrieves a light client bootstrap and authenticates it against the
// provided trusted root.
func bootstrap(ctx context.Context, server string, root common.Hash) (*LightClient, error) {
	api, err := NewBeaconClient(ctx, server)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to beacon server: %w", err)
	}
	bs, err := api.Bootstrap(root)
	if err != nil {
		return nil, fmt.Errorf("failed to get bootstrap data: %w", err)
	}
	if err := bs.Valid(); err != nil {
		return nil, fmt.Errorf("failed to validate bootstrap data: %w", err)
	}
	current, err := bs.Committee.Deserialize()
	if err != nil {
		return nil, fmt.Errorf("failed to deserialize committee")
	}
	return &LightClient{
		beacon: api,
		store: &store{
			config:     SepoliaChainConfig,
			current:    current,
			optimistic: &bs.Header.Header,
			finalized:  &bs.Header.Header,
		},
	}, nil
}

// ChainHeadEvent returns an authenticated execution payload associated with the
// latest accepted head of the beacon chain.
type ChainHeadEvent struct {
	Data *engine.ExecutableData
}

// SubscribeChainHeadEvent allows callers to subscribe a provided channel to new
// head updates.
func (c *LightClient) SubscribeChainHeadEvent(ch chan<- ChainHeadEvent) event.Subscription {
	return c.chainHeadFeed.Subscribe(ch)
}

// Finalized returns the latest finalized head known to the light client.
func (c *LightClient) Finalized() *types.Header {
	return c.store.finalized
}

// Start executes the main active loop of the light client which drives the
// underlying light client store.
func (c *LightClient) Start() {
	log.Info("beacon light client starting")
	ticker := time.NewTicker(SecondsPerSlot * time.Second)
	for ; ; <-ticker.C {
		if c.store.next == nil {
			log.Debug("fetching committee update", "period", c.store.finalizedPeriod())
			updates, err := c.beacon.GetRangeUpdate(c.store.finalizedPeriod(), 1)
			if err != nil {
				log.Error("failed to fetch next committee", "err", err)
			} else {
				for _, update := range updates {
					if err := c.store.Insert(update); err != nil {
						log.Error("failed to insert committee update", "err", err)
						break
					}
				}
			}
		}

		var (
			update *LightClientUpdate
			err    error
		)
		if c.store.optimistic.Slot%SlotsPerEpoch == 0 {
			log.Debug("fetching finality update")
			update, err = c.beacon.GetFinalityUpdate()
		} else {
			log.Debug("fetching optimistic update")
			update, err = c.beacon.GetOptimisticUpdate()
		}
		if err != nil {
			log.Error("failed to retrieve update", "err", err)
			continue
		}
		if err := c.store.Insert(update); err != nil {
			log.Error("failed to insert update", "err", err)
			continue
		}
		head := update.AttestedHeader
		log.Info("updated head", "slot", head.Slot, "root", head.Hash(), "finalized", c.Finalized().Hash(), "signers", update.SyncAggregate.SignerCount())

		// Fetch full execution payload from beacon provider and send to head feed.
		data, err := c.getExecutableData(head.Hash())
		if err != nil {
			log.Error("failed to insert update", "err", err)
			continue
		}
		c.chainHeadFeed.Send(ChainHeadEvent{Data: data})
	}
}

// getExecutableData retrieves the full beacon block associated with the beacon
// block root and returns the inner execution payload.
func (c *LightClient) getExecutableData(head common.Hash) (*engine.ExecutableData, error) {
	block, err := c.beacon.GetBlock(head)
	if err != nil {
		return nil, fmt.Errorf("failed to get execution payload: %w", err)
	}
	// Compute the root of the block and verify it matches the root the sync
	// committee signed.
	root, err := block.Root()
	if err != nil {
		return nil, fmt.Errorf("failed to compute root for beacon block: %w", err)
	}
	if common.Hash(root) != head {
		return nil, fmt.Errorf("unable to verify block body against sync committee update")
	}
	return versionedBlockToExecutableData(block), nil
}
