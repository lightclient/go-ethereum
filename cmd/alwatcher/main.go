package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/eth/tracers"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Println("usage: alwatcher <rpc endpoint> <output>")
		os.Exit(1)
	}
	f, err := os.Open(os.Args[2])
	if err != nil {
		fmt.Printf("failed to open file %s: %v\n", os.Args[2], err)
	}

	r, err := rpc.DialHTTP(os.Args[1])
	if err != nil {
		fmt.Printf("failed to dial client: %v\n", err)
		os.Exit(1)
	}
	c := ethclient.NewClient(r)

	var (
		newHeads = make(chan *types.Header)
		ctx      = context.Background()
	)
	sub, err := c.SubscribeNewHead(ctx, newHeads)
	if err != nil {
		fmt.Printf("failed to subscribe: %v\n", err)
	}
	defer sub.Unsubscribe()

	for {
		select {
		case head := <-newHeads:
			var a tracers.AccessListAnalysis
			err := r.Call(&a, "debug_analyzeAccessListUseBlock", head.Number.Uint64())
			if err != nil {
				fmt.Fprintf(os.Stderr, "failed to call analysis rpc: %v\n", err)
			}
			raw, err := json.MarshalIndent(a, "", "  ")
			f.Write(raw)
		}
	}
}
