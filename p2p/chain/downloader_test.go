package chain

import (
	"testing"
	"time"
	"math/big"
	"sync"

	"github.com/invin/kkchain/core"
	"github.com/invin/kkchain/common"
	"github.com/invin/kkchain/core/types"
	"github.com/invin/kkchain/storage/memdb"
	"github.com/invin/kkchain/consensus/pow"
	"github.com/invin/kkchain/storage"
	"github.com/invin/kkchain/crypto"
)

var (
	testKey, _  = crypto.HexToECDSA("b71c71a67e1177ad4e901695e1b4b9ee17ae16c6668d313eac2f96dbcda3f291")
	testAddress = crypto.PubkeyToAddress(testKey.PublicKey)
)

// Reduce some of the parameters to make the tester faster.
func init() {
	// MaxForkAncestry = uint64(10000)
	// blockCacheItems = 1024
	// fsHeaderContCheck = 500 * time.Millisecond
}

type downloadTester struct {
	chain *Chain
	downloader *Downloader

	genesis *types.Block	// Genesis block used by the tester and peers
	stateDb storage.Database // Database used by the tester for syncing from peers
	peerDb  storage.Database // Database of the peers containing all data

	ownHashes   []common.Hash                  // Hash chain belonging to the tester
	ownHeaders  map[common.Hash]*types.Header  // Headers belonging to the tester
	ownBlocks   map[common.Hash]*types.Block   // Blocks belonging to the tester
	ownReceipts map[common.Hash]types.Receipts // Receipts belonging to the tester
	ownChainTd  map[common.Hash]*big.Int       // Total difficulties of the blocks in the local chain

	peerHashes   map[string][]common.Hash                  // Hash chain belonging to different test peers
	peerHeaders  map[string]map[common.Hash]*types.Header  // Headers belonging to different test peers
	peerBlocks   map[string]map[common.Hash]*types.Block   // Blocks belonging to different test peers
	peerReceipts map[string]map[common.Hash]types.Receipts // Receipts belonging to different test peers
	peerChainTds map[string]map[common.Hash]*big.Int       // Total difficulties of the blocks in the peer chains

	peerMissingStates map[string]map[common.Hash]bool // State entries that fast sync should not return

	lock sync.RWMutex
}

// newTester creates a new downloader test mocker.
func newTester() *downloadTester {
	testdb := memdb.New()
	genesis := core.GenesisBlockForTesting(testdb, testAddress, big.NewInt(1000000000))

	tester := &downloadTester{
		genesis:           genesis,
		peerDb:            testdb,
		ownHashes:         []common.Hash{genesis.Hash()},
		ownHeaders:        map[common.Hash]*types.Header{genesis.Hash(): genesis.Header()},
		ownBlocks:         map[common.Hash]*types.Block{genesis.Hash(): genesis},
		ownReceipts:       map[common.Hash]types.Receipts{genesis.Hash(): nil},
		ownChainTd:        map[common.Hash]*big.Int{genesis.Hash(): genesis.Difficulty()},
		peerHashes:        make(map[string][]common.Hash),
		peerHeaders:       make(map[string]map[common.Hash]*types.Header),
		peerBlocks:        make(map[string]map[common.Hash]*types.Block),
		peerReceipts:      make(map[string]map[common.Hash]types.Receipts),
		peerChainTds:      make(map[string]map[common.Hash]*big.Int),
		peerMissingStates: make(map[string]map[common.Hash]bool),
	}
	tester.stateDb = memdb.New() 
	tester.stateDb.Put(genesis.StateRoot().Bytes(), []byte{0x00})

	tester.downloader = New(FullSync, tester.stateDb, new(event.TypeMux), tester, nil, tester.dropPeer)

	return tester
}

func makeChain() {
	db := memdb.New()
	engine := pow.NewFakeDelayer(15 * time.Second)
	chain, _ := core.NewBlockChain(db, engine)
	

}
func TestDownloader(t *testing.T) {
	a := "2323"
	log.Info("test info", a)
}