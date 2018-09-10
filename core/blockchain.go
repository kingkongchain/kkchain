package core

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math/big"
	"path/filepath"
	"sync"
	"sync/atomic"

	mrand "math/rand"

	lru "github.com/hashicorp/golang-lru"
	"github.com/invin/kkchain/common"
	"github.com/invin/kkchain/consensus"
	"github.com/invin/kkchain/core/rawdb"
	"github.com/invin/kkchain/core/state"
	"github.com/invin/kkchain/core/types"
	"github.com/invin/kkchain/event"
	"github.com/invin/kkchain/storage"
	"github.com/invin/kkchain/storage/memdb"
	"github.com/invin/kkchain/storage/rocksdb"

	log "github.com/sirupsen/logrus"
)

const (
	tdCacheLimit        = 1024
	numberCacheLimit    = 2048
	headerCacheLimit    = 512
	bodyCacheLimit      = 256
	blockCacheLimit     = 256
	maxFutureBlocks     = 256
	maxTimeFutureBlocks = 30
	badBlockLimit       = 10
	triesInMemory       = 128

	// BlockChainVersion ensures that an incompatible database forces a resync from scratch.
	BlockChainVersion = 3
)

var (
	ErrNoGenesis = errors.New("Genesis not found in chain")
)

type Config struct {
	DataDir string
}

//currently for testing purposes
//TODO：the subsequent need to really implement blockchain
type BlockChain struct {
	mu     *sync.RWMutex
	wg     sync.WaitGroup // chain processing wait group for shutting down
	procmu sync.RWMutex   // block processor lock
	config Config

	engine    consensus.Engine
	validator Validator // block and state validator interface

	db           storage.Database
	blockCache   *lru.Cache // Cache for the most recent entire blocks
	futureBlocks *lru.Cache // future blocks are blocks added for later processing

	logsFeed          event.Feed
	chainHeadFeed     event.Feed
	newMinedBlockFeed event.Feed
	scope             event.SubscriptionScope

	stateCache state.Database // State database to reuse between imports (contains state cache)

	genesisBlock *types.Block
	currentBlock atomic.Value

	headerCache *lru.Cache // Cache for the most recent block headers
	tdCache     *lru.Cache // Cache for the most recent block total difficulties
	numberCache *lru.Cache // Cache for the most recent block numbers

	//for test TODO：move to sync mgr
	syncStartFeed event.Feed
	syncDoneFeed  event.Feed

	// TODO: need chain config
	chainID uint64

	txsFeed      event.Feed
	newBlockFeed event.Feed
}

func NewBlockChain(db storage.Database, engine consensus.Engine) (*BlockChain, error) {

	headerCache, _ := lru.New(headerCacheLimit)
	blockCache, _ := lru.New(blockCacheLimit)
	futureBlocks, _ := lru.New(maxFutureBlocks)
	tdCache, _ := lru.New(tdCacheLimit)
	numberCache, _ := lru.New(numberCacheLimit)

	bc := &BlockChain{
		mu:           &sync.RWMutex{},
		db:           db,
		stateCache:   state.NewDatabase(db),
		headerCache:  headerCache,
		tdCache:      tdCache,
		numberCache:  numberCache,
		blockCache:   blockCache,
		futureBlocks: futureBlocks,
		engine:       engine,
	}

	bc.SetValidator(NewBlockValidator(bc))
	//init genesis block
	bc.genesisBlock = bc.GetBlockByNumber(0)
	if bc.genesisBlock == nil {
		return nil, ErrNoGenesis
	}

	//load latest state
	if err := bc.loadLastState(); err != nil {
		return nil, err
	}

	//for test
	//bc.currentBlock.Store(bc.genesisBlock)

	return bc, nil
}

func (bc *BlockChain) ChainID() uint64 {
	return bc.chainID
}

func (bc *BlockChain) GenesisBlock() *types.Block {
	return bc.genesisBlock
}

