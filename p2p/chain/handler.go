package chain

import (
	"context"
	"encoding/hex"
	"fmt"

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

	// TODO: store local chain in context ?

	remoteChainStatus := pmes.ChainStatusMsg
	if ctx.Value("localChainID") != remoteChainStatus.ChainID ||
		ctx.Value("localChainGenesisBlock") != hex.EncodeToString(remoteChainStatus.GenesisBlockHash) {
		return nil, errors.Errorf("remote node %s is in different chain", p.String())
	}
	if ctx.Value("localChainTD") == hex.EncodeToString(remoteChainStatus.Td) &&
		ctx.Value("localChainCurrentBlockHash") == hex.EncodeToString(remoteChainStatus.CurrentBlockHash) {
		return nil, nil
	}

	var (
		startNum  = uint64(0)
		endNum    = uint64(0)
		skipNum   = []uint64{}
		direction = false
		resp      = new(Message)
	)
	localCurrentBlockNum := ctx.Value("localChainCurrentBlockNum").(uint64)
	remoteCurrentBlockNum := remoteChainStatus.CurrentBlockNum

	// local node falls behind, should send get block headers msg
	if localCurrentBlockNum < remoteCurrentBlockNum {
		startNum = localCurrentBlockNum + 1
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
	if localCurrentBlockNum > remoteCurrentBlockNum {
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
