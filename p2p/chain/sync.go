package chain 

import (
	"time"
	"sync/atomic"

	"github.com/invin/kkchain/core"
	log "github.com/sirupsen/logrus"
	"github.com/jbenet/goprocess"
)

const (
	forceSyncCycle		= 3 * time.Second 	// Time interval to force syncs, even if few peers are available
	minDesiredPeerCount = 5				   	// Amount of peers desired to start syncing
)

const (
	Inited = iota
	Started
	Stopped
)

// Syncer represents syncer for blockchain.
// It includes both fetching and downloading
type Syncer struct {
	status uint32
	proc goprocess.Process
	ctrl *core.Controller
	chain *Chain
	blockchain *core.BlockChain
}

// New creates a new syncer object
func NewSyncer(chain *Chain) *Syncer {
	return &Syncer{
		status: Inited,
		chain: chain,
		blockchain: chain.blockchain
	}
}

// Start starts sync operation with remote peers
func (s *Syncer) Start() goprocess.Process {
	if atomic.LoadUint32(&s.status) == Started {
		log.Warn("Already started")
	}

	// Create goprocess with tear down
	s.proc = goprocess.WithTeardown(func () error {
		log.Info("Shutting down sync")
		return nil
	})

	loop := func (p goprocess.Process) {
		// Wait for different events to fire synchronization operations
		forceSync := time.NewTicker(forceSyncCycle)
		defer forceSync.Stop()

		for {
			select {
			case <- p.Closing():
				log.Info("Exiting loop ...")
				return
			case <- forceSync.C:
				// Force a sync even if not enough peers are present
				// TODO: with the best peer
				peers := s.chain.peers
				go s.synchronise(peers.BestPeer())
			}
		}
	}

	s.proc.Go(loop)

	return s.proc
}

// synchronise tries to synchronise with best peer 
func (s *Syncer) synchronise(peer *) {
	log.Info("start synchonise with ", peer)
	// Make sure the peer's TD is higher than our own
	currentBlock := s.blockchain.CurrentBlock()
	td := s.blockchain.GetTd(currentBlock.Hash(), currentBlock.NumberU64())

	pHead, pTd := peer.Head()
	if pTd.Cmp(td) <= 0 {
		return
	}

	// Run the sync cycle, and disable fast sync if we've went past the pivot block
	if err := s.downloader.Synchronise(peer.id, pHead, pTd, downloader.FullSync); err != nil {
		log.Error(err)
		return
	}

	// Otherwise try to sync with downloader
	atomic.StoreUint32(&chain.acceptTxs, 1) // Mark initial sync done
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

	atomic.StoreUint32(&s.status, Stopped)
}

