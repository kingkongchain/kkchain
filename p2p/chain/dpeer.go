package chain

import (
	"math/big"

	"github.com/invin/kkchain/common"
)

// Peer defines interface for requesting blocks and headers from remote peer
type Peer interface {
	ID() string
	Head() (hash common.Hash, td *big.Int)
	RequestHeadersByHash(origin common.Hash, amount int, skip int, reverse bool) error
	RequestHeadersByNumber(origin uint64, amount int, skip int, reverse bool) error
	RequestBlocksByNumber(origin uint64, amount int) error 
}

// DPeer represent a peer for downloading. currently, It is a wrapper for peer
type DPeer struct {
	p *peer
}

// NewDPeer represents a peer for downloading
func NewDPeer(p *peer) *DPeer {
	return &DPeer{
		p: p,
	}
}

// ID returns the identification of the peer
func (dp *DPeer) ID() string {
	return dp.p.ID
}

// Head returns the current head of the peer
func (dp *DPeer) Head() (hash common.Hash, td *big.Int) {
	return dp.p.Head()
}

// RequestHeadersByHash fetches a batch of blocks' headers corresponding to the
// specified header query, based on the hash of an origin block.
func (dp *DPeer) RequestHeadersByHash(origin common.Hash, amount int, skip int, reverse bool) error {
	return dp.p.requestHeadersByHash(origin, amount, skip, reverse)
}

// RequestHeadersByNumber fetches a batch of blocks' headers corresponding to the
// specified header query, based on the number of an origin block.
func (dp *DPeer) RequestHeadersByNumber(origin uint64, amount int, skip int, reverse bool) error {
	return dp.p.requestHeadersByNumber(origin, amount, skip, reverse)
}

// RequestBlocksByNumber fetches a batch of blocks corresponding to the
// specified range
func (dp *DPeer) RequestBlocksByNumber(origin uint64, amount int) error {
	return dp.p.requestBlocksByNumber(origin, amount)
}

// DPeerSetIntf is the interface for peer set
type DPeerSetIntf interface {
	Register(p Peer) error
	UnRegister(id string) error
	Peer(id string) Peer
}

// DPeerSet is a thin wrapper for original peerset, through which we can do testing easily
type DPeerSet struct {
	ps *PeerSet
}

// NewDPeerSet creates a download peerset
func NewDPeerSet(ps *PeerSet) *DPeerSet {
	return &DPeerSet{
		ps: ps,
	}
}

// Register registers peer 
func (s *DPeerSet) Register(p Peer) error {
	panic("not supported yet")
	return nil
}

// UnRegister unregisters peer specified by id 
func (s *DPeerSet) UnRegister(id string) error {
	panic("not supported yet")
	return nil
}

// Peer returns the peer with specified id
func (s *DPeerSet) Peer(id string) Peer {
	p := s.ps.Peer(id)
	return NewDPeer(p)
}