// loadLastState loads the last known chain state from the database. This method
// assumes that the chain manager mutex is held.
func (bc *BlockChain) loadLastState() error {
	// Restore the last known head block
	head := rawdb.ReadHeadBlockHash(bc.db)
	if head == (common.Hash{}) {
		// Corrupt or empty database, init from scratch
		log.Warn("Empty database, resetting chain")
		return bc.Reset()
	}
	// Make sure the entire head block is available
	currentBlock := bc.GetBlockByHash(head)
	if currentBlock == nil {
		// Corrupt or empty database, init from scratch
		log.Warn("Head block missing, resetting chain", "hash", head)
		return bc.Reset()
	}
	// Make sure the state associated with the block is available
	if _, err := state.New(currentBlock.StateRoot(), bc.stateCache); err != nil {
		// Dangling block without a state associated, init from scratch
		log.Warn("Head state missing, repairing chain", "number", currentBlock.Number(), "hash", currentBlock.Hash())
		if err := bc.repair(&currentBlock); err != nil {
			return err
		}
	}
	// Everything seems to be fine, set as the head block
	bc.currentBlock.Store(currentBlock)

	// Restore the last known head header
	currentHeader := currentBlock.Header()
	if head := rawdb.ReadHeadHeaderHash(bc.db); head != (common.Hash{}) {
		if header := bc.GetHeaderByHash(head); header != nil {
			currentHeader = header
		}
	}

	headerTd := bc.GetTd(currentHeader.Hash(), currentHeader.Number.Uint64())
	blockTd := bc.GetTd(currentBlock.Hash(), currentBlock.NumberU64())

	log.Info("Loaded most recent local header", "number", currentHeader.Number, "hash", currentHeader.Hash(), "td", headerTd)
	log.Info("Loaded most recent local full block", "number", currentBlock.Number(), "hash", currentBlock.Hash(), "td", blockTd)

	return nil
}

// repair tries to repair the current blockchain by rolling back the current block
// until one with associated state is found. This is needed to fix incomplete db
// writes caused either by crashes/power outages, or simply non-committed tries.
//
// This method only rolls back the current block. The current header and current
// fast block are left intact.
func (bc *BlockChain) repair(head **types.Block) error {
	for {
		// Abort if we've rewound to a head block that does have associated state
		if _, err := state.New((*head).StateRoot(), bc.stateCache); err == nil {
			log.Info("Rewound blockchain to past state", "number", (*head).Number(), "hash", (*head).Hash())
			return nil
		}
		// Otherwise rewind one block and recheck state availability there
		(*head) = bc.GetBlock((*head).ParentHash(), (*head).NumberU64()-1)
	}
}

// Reset purges the entire blockchain, restoring it to its genesis state.
func (bc *BlockChain) Reset() error {
	return bc.ResetWithGenesisBlock(bc.genesisBlock)
}

// ResetWithGenesisBlock purges the entire blockchain, restoring it to the
// specified genesis state.
func (bc *BlockChain) ResetWithGenesisBlock(genesis *types.Block) error {
	// Dump the entire block chain and purge the caches
	if err := bc.SetHead(0); err != nil {
		return err
	}
	bc.mu.Lock()
	defer bc.mu.Unlock()

	// Prepare the genesis block and reinitialise the chain
	if err := bc.WriteTd(genesis.Hash(), genesis.NumberU64(), genesis.Difficulty()); err != nil {
		log.Error("Failed to write genesis block TD", "err", err)
	}
	rawdb.WriteBlock(bc.db, genesis)

	bc.genesisBlock = genesis
	bc.insert(bc.genesisBlock)
	bc.currentBlock.Store(bc.genesisBlock)

	return nil
}

