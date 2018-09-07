package chain

import (
	"errors"
	"fmt"
	"math/big"
	"sync"

	"time"

	"github.com/deckarep/golang-set"
	"github.com/invin/kkchain/common"
	"github.com/invin/kkchain/core/types"
	"github.com/invin/kkchain/p2p"
	"github.com/sirupsen/logrus"
)

var (
	errClosed            = errors.New("peer set is closed")
	errAlreadyRegistered = errors.New("peer is already registered")
	errNotRegistered     = errors.New("peer is not registered")
)

const (
	maxKnownTxs    = 32768 // Maximum transactions hashes to keep in the known list (prevent DOS)
	maxKnownBlocks = 1024  // Maximum block hashes to keep in the known list (prevent DOS)

	// maxQueuedTxs is the maximum number of transaction lists to queue up before
	// dropping broadcasts. This is a sensitive number as a transaction list might
	// contain a single transaction, or thousands.
	maxQueuedTxs = 128

	// maxQueuedProps is the maximum number of block propagations to queue up before
	// dropping broadcasts. There's not much point in queueing stale blocks, so a few
	// that might cover uncles should be enough.
	maxQueuedProps = 4

	// maxQueuedAnns is the maximum number of block announcements to queue up before
	// dropping broadcasts. Similarly to block propagations, there's no point to queue
	// above some healthy uncle limit, so use that.
	maxQueuedAnns = 4

	handshakeTimeout = 5 * time.Second
)

type peer struct {
	ID          string
	conn        p2p.Conn
	head        common.Hash
	td          *big.Int
	lock        sync.RWMutex
	knownTxs    mapset.Set                // Set of transaction hashes known to be known by this peer
	knownBlocks mapset.Set                // Set of block hashes known to be known by this peer
	queuedTxs   chan []*types.Transaction // Queue of transactions to broadcast to the peer
	queuedProps chan *types.Block         // Queue of blocks to broadcast to the peer
	queuedAnns  chan []common.Hash        // Queue of blocks to announce to the peer
	term        chan struct{}             // Termination channel to stop the broadcaster
}

func NewPeer(conn p2p.Conn) *peer {
	return &peer{
		conn:        conn,
		ID:          fmt.Sprintf("%x", conn.RemotePeer().PublicKey[:8]),
		knownTxs:    mapset.NewSet(),
		knownBlocks: mapset.NewSet(),
		queuedTxs:   make(chan []*types.Transaction, maxQueuedTxs),
		queuedProps: make(chan *types.Block, maxQueuedProps),
		queuedAnns:  make(chan []common.Hash, maxQueuedAnns),
		term:        make(chan struct{}),
	}
}

// push all broadcast msg to remote peer
func (p *peer) broadcast() {
	for {
		select {
		case txs := <-p.queuedTxs:
			if err := p.SendTransactions(txs); err != nil {
				logrus.WithFields(logrus.Fields{
					"tx_count": len(txs),
					"error":    err,
				}).Error("failed to broadcast txs")
				return
			}
		case blockhashes := <-p.queuedAnns:
			if err := p.SendNewBlockHashes(blockhashes); err != nil {
				logrus.WithFields(logrus.Fields{
					"hash_count": len(blockhashes),
					"error":      err,
				}).Error("failed to broadcast block hashes")
				return
			}
		case block := <-p.queuedProps:
			if err := p.SendNewBlock(block); err != nil {
				logrus.WithFields(logrus.Fields{
					"block_num": block.NumberU64(),
					"error":     err,
				}).Error("failed to broadcast new block")
				return
			}
		}
	}
}

func (p *peer) close() {
	close(p.term)
}

func (p *peer) Head() (hash common.Hash, td *big.Int) {
	p.lock.Lock()
	defer p.lock.Unlock()
	copy(hash[:], p.head[:])
	return hash, new(big.Int).Set(p.td)
}

