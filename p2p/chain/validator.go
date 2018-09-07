package chain

import (
	"github.com/invin/kkchain/core"
	"github.com/invin/kkchain/core/types"
)

type BlockValidator struct {
	bc *core.BlockChain // Canonical block chain
}

// NewBlockValidator returns a new block validator which is safe for re-use
func NewBlockValidator(blockchain *core.BlockChain) *BlockValidator {
	validator := &BlockValidator{
		bc: blockchain, // Canonical block chain
	}
	return validator
}

func (v *BlockValidator) ValidateBody(block *types.Block) error {
	// Check whether the block's known, and if not, that it's linkable
	if v.bc.HasBlockAndState(block.Hash(), block.NumberU64()) {
		return core.ErrKnownBlock
	}
	return nil
}
