package core

import (
	"sync/atomic"

	"github.com/invin/kkchain/common"
	"github.com/invin/kkchain/core/types"
	"github.com/invin/kkchain/event"
)

//currently for testing purposes
//TODO：the subsequent need to really implement blockchain
type BlockChain struct {
	syncStartFeed event.Feed
	syncDoneFeed  event.Feed
	txsFeed       event.Feed
	newBlockFeed  event.Feed
	scope         event.SubscriptionScope

	genesisBlock *types.Block
	currentBlock atomic.Value

	// TODO: need chain config
	chainID uint64
}

func NewBlockChain() *BlockChain {
	bc := &BlockChain{genesisBlock: DefaultGenesisBlock().ToBlock()}
	bc.currentBlock.Store(bc.genesisBlock)
	return bc
}

func (bc *BlockChain) ChainID() uint64 {
	return bc.chainID
}

func (bc *BlockChain) GenesisBlock() *types.Block {
	return bc.genesisBlock
}

// CurrentBlock retrieves the current head block of the canonical chain. The
// block is retrieved from the blockchain's internal cache.
func (bc *BlockChain) CurrentBlock() *types.Block {
	return bc.currentBlock.Load().(*types.Block)
}

// CurrentHeader retrieves the current header from the local chain.
func (bc *BlockChain) CurrentHeader() *types.Header {
	return bc.currentBlock.Load().(*types.Block).Header()
}

// GetHeader retrieves a block header from the database by hash and number.
func (bc *BlockChain) GetHeader(hash common.Hash, number uint64) *types.Header {
	return nil
}

// GetHeaderByNumber retrieves a block header from the database by number.
func (bc *BlockChain) GetHeaderByNumber(number uint64) *types.Header {
	return nil
}

// GetHeaderByHash retrieves a block header from the database by its hash.
func (bc *BlockChain) GetHeaderByHash(hash common.Hash) *types.Header {
	return nil
}

// GetBlock retrieves a block from the database by hash and number.
func (bc *BlockChain) GetBlock(hash common.Hash, number uint64) *types.Block {
	return nil
}

func (bc *BlockChain) GetBlockByHash(hash common.Hash) *types.Block {
	return nil
}

func (bc *BlockChain) GetReceiptByHash(hash common.Hash) *types.Receipt {
	return nil
}

func (bc *BlockChain) SubscribeSyncStartEvent(ch chan<- struct{}) event.Subscription {
	return bc.scope.Track(bc.syncStartFeed.Subscribe(ch))
}

func (bc *BlockChain) PostSyncStartEvent(event struct{}) {
	bc.syncStartFeed.Send(event)
}

func (bc *BlockChain) SubscribeSyncDoneEvent(ch chan<- struct{}) event.Subscription {
	return bc.scope.Track(bc.syncDoneFeed.Subscribe(ch))
}

func (bc *BlockChain) PostSyncDoneEvent(event struct{}) {
	bc.syncDoneFeed.Send(event)
}

func (bc *BlockChain) SubscribeTxsEvent(ch chan<- []*types.Transaction) event.Subscription {
	return bc.scope.Track(bc.txsFeed.Subscribe(ch))
}

func (bc *BlockChain) PostTxsEvent(txs []*types.Transaction) {
	bc.txsFeed.Send(txs)
}

func (bc *BlockChain) SubscribeNewBlockEvent(ch chan<- *types.Block) event.Subscription {
	return bc.scope.Track(bc.newBlockFeed.Subscribe(ch))
}

func (bc *BlockChain) PostNewBlockEvent(block *types.Block) {
	bc.currentBlock.Store(block)
	bc.newBlockFeed.Send(block)
}
