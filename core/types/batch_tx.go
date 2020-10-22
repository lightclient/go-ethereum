package types

import (
	"math/big"
	"time"

	"encoding/json"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
)

type BatchTransaction struct {
	Chain        *big.Int     `json:"chain"        gencodec:"required"`
	AccountNonce uint64       `json:"nonce"        gencodec:"required"`
	Price        *big.Int     `json:"gasPrice"     gencodec:"required"`
	Children     []ChildBatch `json:"transactions" gencodec:"required"`

	// Sponsor signature values
	V *big.Int `json:"v" gencodec:"required"`
	R *big.Int `json:"r" gencodec:"required"`
	S *big.Int `json:"s" gencodec:"required"`
}

type ChildBatch struct {
	Type         uint8              `json:"type"         gencodec:"required"`
	Chain        *big.Int           `json:"chain"        gencodec:"required"`
	AccountNonce uint64             `json:"nonce"        gencodec:"required"`
	Txs          []ChildTransaction `json:"transactions" gencodec:"required"`
	MaxPrice     *big.Int           `json:"gasPrice"     gencodec:"required"`

	// Signature values
	V *big.Int `json:"v" gencodec:"required"`
	R *big.Int `json:"r" gencodec:"required"`
	S *big.Int `json:"s" gencodec:"required"`
}

func (pkg *ChildBatch) RawSignatureValues() (v, r, s *big.Int) {
	return pkg.V, pkg.R, pkg.S
}

type ChildTransaction struct {
	Flags     uint8           `json:"flags"    gencodec:"required"`
	Recipient *common.Address `json:"to"       rlp:"nil"` // nil means contract creation
	Amount    *big.Int        `json:"value"    gencodec:"required"`
	Payload   []byte          `json:"input"    gencodec:"required"`
	Extra     []byte          `json:"extra"    gencodec:"required"`
	GasLimit  uint64          `json:"gas"      gencodec:"required"`
}

func NewChildTransaction(flags uint8, to common.Address, amount *big.Int, gasLimit uint64, data []byte) *ChildTransaction {
	if len(data) > 0 {
		data = common.CopyBytes(data)
	}
	c := ChildTransaction{
		Flags:     flags,
		Recipient: &to,
		Amount:    new(big.Int),
		Payload:   data,
		GasLimit:  gasLimit,
	}
	if amount != nil {
		c.Amount.Set(amount)
	}
	return &c
}

func NewChildBatch(ty uint8, nonce uint64, maxGasPrice *big.Int, txs []ChildTransaction) *ChildBatch {
	c := ChildBatch{
		Type:         ty,
		AccountNonce: nonce,
		Txs:          txs,
		MaxPrice:     new(big.Int),
		V:            new(big.Int),
		R:            new(big.Int),
		S:            new(big.Int),
	}
	if maxGasPrice != nil {
		c.MaxPrice.Set(maxGasPrice)
	}
	return &c
}

func NewBatchTransaction(chainId *big.Int, batch []ChildBatch) *Transaction {
	i := BatchTransaction{
		Chain:    new(big.Int),
		Children: batch,
		V:        new(big.Int),
		R:        new(big.Int),
		S:        new(big.Int),
	}
	if chainId != nil {
		i.Chain.Set(chainId)
	}
	return &Transaction{
		typ:   BatchTxId,
		inner: &i,
		time:  time.Now(),
	}
}

func (tx *BatchTransaction) ChainId() *big.Int {
	return tx.Chain
}

func (tx *BatchTransaction) Protected() bool {
	return true
}

func (tx *BatchTransaction) Data() []byte { return []byte{} }
func (tx *BatchTransaction) Gas() uint64 {
	var limit uint64 = params.BatchTxGas

	for _, child := range tx.Children {
		for _, tx := range child.Txs {
			limit += tx.GasLimit
		}
	}

	return limit
}
func (tx *BatchTransaction) GasPrice() *big.Int { return new(big.Int).Set(tx.Price) }
func (tx *BatchTransaction) Value() *big.Int    { return big.NewInt(0) }
func (tx *BatchTransaction) Nonce() uint64      { return tx.AccountNonce }
func (tx *BatchTransaction) CheckNonce() bool   { return true }

func (tx *BatchTransaction) Hash() common.Hash {
	return rlpHash(tx)
}

// To returns the recipient address of the transaction.
// It returns nil if the transaction is a contract creation.
func (tx *BatchTransaction) To() *common.Address {
	// TODO: this is bad
	return &common.Address{1}
}

func (tx *BatchTransaction) AccessList() *AccessList {
	return nil
}