// SetHead rewinds the local chain to a new head. In the case of headers, everything
// above the new head will be deleted and the new one set. In the case of blocks
// though, the head may be further rewound if block bodies are missing (non-archive
// nodes after a fast sync).
func (bc *BlockChain) SetHead(head uint64) error {
	log.Warnf("Rewinding blockchain", "target", head)

	bc.mu.Lock()
	defer bc.mu.Unlock()

	height := uint64(0)
	if hdr := bc.CurrentHeader(); hdr != nil {
		height = hdr.Number.Uint64()
	}
	batch := bc.db.NewBatch()
	for hdr := bc.CurrentHeader(); hdr != nil && hdr.Number.Uint64() > head; hdr = bc.CurrentHeader() {
		hash := hdr.Hash()
		num := hdr.Number.Uint64()
		rawdb.DeleteBody(batch, hash, num)
		rawdb.DeleteHeader(batch, hash, num)
		rawdb.DeleteTd(batch, hash, num)

		bc.currentBlock.Store(bc.GetBlock(hdr.ParentHash, hdr.Number.Uint64()-1))
	}
	// Roll back the canonical chain numbering
	for i := height; i > head; i-- {
		rawdb.DeleteCanonicalHash(batch, i)
	}
	batch.Write()

	// Clear out any stale content from the caches
	bc.headerCache.Purge()
	bc.tdCache.Purge()
	bc.numberCache.Purge()

	if bc.CurrentHeader() == nil {
		bc.currentBlock.Store(bc.genesisBlock)
	}
	currentHeaderHash := bc.CurrentHeader().Hash()

	rawdb.WriteHeadHeaderHash(bc.db, currentHeaderHash)

	// Clear out any stale content from the caches
	bc.blockCache.Purge()

	if currentBlock := bc.CurrentBlock(); currentBlock != nil {
		if _, err := state.New(currentBlock.StateRoot(), bc.stateCache); err != nil {
			// Rewound state missing, rolled back to before pivot, reset to genesis
			bc.currentBlock.Store(bc.genesisBlock)
		}
	}

	// If either blocks reached nil, reset to the genesis state
	if currentBlock := bc.CurrentBlock(); currentBlock == nil {
		bc.currentBlock.Store(bc.genesisBlock)
	}

	currentBlock := bc.CurrentBlock()

	rawdb.WriteHeadBlockHash(bc.db, currentBlock.Hash())

	return bc.loadLastState()
}

//TODO: remove to node config
func OpenDatabase(config *Config, name string) (storage.Database, error) {
	if config.DataDir == "" {
		return memdb.New(), nil
	}
	db, err := rocksdb.New(resolvePath(name), nil)
	if err != nil {
		return nil, err
	}
	return db, nil
}

// ResolvePath resolves path in the instance directory.
func resolvePath(path string) string {
	if filepath.IsAbs(path) {
		return path
	}

	return filepath.Join("./geth", path)
}

//END TODO

// WriteBlockWithState writes the block and all associated state to the database.
func (bc *BlockChain) WriteBlockWithState(block *types.Block, receipts []*types.Receipt, state *state.StateDB) error {
	bc.wg.Add(1)
	defer bc.wg.Done()

	// Calculate the total difficulty of the block
	ptd := bc.GetTd(block.ParentHash(), block.NumberU64()-1)
	if ptd == nil {
		return consensus.ErrUnknownAncestor
	}
	// Make sure no inconsistent state is leaked during insertion
	bc.mu.Lock()
	defer bc.mu.Unlock()

	currentBlock := bc.CurrentBlock()
	localTd := bc.GetTd(currentBlock.Hash(), currentBlock.NumberU64())
	externTd := new(big.Int).Add(block.Difficulty(), ptd)

	// Irrelevant of the canonical status, write the block itself to the database
	if err := bc.WriteTd(block.Hash(), block.NumberU64(), externTd); err != nil {
		return err
	}
	rawdb.WriteBlock(bc.db, block)

	root, err := state.Commit(false)
	if err != nil {
		return err
	}
	triedb := bc.stateCache.TrieDB()

	// If we're running an archive node, always flush
	if err := triedb.Commit(root, false); err != nil {
		return err
	}

	// Write other block data using a batch.
	//TODO:implement writeReceipts
	//batch := bc.db.NewBatch()
	//rawdb.WriteReceipts(batch, block.Hash(), block.NumberU64(), receipts)

	// If the total difficulty is higher than our known, add it to the canonical chain
	// Second clause in the if statement reduces the vulnerability to selfish mining.
	// Please refer to http://www.cs.cornell.edu/~ie53/publications/btcProcFC.pdf
	reorg := externTd.Cmp(localTd) > 0
	currentBlock = bc.CurrentBlock()
	if !reorg && externTd.Cmp(localTd) == 0 {
		// Split same-difficulty blocks by number, then at random
		reorg = block.NumberU64() < currentBlock.NumberU64() || (block.NumberU64() == currentBlock.NumberU64() && mrand.Float64() < 0.5)
	}
	if reorg {
		// Reorganise the chain if the parent is not the head block
		if block.ParentHash() != currentBlock.Hash() {
			if err := bc.reorg(currentBlock, block); err != nil {
				return err
			}
		}
		// Write the positional metadata for transaction/receipt lookups and preimages
		//rawdb.WriteTxLookupEntries(batch, block)
		//rawdb.WritePreimages(batch, block.NumberU64(), state.Preimages())

	}

	// if err := batch.Write(); err != nil {
	// 	return err
	// }

	// Set new head.
	bc.insert(block)
	return nil
}

