package consensus

import (
	"github.com/invin/kkchain/types"
)

type Context struct {
	//parent block

	//chain reader

	//state

	//now time

	//validators

	//self

	//proposer

}

// ChainReader defines a small collection of methods needed to access the local
// blockchain during header and/or uncle verification.
type ChainReader interface {

	// CurrentHeader retrieves the current header from the local chain.
	CurrentHeader() *types.Header

	// GetHeader retrieves a block header from the database by hash and number.
	GetHeader(hash []byte, number uint64) *types.Header

	// GetHeaderByNumber retrieves a block header from the database by number.
	GetHeaderByNumber(number uint64) *types.Header

	// GetHeaderByHash retrieves a block header from the database by its hash.
	GetHeaderByHash(hash []byte) *types.Header

	// GetBlock retrieves a block from the database by hash and number.
	GetBlock(hash []byte, number uint64) *types.Block
}

// Engine is an algorithm agnostic consensus engine.
type Engine interface {
	Initialize(chain ChainReader, txs []types.Transaction) (*types.Block, error)

	Execute(chain ChainReader, block *types.Block, stop <-chan struct{}) (*types.Block, error)

	Finalize(chain ChainReader, block *types.Block) error

	Abort(block *types.Block)

	VerifyHeader(header *types.Header) error
	Verify(block *types.Block) error
}

type Network interface {
	Broadcast()
}
