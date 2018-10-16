package node

import (
	"context"
	"math/big"

	"github.com/invin/kkchain/accounts"
	"github.com/invin/kkchain/api"
	"github.com/invin/kkchain/common"
	"github.com/invin/kkchain/core/rawdb"
	"github.com/invin/kkchain/core/types"
	"github.com/invin/kkchain/params"
	"github.com/invin/kkchain/rpc"
	"github.com/invin/kkchain/storage"
)

// APIBackend implements api.Backend for full nodes
type APIBackend struct {
	kkchain *Node
	gpo     *api.Oracle
}

func (b *APIBackend) SetGasOracle(gpo *api.Oracle) {
	b.gpo = gpo
}

func (b *APIBackend) SuggestPrice(ctx context.Context) (*big.Int, error) {
	return b.gpo.SuggestPrice(ctx)
}

func (b *APIBackend) ChainDb() storage.Database {
	return b.kkchain.ChainDb()
}

func (b *APIBackend) AccountManager() *accounts.Manager {
	return b.kkchain.AccountManager()
}

func (b *APIBackend) HeaderByNumber(ctx context.Context, blockNr rpc.BlockNumber) (*types.Header, error) {
	// Pending block is only known by the miner
	if blockNr == rpc.PendingBlockNumber {
		block := b.kkchain.Miner().PendingBlock()
		return block.Header(), nil
	}
	// Otherwise resolve and return the block
	if blockNr == rpc.LatestBlockNumber {
		return b.kkchain.BlockChain().CurrentBlock().Header(), nil
	}
	return b.kkchain.BlockChain().GetHeaderByNumber(uint64(blockNr)), nil
}

func (b *APIBackend) BlockByNumber(ctx context.Context, blockNr rpc.BlockNumber) (*types.Block, error) {
	// Pending block is only known by the miner
	if blockNr == rpc.PendingBlockNumber {
		block := b.kkchain.Miner().PendingBlock()
		return block, nil
	}
	// Otherwise resolve and return the block
	if blockNr == rpc.LatestBlockNumber {
		return b.kkchain.BlockChain().CurrentBlock(), nil
	}
	return b.kkchain.BlockChain().GetBlockByNumber(uint64(blockNr)), nil
}

func (b *APIBackend) GetReceipts(ctx context.Context, hash common.Hash) (types.Receipts, error) {
	if number := rawdb.ReadHeaderNumber(b.kkchain.ChainDb(), hash); number != nil {
		return rawdb.ReadReceipts(b.kkchain.ChainDb(), hash, *number), nil
	}
	return nil, nil
}

func (b *APIBackend) SendTx(ctx context.Context, signedTx *types.Transaction) error {
	return b.kkchain.TxPool().AddLocal(signedTx)
}

func (b *APIBackend) GetPoolNonce(ctx context.Context, addr common.Address) (uint64, error) {
	return b.kkchain.TxPool().State().GetNonce(addr), nil
}

// ChainConfig returns the active chain configuration.
func (b *APIBackend) ChainConfig() *params.ChainConfig {
	return b.kkchain.ChainConfig()
}
