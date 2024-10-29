package main

import (
	"context"
	"fmt"
	"math/big"
	"os"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/holiman/uint256"
)

var (
	chainID = big.NewInt(7042905162)
	key, _  = crypto.HexToECDSA("b71c71a67e1177ad4e901695e1b4b9ee17ae16c6668d313eac2f96dbcda3f291")
	addr    = crypto.PubkeyToAddress(key.PublicKey)
	s       = types.NewPragueSigner(chainID)
)

func main() {
	if err := run(); err != nil {
		fmt.Printf("error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	ctx := context.Background()
	client, err := ethclient.Dial("https://rpc.pectra-devnet-4.ethpandaops.io")
	if err != nil {
		return fmt.Errorf("failed to connect to rpc: %w", err)
	}
	nonce, err := client.NonceAt(ctx, addr, nil)
	if err != nil {
		return fmt.Errorf("failed to retrieve nonce: %w", err)
	}
	auth, _ := types.SignAuth(&types.Authorization{
		ChainID: chainID.Uint64(),
		Address: common.Address{0x42},
		Nonce:   nonce + 1,
	}, key)
	txdata := &types.SetCodeTx{
		ChainID:   chainID.Uint64(),
		Nonce:     nonce,
		GasTipCap: uint256.NewInt(1),
		GasFeeCap: uint256.NewInt(1000),
		Gas:       100000,
		To:        addr,
		AuthList:  []*types.Authorization{auth},
	}

	fmt.Println("sending auth tx..")
	tx := types.MustSignNewTx(key, s, txdata)
	if err := client.SendTransaction(ctx, tx); err != nil {
		return fmt.Errorf("failed to send tx: %w", err)
	}
	fmt.Println("success. txhash:", tx.Hash())
	fmt.Println()

	b, _ := tx.MarshalBinary()
	fmt.Println("encoded tx, from:", addr)
	fmt.Println(common.Bytes2Hex(b))
	return nil
}