// reorgs takes two blocks, an old chain and a new chain and will reconstruct the blocks and inserts them
// to be part of the new canonical chain and accumulates potential missing transactions and post an
// event about them
func (bc *BlockChain) reorg(oldBlock, newBlock *types.Block) error {
	var (
		newChain    types.Blocks
		oldChain    types.Blocks
		commonBlock *types.Block
		deletedTxs  types.Transactions
	)

	// first reduce whoever is higher bound
	if oldBlock.NumberU64() > newBlock.NumberU64() {
		// reduce old chain
		for ; oldBlock != nil && oldBlock.NumberU64() != newBlock.NumberU64(); oldBlock = bc.GetBlock(oldBlock.ParentHash(), oldBlock.NumberU64()-1) {
			oldChain = append(oldChain, oldBlock)
			deletedTxs = append(deletedTxs, oldBlock.Transactions()...)
		}
	} else {
		// reduce new chain and append new chain blocks for inserting later on
		for ; newBlock != nil && newBlock.NumberU64() != oldBlock.NumberU64(); newBlock = bc.GetBlock(newBlock.ParentHash(), newBlock.NumberU64()-1) {
			newChain = append(newChain, newBlock)
		}
	}
	if oldBlock == nil {
		return fmt.Errorf("Invalid old chain")
	}
	if newBlock == nil {
		return fmt.Errorf("Invalid new chain")
	}

	for {
		if oldBlock.Hash() == newBlock.Hash() {
			commonBlock = oldBlock
			break
		}

		oldChain = append(oldChain, oldBlock)
		newChain = append(newChain, newBlock)
		deletedTxs = append(deletedTxs, oldBlock.Transactions()...)

		oldBlock, newBlock = bc.GetBlock(oldBlock.ParentHash(), oldBlock.NumberU64()-1), bc.GetBlock(newBlock.ParentHash(), newBlock.NumberU64()-1)
		if oldBlock == nil {
			return fmt.Errorf("Invalid old chain")
		}
		if newBlock == nil {
			return fmt.Errorf("Invalid new chain")
		}
	}
	// Ensure the user sees large reorgs
	if len(oldChain) > 0 && len(newChain) > 0 {
		logFn := log.Debug
		if len(oldChain) > 63 {
			logFn = log.Warn
		}
		logFn("Chain split detected", "number", commonBlock.Number(), "hash", commonBlock.Hash(),
			"drop", len(oldChain), "dropfrom", oldChain[0].Hash(), "add", len(newChain), "addfrom", newChain[0].Hash())
	} else {
		log.Error("Impossible reorg, please file an issue", "oldnum", oldBlock.Number(), "oldhash", oldBlock.Hash(), "newnum", newBlock.Number(), "newhash", newBlock.Hash())
	}
	// Insert the new chain, taking care of the proper incremental order
	var addedTxs types.Transactions
	for i := len(newChain) - 1; i >= 0; i-- {
		// insert the block in the canonical way, re-writing history
		bc.insert(newChain[i])
		// write lookup entries for hash based transaction/receipt searches
		rawdb.WriteTxLookupEntries(bc.db, newChain[i])
		addedTxs = append(addedTxs, newChain[i].Transactions()...)
	}
	// calculate the difference between deleted and added transactions
	diff := types.TxDifference(deletedTxs, addedTxs)
	// When transactions get deleted from the database that means the
	// receipts that were created in the fork must also be deleted
	batch := bc.db.NewBatch()
	for _, tx := range diff {
		rawdb.DeleteTxLookupEntry(batch, tx.Hash())
	}
	batch.Write()

	//if len(oldChain) > 0 {
	//	go func() {
	//		for _, block := range oldChain {
	//			bc.chainSideFeed.Send(ChainSideEvent{Block: block})
	//		}
	//	}()
	//}

	return nil
}

