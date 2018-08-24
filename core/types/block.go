package types

import (
	"encoding/binary"
	"math/big"
	"sync/atomic"
	"time"

	"github.com/invin/kkchain/common"
)

// nonce for POW computation
type BlockNonce [8]byte

// from uint64 to BlockNonce
func EncodeNonce(i uint64) BlockNonce {
	var n BlockNonce
	binary.BigEndian.PutUint64(n[:], i)
	return n
}

// from BlockNonce to uint64
func (n BlockNonce) Uint64() uint64 {
	return binary.BigEndian.Uint64(n[:])
}

type Header struct {
	ParentHash  common.Hash    `json:"parentHash"       gencodec:"required"`
	Miner       common.Address `json:"miner"            gencodec:"required"`
	StateRoot   common.Hash    `json:"stateRoot"        gencodec:"required"`
	TxRoot      common.Hash    `json:"transactionsRoot" gencodec:"required"`
	ReceiptRoot common.Hash    `json:"receiptsRoot"     gencodec:"required"`
	Difficulty  *big.Int       `json:"difficulty"       gencodec:"required"`
	Number      *big.Int       `json:"number"           gencodec:"required"`
	Time        *big.Int       `json:"timestamp"        gencodec:"required"`
	Extra       []byte         `json:"extraData"        gencodec:"required"`
	MixDigest   common.Hash    `json:"mixHash"          gencodec:"required"`
	Nonce       BlockNonce     `json:"nonce"            gencodec:"required"`
}

// Hash returns the block hash of the header
func (h *Header) Hash() common.Hash {
	// TODO:
	return common.Hash{}
}

type Block struct {
	header       *Header
	transactions Transactions

	// caches
	hash atomic.Value
	size atomic.Value

	// Td is used by package core to store the total difficulty
	// of the chain up to and including the block.
	td *big.Int

	// These fields are used by package eth to track
	// inter-peer block relay.
	ReceivedAt   time.Time
	ReceivedFrom interface{}
}

func NewBlock(header *Header, txs []*Transaction) *Block {
	// TODO:
	return nil
}

func NewBlockWithHeader(header *Header) *Block {
	return &Block{header: CopyHeader(header)}
}

func CopyHeader(h *Header) *Header {
	cpy := *h
	if cpy.Time = new(big.Int); h.Time != nil {
		cpy.Time.Set(h.Time)
	}
	if cpy.Difficulty = new(big.Int); h.Difficulty != nil {
		cpy.Difficulty.Set(h.Difficulty)
	}
	if cpy.Number = new(big.Int); h.Number != nil {
		cpy.Number.Set(h.Number)
	}
	if len(h.Extra) > 0 {
		cpy.Extra = make([]byte, len(h.Extra))
		copy(cpy.Extra, h.Extra)
	}
	return &cpy
}

func (b *Block) Transactions() Transactions { return b.transactions }

// get tx with given tx hash
func (b *Block) GetTransaction(hash common.Hash) *Transaction {
	// TODO:
	return nil
}

func (b *Block) Number() *big.Int     { return new(big.Int).Set(b.header.Number) }
func (b *Block) Difficulty() *big.Int { return new(big.Int).Set(b.header.Difficulty) }
func (b *Block) Time() *big.Int       { return new(big.Int).Set(b.header.Time) }

func (b *Block) NumberU64() uint64        { return b.header.Number.Uint64() }
func (b *Block) MixDigest() common.Hash   { return b.header.MixDigest }
func (b *Block) Nonce() uint64            { return binary.BigEndian.Uint64(b.header.Nonce[:]) }
func (b *Block) Miner() common.Address    { return b.header.Miner }
func (b *Block) StateRoot() common.Hash   { return b.header.StateRoot }
func (b *Block) ParentHash() common.Hash  { return b.header.ParentHash }
func (b *Block) TxRoot() common.Hash      { return b.header.TxRoot }
func (b *Block) ReceiptRoot() common.Hash { return b.header.ReceiptRoot }
func (b *Block) Extra() []byte            { return common.CopyBytes(b.header.Extra) }

func (b *Block) Header() *Header { return CopyHeader(b.header) }

func (b *Block) WithSeal(header *Header) *Block {
	cpy := *header
	return &Block{
		header:       &cpy,
		transactions: b.transactions,
	}
}

// returns the hash of block header.
func (b *Block) Hash() common.Hash {
	if hash := b.hash.Load(); hash != nil {
		return hash.(common.Hash)
	}
	v := b.header.Hash()
	b.hash.Store(v)
	return v
}
