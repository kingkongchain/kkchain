package chain

import (
	"context"
	"fmt"

	"bytes"

	"github.com/invin/kkchain/p2p"
	"github.com/pkg/errors"
)

// chainHandler specifies the signature of functions that handle DHT messages.
type chainHandler func(context.Context, p2p.ID, *Message) (*Message, error)

func (c *Chain) handlerForMsgType(t Message_Type) chainHandler {
	switch t {
	case Message_CHAIN_STATUS:
		return c.handleChainStatus
	case Message_GET_BLOCK_BODIES:
		return c.handleGetBlockBodies
	case Message_BLOCKS_BODIES:
		return c.handleBlockBodies
	case Message_GET_BLOCK_HEADERS:
		return c.handleGetBlockHeaders
	case Message_BLOCK_HEADERS:
		return c.handleBlockHeaders
	case Message_TRANSACTIONS:
		return c.handleTransactions
	case Message_GET_RECEIPTS:
		return c.handleGetReceipts
	case Message_RECEIPTS:
		return c.handleReceipts
	case Message_NEW_BLOCK_HASHS:
		return c.handleNewBlockHashs
	case Message_NEW_BLOCK:
		return c.handleNewBlock
	default:
		return nil
	}
}

func (c *Chain) handleChainStatus(ctx context.Context, p p2p.ID, pmes *Message) (_ *Message, err error) {
	fmt.Println("接收到chain status消息：%v", pmes.String())
	localChainID := c.blockchain.ChainID()
	currentBlock := c.blockchain.CurrentBlock()
	localChainTD := currentBlock.DeprecatedTd().Bytes()
	localChainCurrentBlockHash := currentBlock.Hash().Bytes()
	localChainCurrentBlockNum := currentBlock.NumberU64()
	localChainGenesisBlock := c.blockchain.GenesisBlock().Hash().Bytes()
	remoteChainStatus := pmes.ChainStatusMsg
	if remoteChainStatus == nil {
		return nil, errors.New("empty msg content")
	}
	if localChainID != remoteChainStatus.ChainID ||
		!bytes.Equal(localChainGenesisBlock, remoteChainStatus.GenesisBlockHash) {
		return nil, errors.Errorf("remote node %s is in different chain", p.String())
	}
	if bytes.Equal(localChainTD, remoteChainStatus.Td) &&
		bytes.Equal(localChainCurrentBlockHash, remoteChainStatus.CurrentBlockHash) {
		return nil, nil
	}

	var (
		startNum  = uint64(0)
		endNum    = uint64(0)
		skipNum   = []uint64{}
		direction = false
		resp      = new(Message)
	)
	remoteCurrentBlockNum := remoteChainStatus.CurrentBlockNum

	// local node falls behind, should send get block headers msg
	if localChainCurrentBlockNum < remoteCurrentBlockNum {
		startNum = localChainCurrentBlockNum + 1
		endNum = remoteCurrentBlockNum

		// TODO: checkout local receive broadcast block that between startNum and endNum
		direction = false
		getBlockHeadersMsg := &GetBlockHeadersMsg{
			StartNum:  startNum,
			EndNum:    endNum,
			SkipNum:   skipNum,
			Direction: direction,
		}
		resp = NewMessage(Message_GET_BLOCK_HEADERS, getBlockHeadersMsg)
	}

	// remote node falls behind, should send new block hashes msg
	if localChainCurrentBlockNum > remoteCurrentBlockNum {
		// TODO: iterator prev from current block , find block hash until the remote block num
		newBlockHashesMsg := &DataMsg{
			Data: [][]byte{},
		}
		resp = NewMessage(Message_NEW_BLOCK_HASHS, newBlockHashesMsg)
	}

	return resp, nil
}

func (c *Chain) handleGetBlockBodies(ctx context.Context, p p2p.ID, pmes *Message) (_ *Message, err error) {
	// Check result and return corresponding code
	var resp *Message

	// TODO:
	return resp, nil
}

func (c *Chain) handleGetBlockHeaders(ctx context.Context, p p2p.ID, pmes *Message) (_ *Message, err error) {
	// Check result and return corresponding code
	var resp *Message

	// TODO:
	return resp, nil
}

func (c *Chain) handleBlockBodies(ctx context.Context, p p2p.ID, pmes *Message) (_ *Message, err error) {
	// Check result and return corresponding code
	var resp *Message

	// TODO:
	return resp, nil
}

func (c *Chain) handleBlockHeaders(ctx context.Context, p p2p.ID, pmes *Message) (_ *Message, err error) {
	// Check result and return corresponding code
	var resp *Message

	// TODO:
	return resp, nil
}

func (c *Chain) handleTransactions(ctx context.Context, p p2p.ID, pmes *Message) (_ *Message, err error) {
	// Check result and return corresponding code
	var resp *Message

	// TODO:
	return resp, nil
}

func (c *Chain) handleGetReceipts(ctx context.Context, p p2p.ID, pmes *Message) (_ *Message, err error) {
	// Check result and return corresponding code
	var resp *Message

	// TODO:
	return resp, nil
}

func (c *Chain) handleReceipts(ctx context.Context, p p2p.ID, pmes *Message) (_ *Message, err error) {
	// Check result and return corresponding code
	var resp *Message

	// TODO:
	return resp, nil
}

func (c *Chain) handleNewBlockHashs(ctx context.Context, p p2p.ID, pmes *Message) (_ *Message, err error) {
	// Check result and return corresponding code
	var resp *Message

	// TODO:
	return resp, nil
}

func (c *Chain) handleNewBlock(ctx context.Context, p p2p.ID, pmes *Message) (_ *Message, err error) {
	// Check result and return corresponding code
	var resp *Message

	// TODO:
	return resp, nil
}