// insert injects a new head block into the current block chain. This method
// assumes that the block is indeed a true head. It will also reset the head
// header and the head fast sync block to this very same block if they are older
// or if they are on a different side chain.
//
// Note, this function assumes that the `mu` mutex is held!
func (bc *BlockChain) insert(block *types.Block) {

	// Add the block to the canonical chain number scheme and mark as the head
	rawdb.WriteCanonicalHash(bc.db, block.Hash(), block.NumberU64())
	rawdb.WriteHeadBlockHash(bc.db, block.Hash())

	bc.currentBlock.Store(block)

}

// State returns a new mutable state based on the current HEAD block.
func (bc *BlockChain) State() (*state.StateDB, error) {
	return bc.StateAt(bc.CurrentBlock().StateRoot())
}

// StateAt returns a new mutable state based on a particular point in time.
func (bc *BlockChain) StateAt(root common.Hash) (*state.StateDB, error) {
	return state.New(root, bc.stateCache)
}

// CurrentBlock retrieves the current head block of the canonical chain. The
// block is retrieved from the blockchain's internal cache.
func (bc *BlockChain) CurrentBlock() *types.Block {
	return bc.currentBlock.Load().(*types.Block)
}

// CurrentHeader retrieves the current header from the local chain.
func (bc *BlockChain) CurrentHeader() *types.Header {
	block := bc.currentBlock.Load()
	if block == nil {
		return nil
	}
	return block.(*types.Block).Header()
}

// GetHeader retrieves a block header from the database by hash and number.
func (bc *BlockChain) GetHeader(hash common.Hash, number uint64) *types.Header {
	// Short circuit if the header's already in the cache, retrieve otherwise
	if header, ok := bc.headerCache.Get(hash); ok {
		return header.(*types.Header)
	}
	header := rawdb.ReadHeader(bc.db, hash, number)
	if header == nil {
		return nil
	}
	// Cache the found header for next time and return
	bc.headerCache.Add(hash, header)
	return header
}

// GetHeaderByNumber retrieves a block header from the database by number.
func (bc *BlockChain) GetHeaderByNumber(number uint64) *types.Header {
	hash := rawdb.ReadCanonicalHash(bc.db, number)
	if hash == (common.Hash{}) {
		return nil
	}
	return bc.GetHeader(hash, number)
}

// GetHeaderByHash retrieves a block header from the database by its hash.
func (bc *BlockChain) GetHeaderByHash(hash common.Hash) *types.Header {
	number := bc.GetBlockNumber(hash)
	if number == nil {
		return nil
	}
	return bc.GetHeader(hash, *number)
}

// HasBlock checks if a block is fully present in the database or not.
func (bc *BlockChain) HasBlock(hash common.Hash, number uint64) bool {
	if bc.blockCache.Contains(hash) {
		return true
	}
	return rawdb.HasBody(bc.db, hash, number)
}