func (p *peer) SetHead(hash common.Hash, td *big.Int) {
	p.lock.Lock()
	defer p.lock.Unlock()
	copy(p.head[:], hash[:])
	p.td.Set(td)
}

func (p *peer) MarkBlock(hash common.Hash) {
	for p.knownBlocks.Cardinality() >= maxKnownBlocks {
		p.knownBlocks.Pop()
	}
	p.knownBlocks.Add(hash)
}

func (p *peer) MarkTransactions(hash common.Hash) {
	for p.knownTxs.Cardinality() >= maxKnownTxs {
		p.knownTxs.Pop()
	}
	p.knownTxs.Add(hash)
}

func (p *peer) SendTransactions(txs []*types.Transaction) error {
	for _, tx := range txs {
		p.knownTxs.Add(tx.Hash())
	}
	return p.conn.SendChainMsg(int32(Message_TRANSACTIONS), txs)
}

func (p *peer) SendNewBlockHashes(hashes []common.Hash) error {
	for _, hash := range hashes {
		p.knownBlocks.Add(hash)
	}
	return p.conn.SendChainMsg(int32(Message_NEW_BLOCK_HASHS), hashes)
}

func (p *peer) SendNewBlock(block *types.Block) error {
	p.knownBlocks.Add(block.Hash())
	return p.conn.SendChainMsg(int32(Message_NEW_BLOCK), block)
}

type PeerSet struct {
	peers  map[string]*peer
	lock   sync.RWMutex
	closed bool
}

func NewPeerSet() *PeerSet {
	return &PeerSet{peers: make(map[string]*peer)}
}

func (ps *PeerSet) Register(p *peer) error {
	ps.lock.Lock()
	defer ps.lock.Unlock()
	if ps.closed {
		return errClosed
	}
	if _, ok := ps.peers[p.ID]; ok {
		return errAlreadyRegistered
	}
	ps.peers[p.ID] = p
	go p.broadcast()
	return nil
}

func (ps *PeerSet) Unregister(id string) error {
	ps.lock.Lock()
	defer ps.lock.Unlock()
	if ps.closed {
		return errClosed
	}

	p, ok := ps.peers[id]
	if !ok {
		return errNotRegistered
	}
	delete(ps.peers, id)
	p.close()
	return nil
}

func (ps *PeerSet) Peer(id string) *peer {
	ps.lock.Lock()
	defer ps.lock.Unlock()
	return ps.peers[id]
}

func (ps *PeerSet) Len() int {
	ps.lock.Lock()
	defer ps.lock.Unlock()
	return len(ps.peers)
}

func (ps *PeerSet) PeersWithoutBlock(hash common.Hash) []*peer {
	ps.lock.Lock()
	defer ps.lock.Unlock()
	result := make([]*peer, 0, len(ps.peers))
	for _, p := range ps.peers {
		if !p.knownBlocks.Contains(hash) {
			result = append(result, p)
		}
	}
	return result
}

func (ps *PeerSet) PeersWithoutTx(hash common.Hash) []*peer {
	ps.lock.Lock()
	defer ps.lock.Unlock()
	result := make([]*peer, 0, len(ps.peers))
	for _, p := range ps.peers {
		if !p.knownTxs.Contains(hash) {
			result = append(result, p)
		}
	}
	return result
}

// find the best difficult peer
func (ps *PeerSet) BestPeer() *peer {
	ps.lock.Lock()
	defer ps.lock.Unlock()
	var (
		bestPeer *peer
		bestTD   *big.Int
	)
	for _, p := range ps.peers {
		if _, td := p.Head(); bestPeer == nil || td.Cmp(bestTD) > 0 {
			bestPeer, bestTD = p, td
		}
	}
	return bestPeer
}

// TODO: should close all peer conn ?
func (ps *PeerSet) Close() {
	ps.lock.Lock()
	defer ps.lock.Unlock()
	for _, p := range ps.peers {
		p.close()
	}
	ps.closed = true
}
