package pow

import (
	"github.com/invin/kkchain/consensus"
	"github.com/invin/kkchain/types"

	"github.com/op/go-logging"
)

var (
	logger = logging.MustGetLogger("pow")
)

type Pow struct {
}

func (*Pow) Initialize(chain consensus.ChainReader, txs []types.Transaction) (*types.Block, error) {
	return nil, nil
}

func (*Pow) Execute(chain consensus.ChainReader, block *types.Block, result chan<- *types.Block) error {
	return nil
}

func (*Pow) Finalize(chain consensus.ChainReader, block *types.Block) error {
	return nil
}

func (*Pow) Abort(block *types.Block) {

}

func (*Pow) VerifyHeader(header *types.Header) error {
	return nil
}

func (*Pow) Verify(block *types.Block) error {
	return nil
}
