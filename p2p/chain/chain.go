package chain

import (
	"context"

	"math/big"

	"github.com/gogo/protobuf/proto"
	"github.com/invin/kkchain/blockchain"
	"github.com/invin/kkchain/consensus"
	"github.com/invin/kkchain/core"
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
	ps *blockchain.PeerSet

	blockchain *core.BlockChain
}

// New creates a new Chain object
func New(host p2p.Host, bc *core.BlockChain) *Chain {
	c := &Chain{
		host:       host,
		blockchain: bc,
		ps:         blockchain.NewPeerSet(),
	}

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
	peer := consensus.NewPeer(conn)
	c.ps.Register(peer)

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