// GetBlock retrieves a block from the database by hash and number.
func (bc *BlockChain) GetBlock(hash common.Hash, number uint64) *types.Block {
	// Short circuit if the block's already in the cache, retrieve otherwise
	if block, ok := bc.blockCache.Get(hash); ok {
		return block.(*types.Block)
	}
	block := rawdb.ReadBlock(bc.db, hash, number)
	if block == nil {
		return nil
	}
	// Cache the found block for next time and return
	bc.blockCache.Add(block.Hash(), block)
	return block
}

// GetBlockByNumber retrieves a block from the database by number, caching it
// (associated with its hash) if found.
func (bc *BlockChain) GetBlockByNumber(number uint64) *types.Block {
	//TODO: read hash by number from rawdb
	hash := rawdb.ReadCanonicalHash(bc.db, number)
	if hash == (common.Hash{}) {
		return nil
	}
	return bc.GetBlock(hash, number)
}

// GetTd retrieves a block's total difficulty in the canonical chain from the
// database by hash and number, caching it if found.
func (bc *BlockChain) GetTd(hash common.Hash, number uint64) *big.Int {
	if cached, ok := bc.tdCache.Get(hash); ok {
		return cached.(*big.Int)
	}
	td := rawdb.ReadTd(bc.db, hash, number)
	if td == nil {
		return nil
	}
	// Cache the found body for next time and return
	bc.tdCache.Add(hash, td)
	return td
}

// GetTdByHash retrieves a block's total difficulty in the canonical chain from the
// database by hash, caching it if found.
func (bc *BlockChain) GetTdByHash(hash common.Hash) *big.Int {
	number := bc.GetBlockNumber(hash)
	if number == nil {
		return nil
	}
	return bc.GetTd(hash, *number)
}

// WriteTd stores a block's total difficulty into the database, also caching it
// along the way.
func (bc *BlockChain) WriteTd(hash common.Hash, number uint64, td *big.Int) error {
	rawdb.WriteTd(bc.db, hash, number, td)
	bc.tdCache.Add(hash, new(big.Int).Set(td))
	return nil
}

// GetBlockByHash retrieves a block from the database by hash, caching it if found.
func (bc *BlockChain) GetBlockByHash(hash common.Hash) *types.Block {
	number := bc.GetBlockNumber(hash)
	if number == nil {
		return nil
	}
	return bc.GetBlock(hash, *number)
}

// GetBlockNumber retrieves the block number belonging to the given hash
// from the cache or database
func (bc *BlockChain) GetBlockNumber(hash common.Hash) *uint64 {
	if cached, ok := bc.numberCache.Get(hash); ok {
		number := cached.(uint64)
		return &number
	}
	number := rawdb.ReadHeaderNumber(bc.db, hash)
	if number != nil {
		bc.numberCache.Add(hash, *number)
	}
	return number
}

func (bc *BlockChain) GetReceiptByHash(hash common.Hash) *types.Receipt {
	return nil
}

//for test
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

// PostChainEvents iterates over the events generated by a chain insertion and
// posts them into the event feed.
// TODO: Should not expose PostChainEvents. The chain events should be posted in WriteBlock.
func (bc *BlockChain) PostChainEvents(events []interface{}, logs []*types.Log) {
	// post event logs for further processing
	if logs != nil {
		bc.logsFeed.Send(logs)
	}
	for _, event := range events {
		switch ev := event.(type) {
		case ChainHeadEvent:
			bc.chainHeadFeed.Send(ev)

		case NewMinedBlockEvent:
			log.Info("*******send newMinedBlockFeed")
			bc.newMinedBlockFeed.Send(ev)

		}
	}
}

// SubscribeLogsEvent registers a subscription of []*types.Log.
func (bc *BlockChain) SubscribeLogsEvent(ch chan<- []*types.Log) event.Subscription {
	return bc.scope.Track(bc.logsFeed.Subscribe(ch))
}

// SubscribeChainHeadEvent registers a subscription of ChainHeadEvent.
func (bc *BlockChain) SubscribeChainHeadEvent(ch chan<- ChainHeadEvent) event.Subscription {
	return bc.scope.Track(bc.chainHeadFeed.Subscribe(ch))
}

