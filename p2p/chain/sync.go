package chain

import (
	"sync/atomic"
	"time"

	"github.com/invin/kkchain/core"
	"github.com/invin/kkchain/core/types"
	"github.com/jbenet/goprocess"
)

const (
	forceSyncCycle      = 3 * time.Second // Time interval to force syncs, even if few peers are available
	minDesiredPeerCount = 5               // Amount of peers desired to start syncing
)

const (
	Stopped = iota
	Started
)

// Syncer represents syncer for blockchain.
// It includes both fetching and downloading
type Syncer struct {
	status     int32
	proc       goprocess.Process
	ctrl       *core.Controller
	chain      *Chain
	blockchain *core.BlockChain
	downloader *Downloader
	fetcher    *Fetcher
}

// NewSyncer creates a new syncer object
func NewSyncer(chain *Chain) *Syncer {
	bc := chain.blockchain
	validator := func(header *types.Header) error {
		return bc.Engine().VerifyHeader(bc, header)
	}
	heighter := func() uint64 {
		return bc.CurrentBlock().NumberU64()
	}
	inserter := func(blocks types.Blocks) (int, error) {
		atomic.StoreUint32(&chain.acceptTxs, 1) // Mark initial sync done on any fetcher import
		return bc.InsertChain(blocks)
	}
	return &Syncer{
		status:     Stopped,
		chain:      chain,
		blockchain: chain.blockchain,
		downloader: NewDownloader(chain),
		fetcher:    NewFetcher(bc.GetBlockByHash, validator, chain.BroadcastBlock, heighter, inserter, chain.removePeer),
	}
}

// Start starts sync operation with remote peers
func (s *Syncer) Start() error {
	if !atomic.CompareAndSwapInt32(&s.status, Stopped, Started) {
		log.Warning("Already started")
		return errBusy
	}
	// Create goprocess with tear down
	s.proc = goprocess.WithTeardown(func() error {
		log.Info("Shutting down sync")
		return nil
	})

	loop := func(p goprocess.Process) {
		s.fetcher.Start()
		defer s.fetcher.Stop()
		defer s.downloader.Terminate()
		// Wait for different events to fire synchronization operations
		forceSync := time.NewTicker(forceSyncCycle)
		defer forceSync.Stop()

		for {
			select {
			case <-p.Closing():
				log.Info("Exiting loop ...")
				return
			case <-forceSync.C:
				// Force a sync even if not enough peers are present
				// TODO: with the best peer
				peers := s.chain.peers
				go s.synchronise(peers.BestPeer())
			}
		}
	}

	s.proc.Go(loop)

	return nil
}

// synchronise tries to synchronise with best peer
func (s *Syncer) synchronise(p *peer) {
	// Short circuit if no peers are available
	if p == nil {
		return
	}

	log.Info("start synchonise with ", p.ID)
	// Make sure the peer's TD is higher than our own
	currentBlock := s.blockchain.CurrentBlock()
	td := s.blockchain.GetTd(currentBlock.Hash(), currentBlock.NumberU64())

	pHead, pTd := p.Head()
	if pTd.Cmp(td) <= 0 {
		return
	}

	// Run the sync cycle, and disable fast sync if we've went past the pivot block
	if err := s.downloader.Synchronise(p.ID, pHead, pTd, FullSync); err != nil {
		log.Error("Failed to sync,error: ", err)
		return
	}

	// Otherwise try to sync with downloader
	atomic.StoreUint32(&s.chain.acceptTxs, 1) // Mark initial sync done
	if head := s.blockchain.CurrentBlock(); head.NumberU64() > 0 {
		go s.chain.BroadcastBlock(head, false)
	}

}

// Stop stops sync operation
func (s *Syncer) Stop() {
	if s.proc != nil {
		s.proc.Close()
		s.proc = nil
	}

	// FIXME:
	atomic.StoreInt32(&s.status, Stopped)
}

// NewBlock injects a new received block from remote peer
func (s *Syncer) NewBlock(id string, block *types.Block) error {
	// s.fetcher.Enqueue(id, block)
	// TODO: compare TD and triggle synchronize if necessary
	return nil
}

// DeliverHeaders injects a new batch of block headers received from a remote
// node into the local schedule.
func (s *Syncer) DeliverHeaders(id string, headers []*types.Header) (err error) {
	// TODO: filter from fetchers
	filter := len(headers) == 1
	if filter {
		// TODO: send the header to the fetcher just in case
		// headers = s.fetcher.FilterHeaders(p.id, headers, time.Now())
	}

	if len(headers) > 0 || !filter {
		return s.downloader.DeliverHeaders(id, headers)
	}

	return nil
}

// DeliverBlocks injects a new batch of blocks  received from a remote
// node into the local schedule.
func (s *Syncer) DeliverBlocks(id string, blocks []*types.Block) (err error) {
	// TODO: filter from fetchers
	return s.downloader.DeliverBlocks(id, blocks)
}
