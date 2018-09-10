package chain

import (
	"context"
	"math"

	"math/big"

	"github.com/gogo/protobuf/proto"
	"github.com/invin/kkchain/common"
	"github.com/invin/kkchain/core"
	"github.com/invin/kkchain/core/types"
	"github.com/invin/kkchain/event"
	"github.com/invin/kkchain/p2p"
	"github.com/op/go-logging"
)

const (
	protocolChain = "/kkchain/p2p/chain/1.0.0"
)

var log = logging.MustGetLogger("p2p/chain")

// Chain implements protocol for chain related messages
type Chain struct {
	// self
	host p2p.Host

	// use to manager broadcasting for remote
	peers    *PeerSet
	maxPeers int

	blockchain *core.BlockChain

	txPool *core.TxPool

	mineBlockSub event.Subscription

	newMinedBlockCh chan core.NewMinedBlockEvent
}

// New creates a new Chain object
func New(host p2p.Host, bc *core.BlockChain) *Chain {
	c := &Chain{
		host:       host,
		blockchain: bc,
		peers:      NewPeerSet(),
	}
	c.newMinedBlockCh = make(chan core.NewMinedBlockEvent, 256)

	if err := host.SetMessageHandler(protocolChain, c.handleMessage); err != nil {
		panic(err)
	}

	host.Register(c)

	return c
}

func (c *Chain) GetBlockChain() *core.BlockChain {
	return c.blockchain
}

// handleMessage handles messages within the stream
func (c *Chain) handleMessage(conn p2p.Conn, msg proto.Message) {
	// Ignore maxPeers if this is a trusted peer
	if c.peers.Len() >= c.maxPeers {
		return
	}
	// Register the peer locally
	//c.Connected(conn)
	peer := NewPeer(conn)
	if c.peers.peers[peer.ID] == nil {
		c.peers.Register(peer)
		log.Infof("a conn is notified,remote ID: %s", conn.RemotePeer())
	}

	//defer c.removePeer(p.id)

	// check message type
	switch message := msg.(type) {
	case *Message:
		c.doHandleMessage(conn, message)
	default:
		conn.Close()
		log.Errorf("unexpected message: %v", msg)
	}
}

// doHandleMessage handles messsage
func (c *Chain) doHandleMessage(conn p2p.Conn, msg *Message) {
	// get handler
	handler := c.handlerForMsgType(msg.GetType())
	if handler == nil {
		conn.Close()
		log.Errorf("unknown message type: %v", msg.GetType())
		return
	}

	// dispatch handler
	// TODO: get context and peer id
	ctx := context.Background()
	pid := conn.RemotePeer()

	rpmes, err := handler(ctx, pid, msg)

	// if nil response, return it before serializing
	if rpmes == nil {
		log.Warning("got back nil response from request")
		if err != nil {
			log.Error("failed to make response for request,error: %v", err)
		}
		return
	}

	// send out response msg
	if err = conn.WriteMessage(rpmes, protocolChain); err != nil {
		conn.Close()
		log.Errorf("send response error: %s", err)
		return
	}
}

func (c *Chain) Connected(conn p2p.Conn) {

	// create a peer with this conn, and register
	peer := NewPeer(conn)
	c.peers.Register(peer)

	log.Infof("a conn is notified,remote ID: %s", conn.RemotePeer())
	currentBlock := c.blockchain.CurrentBlock()
	if currentBlock == nil {
		log.Warning("local chain current block is nil")
		return
	}
	chainID := c.blockchain.ChainID()

	// TODO: retrive local current block td
	td := new(big.Int).Bytes()
	currentBlockHash := currentBlock.Hash().Bytes()
	currentBlockNum := currentBlock.NumberU64()
	genesisBlockHash := c.blockchain.GenesisBlock().Hash().Bytes()

	chainMsg := &ChainStatusMsg{
		ChainID:          chainID,
		Td:               td,
		CurrentBlockHash: currentBlockHash,
		CurrentBlockNum:  currentBlockNum,
		GenesisBlockHash: genesisBlockHash,
	}
	chainStatueMsg := NewMessage(Message_CHAIN_STATUS, chainMsg)
	err := conn.WriteMessage(chainStatueMsg, protocolChain)
	if err != nil {
		log.Errorf("failed to send chain status msg to %s", conn.RemotePeer())
	}
}