func (bc *BlockChain) SubscribeNewMinedBlockEvent(ch chan<- NewMinedBlockEvent) event.Subscription {
	return bc.scope.Track(bc.newMinedBlockFeed.Subscribe(ch))
}

// HasState checks if state trie is fully present in the database or not.
func (bc *BlockChain) HasState(hash common.Hash) bool {
	_, err := bc.stateCache.OpenTrie(hash)
	return err == nil
}

// HasBlockAndState checks if a block and associated state trie is fully present
// in the database or not, caching it if present.
func (bc *BlockChain) HasBlockAndState(hash common.Hash, number uint64) bool {
	// Check first that the block itself is known
	block := bc.GetBlock(hash, number)
	if block == nil {
		return false
	}
	return bc.HasState(block.StateRoot())
}

// encodeBlockNumber encodes a block number as big endian uint64
func encodeBlockNumber(number uint64) []byte {
	enc := make([]byte, 8)
	binary.BigEndian.PutUint64(enc, number)
	return enc
}

//GetDb return  db of blockchain using
func (bc *BlockChain) GetDb() storage.Database {
	return bc.db
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

func (bc *BlockChain) InsertChain(chain types.Blocks) (int, error) {
	n, events, logs, err := bc.insertChain(chain)
	bc.PostChainEvents(events, logs)
	return n, err
}

func (bc *BlockChain) insertChain(chain types.Blocks) (int, []interface{}, []*types.Log, error) {
	if len(chain) == 0 {
		return 0, nil, nil, nil
	}

	bc.wg.Add(1)
	defer bc.wg.Done()

	bc.mu.Lock()
	defer bc.mu.Unlock()

	// A queued approach to delivering events. This is generally
	// faster than direct delivery and requires much less mutex
	// acquiring.
	var (
		//stats         = insertStats{startTime: mclock.Now()}
		events        = make([]interface{}, 0, len(chain))
		lastCanon     *types.Block
		coalescedLogs []*types.Log
	)
	for i, block := range chain {
		//fmt.Println("########insertChain里的parent：%v", block.Header())
		// fmt.Println("########insertChain里的parent的number消息：%v", block.Header().Number)
		// fmt.Println("########insertChain里的parent的hash消息：%v", block.Hash().String())
		// fmt.Println("########insertChain里的parent的ParentHash消息：%v", block.ParentHash().String())
		// fmt.Println("########insertChain里的parent的Difficulty消息：%v", block.Difficulty())
		// fmt.Println("########insertChain里的parent的StateRoot消息：%v", block.StateRoot().String())

		// Write the block to the chain and get the status.
		//err: = bc.WriteBlockWithState(block, make([]*types.Receipt, 1), state) //receipt is empty just for test
		err := bc.WriteBlockWithoutState(block)
		if err != nil {
			return i, events, coalescedLogs, err
		}
		events = append(events, ChainEvent{block, block.Hash(), make([]*types.Log, 1)})
		lastCanon = block
	}
	//TODO：need to implement real TD
	if lastCanon.Difficulty().Cmp(bc.CurrentBlock().Difficulty()) > 0 {
		bc.insert(lastCanon)
	}

	// Append a single chain head event if we've progressed the chain
	if lastCanon != nil && bc.CurrentBlock().Hash() == lastCanon.Hash() {
		events = append(events, ChainHeadEvent{lastCanon})
	}
	return 0, events, coalescedLogs, nil
}

func (bc *BlockChain) WriteBlockWithoutState(block *types.Block) (err error) {
	bc.wg.Add(1)
	defer bc.wg.Done()

	rawdb.WriteBlock(bc.db, block)
	return nil
}

//SetValidator sets the validator which is used to validate incoming blocks.
func (bc *BlockChain) SetValidator(validator Validator) {
	bc.procmu.Lock()
	defer bc.procmu.Unlock()
	bc.validator = validator
}

// Validator returns the current validator.
func (bc *BlockChain) Validator() Validator {
	bc.procmu.RLock()
	defer bc.procmu.RUnlock()
	return bc.validator
}
