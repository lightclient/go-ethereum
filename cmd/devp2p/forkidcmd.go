// Copyright 2019 The go-ethereum Authors
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
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/forkid"
	"github.com/urfave/cli/v2"
)

var (
	forkidCommand = &cli.Command{
		Name:      "forkid",
		Usage:     "Calculates forkid",
		ArgsUsage: "genesis.json",
		Action:    calcForkid,
	}
)

func calcForkid(ctx *cli.Context) error {
	if ctx.NArg() != 1 {
		return errors.New("need key file as argument")
	}
	b, err := os.ReadFile(ctx.Args().Get(0))
	if err != nil {
		return fmt.Errorf("couldn't read genesis: %v", err)
	}
	var gspec core.Genesis
	if err := json.Unmarshal(b, &gspec); err != nil {
		return fmt.Errorf("couldn't unmarshal genesis: %v", err)
	}
	genesis := gspec.ToBlock().Header()
	b, _ = json.MarshalIndent(genesis, "", "  ")
	fmt.Println(string(b))
	id := forkid.NewID(gspec.Config, gspec.ToBlock(), 0, uint64(time.Now().Unix()))
	fmt.Println("forkid", "hash", common.Bytes2Hex(id.Hash[:]), "next", id.Next)
	return nil
}