// RawSignatureValues returns the V, R, S signature values of the transaction.
// The return values should not be modified by the caller.
func (tx *BatchTransaction) RawSignatureValues() (v, r, s *big.Int) {
	return tx.V, tx.R, tx.S
}

// MarshalJSONWithHash marshals as JSON with a hash.
func (t *BatchTransaction) MarshalJSONWithHash(hash *common.Hash) ([]byte, error) {
	type txdata struct {
		ChainId      *hexutil.Big    `json:"chainId"    gencodec:"required"`
		AccountNonce hexutil.Uint64  `json:"nonce"    gencodec:"required"`
		Price        *hexutil.Big    `json:"gasPrice" gencodec:"required"`
		GasLimit     hexutil.Uint64  `json:"gas"      gencodec:"required"`
		Recipient    *common.Address `json:"to"       rlp:"nil"`
		Amount       *hexutil.Big    `json:"value"    gencodec:"required"`
		Payload      hexutil.Bytes   `json:"input"    gencodec:"required"`
		V            *hexutil.Big    `json:"v" gencodec:"required"`
		R            *hexutil.Big    `json:"r" gencodec:"required"`
		S            *hexutil.Big    `json:"s" gencodec:"required"`
		Hash         *common.Hash    `json:"hash" rlp:"-"`
	}

	var enc txdata

	// TODO
	// enc.ChainId = (*hexutil.Big)(t.Chain)
	// enc.AccountNonce = hexutil.Uint64(t.AccountNonce)
	// enc.Price = (*hexutil.Big)(t.Price)
	// enc.GasLimit = hexutil.Uint64(t.GasLimit)
	// enc.Recipient = t.Recipient
	// enc.Amount = (*hexutil.Big)(t.Amount)
	// enc.Payload = t.Payload
	// enc.V = (*hexutil.Big)(t.V)
	// enc.R = (*hexutil.Big)(t.R)
	// enc.S = (*hexutil.Big)(t.S)
	// enc.Hash = hash

	return json.Marshal(&enc)
}

// UnmarshalJSON unmarshals from JSON.
func (t *BatchTransaction) UnmarshalJSON(input []byte) error {
	type txdata struct {
		ChainId      *hexutil.Big    `json:"chainId"    gencodec:"required"`
		AccountNonce *hexutil.Uint64 `json:"nonce"    gencodec:"required"`
		Price        *hexutil.Big    `json:"gasPrice" gencodec:"required"`
		GasLimit     *hexutil.Uint64 `json:"gas"      gencodec:"required"`
		Recipient    *common.Address `json:"to"       rlp:"nil"`
		Amount       *hexutil.Big    `json:"value"    gencodec:"required"`
		Payload      *hexutil.Bytes  `json:"input"    gencodec:"required"`
		V            *hexutil.Big    `json:"v" gencodec:"required"`
		R            *hexutil.Big    `json:"r" gencodec:"required"`
		S            *hexutil.Big    `json:"s" gencodec:"required"`
	}
	// var dec txdata
	// if err := json.Unmarshal(input, &dec); err != nil {
	//         return err
	// }
	// if dec.ChainId == nil {
	//         return errors.New("missing required field 'chainId' for txdata")
	// }
	// t.Chain = (*big.Int)(dec.ChainId)
	// if dec.AccountNonce == nil {
	//         return errors.New("missing required field 'nonce' for txdata")
	// }
	// t.AccountNonce = uint64(*dec.AccountNonce)
	// if dec.Price == nil {
	//         return errors.New("missing required field 'gasPrice' for txdata")
	// }
	// t.Price = (*big.Int)(dec.Price)
	// if dec.GasLimit == nil {
	//         return errors.New("missing required field 'gas' for txdata")
	// }
	// t.GasLimit = uint64(*dec.GasLimit)
	// if dec.Recipient != nil {
	//         t.Recipient = dec.Recipient
	// }
	// if dec.Amount == nil {
	//         return errors.New("missing required field 'value' for txdata")
	// }
	// t.Amount = (*big.Int)(dec.Amount)
	// if dec.Payload == nil {
	//         return errors.New("missing required field 'input' for txdata")
	// }
	// t.Payload = *dec.Payload
	// if dec.V == nil {
	//         return errors.New("missing required field 'v' for txdata")
	// }
	// t.V = (*big.Int)(dec.V)
	// if dec.R == nil {
	//         return errors.New("missing required field 'r' for txdata")
	// }
	// t.R = (*big.Int)(dec.R)
	// if dec.S == nil {
	//         return errors.New("missing required field 's' for txdata")
	// }
	// t.S = (*big.Int)(dec.S)

	// TODO
	return nil
}
