package main

import (
	"context"
	"fmt"
	"math/big"
	"os"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/holiman/uint256"
)

func main() {
	r, err := rpc.Dial("http://localhost:8545")
	if err != nil {
		exit("unable to connect to rpc")
	}
	c := ethclient.NewClient(r)

	pk, _ := crypto.HexToECDSA("62e329decd40660a048fa51e3507f9433a7db78b6c343c68fd2f58a1524ba740")
	addr := crypto.PubkeyToAddress(pk.PublicKey)

	ctx := context.Background()
	nonce, err := c.NonceAt(ctx, addr, big.NewInt(int64(rpc.LatestBlockNumber)))
	if err != nil {
		exit(fmt.Sprintf("error retrieving nonce: %v", err))
	}

	balance, err := c.BalanceAt(ctx, addr, big.NewInt(int64(rpc.LatestBlockNumber)))
	if err != nil {
		exit(fmt.Sprintf("error retrieving nonce: %v", err))
	}
	fmt.Println(addr, "nonce", nonce, "balance", balance)

	auth1, _ := types.SignSetCode(pk, types.SetCodeAuthorization{
		ChainID: *uint256.MustFromBig(params.AllDevChainProtocolChanges.ChainID),
		Address: common.Address{0x42},
		Nonce:   nonce + 1,
	})
	txdata := &types.SetCodeTx{
		ChainID:   uint256.MustFromBig(params.AllDevChainProtocolChanges.ChainID),
		Nonce:     nonce,
		To:        common.Address{0x42},
		Gas:       500000,
		GasFeeCap: uint256.NewInt(1000000),
		GasTipCap: uint256.NewInt(42),
		AuthList:  []types.SetCodeAuthorization{auth1},
	}
	tx := types.MustSignNewTx(pk, types.LatestSigner(params.AllDevChainProtocolChanges), txdata)

	if err := c.SendTransaction(ctx, tx); err != nil {
		exit(fmt.Sprintf("error submitting tx: %v", err))
	}

	for {
		if receipt, err := c.TransactionReceipt(ctx, tx.Hash()); err != nil {
			fmt.Printf("error retrieving receipt: %v\n", err)
		} else if receipt != nil {
			break
		}
		time.Sleep(time.Millisecond * 10)
	}

	code, err := c.CodeAt(ctx, addr, big.NewInt(int64(rpc.LatestBlockNumber)))
	if err != nil {
		exit(fmt.Sprintf("error retrieving code: %v", err))
	}

	if code == nil {
		fmt.Println("got code", "nil")
	} else {
		fmt.Println("got code", common.Bytes2Hex(code))
	}
}

func exit(msg string) {
	fmt.Println(msg)
	os.Exit(1)
}