func (c *Chain) Disconnected(conn p2p.Conn) {
	log.Infof("a disconn is notified,remote ID: %s", conn.RemotePeer())
}

func (c *Chain) removePeer(id string) {
	// Short circuit if the peer was already removed
	peer := c.peers.Peer(id)
	if peer == nil {
		return
	}
	log.Debug("Removing KKChain peer", "peer", id)

	// Unregister the peer from the downloader and Ethereum peer set
	// pm.downloader.UnregisterPeer(id)
	if err := c.peers.Unregister(id); err != nil {
		log.Error("Peer removal failed", "peer", id, "err", err)
	}
	// Hard disconnect at the networking layer
	// if peer != nil {
	// 	peer.Peer.Disconnect(p2p.DiscUselessPeer)
	// }
}

func (c *Chain) Start(maxPeers int) {
	c.maxPeers = maxPeers

	// broadcast transactions
	// pm.txsCh = make(chan core.NewTxsEvent, txChanSize)
	// pm.txsSub = c.txpool.SubscribeNewTxsEvent(pm.txsCh)
	// go pm.txBroadcastLoop()

	// broadcast mined blocks
	c.mineBlockSub = c.blockchain.SubscribeNewMinedBlockEvent(c.newMinedBlockCh)
	log.Info("****SubscribeNewMinedBlockEvent ")
	go c.minedBroadcastLoop()

	// start sync handlers
	// go pm.syncer()
	// go pm.txsyncLoop()
}

func (c *Chain) Stop() {
	log.Info("Stopping KKChain ")

	//pm.txsSub.Unsubscribe()        // quits txBroadcastLoop
	c.mineBlockSub.Unsubscribe() // quits blockBroadcastLoop
	c.peers.Close()

	log.Info("KKChain stopped")
}

// Mined broadcast loop
func (c *Chain) minedBroadcastLoop() {
	for {
		log.Info("*******Enter into minedBroadcastLoop")
		select {
		case newMinedBlockCh := <-c.newMinedBlockCh:
			log.Info("*******receive newMinedBlockEvent:blockNum:", newMinedBlockCh.Block.NumberU64(), ",prepare execute BroadcastBlock")
			c.BroadcastBlock(newMinedBlockCh.Block, true)  // First propagate block to peers
			c.BroadcastBlock(newMinedBlockCh.Block, false) // Only then announce to the rest
		}
	}
}

// will only announce it's availability (depending what's requested).
func (c *Chain) BroadcastBlock(block *types.Block, propagate bool) {
	log.Info("*******Enter into BroadcastBlock")
	hash := block.Hash()
	peers := c.peers.PeersWithoutBlock(hash)

	// If propagation is requested, send to a subset of the peer
	if propagate {
		// Calculate the TD of the block (it's not imported yet, so block.Td is not valid)
		// var td *big.Int
		// if parent := c.blockchain.GetBlock(block.ParentHash(), block.NumberU64()-1); parent != nil {
		// 	td = new(big.Int).Add(block.Difficulty(), c.blockchain.GetTd(block.ParentHash(), block.NumberU64()-1))
		// } else {
		// 	log.Error("Propagating dangling block", "number", block.Number(), "hash", hash)
		// 	return
		// }
		// Send the block to a subset of our peers
		transfer := peers[:int(math.Sqrt(float64(len(peers))))]
		for _, peer := range transfer {
			peer.SendNewBlock(block)
		}
		log.Info("*****Send block Suceess.Propagated block", "hash", hash, "recipients", len(transfer))
		return
	}
	// Otherwise if the block is indeed in out own chain, announce it
	if c.blockchain.HasBlock(hash, block.NumberU64()) {
		for _, peer := range peers {
			hash := []common.Hash{block.Hash()}
			peer.SendNewBlockHashes(hash)
		}
		log.Info("*****Send block Suceess.Announced block", "hash", hash, "recipients", len(peers))
	}
}
