package chain

import (
	"context"
	"fmt"

	"bytes"

	"encoding/json"

	"encoding/hex"

	"math/big"

	"github.com/invin/kkchain/common"
	"github.com/invin/kkchain/core/types"
	"github.com/invin/kkchain/p2p"
	"github.com/pkg/errors"
)

var (
	errEmptyMsgContent = errors.New("empty msg content")
	softResponseLimit = 2 * 1024 * 1024 // Target maximum size of returned blocks, headers or node data.
	estHeaderJsonSize  = 500             // Approximate size of an RLP encoded block header
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
	case Message_GET_BLOCKS:
		return c.handleGetBlocks
	case Message_BLOCKS:
		return c.handleBlocks
	default:
		return nil
	}
}

// only retrive chain status
func (c *Chain) handleChainStatus(ctx context.Context, p p2p.ID, pmes *Message) (_ *Message, err error) {
	fmt.Println("接收到chain status消息：%v", pmes.String())

	peerID := hex.EncodeToString(p.PublicKey)

	localChainID := c.blockchain.ChainID()
	currentBlock := c.blockchain.CurrentBlock()

	localChainTD := currentBlock.Td
	if localChainTD == nil {
		localChainTD = new(big.Int)
	}

	localChainGenesisBlock := c.blockchain.GenesisBlock().Hash().Bytes()
	remoteChainStatus := pmes.ChainStatusMsg
	if remoteChainStatus == nil {
		c.peers.Unregister(peerID)
		return nil, errEmptyMsgContent
	}
	if localChainID != remoteChainStatus.ChainID ||
		!bytes.Equal(localChainGenesisBlock, remoteChainStatus.GenesisBlockHash) {
		c.peers.Unregister(peerID)
		return nil, errors.Errorf("remote node %s is in different chain", p.String())
	}

	// set head and td for this peer
	remoteHeadHash := common.BytesToHash(remoteChainStatus.CurrentBlockHash)
	remoteTD := new(big.Int).SetBytes(remoteChainStatus.Td)
	c.peers.Peer(peerID).SetHead(remoteHeadHash, remoteTD)

	return nil, nil
}

func (c *Chain) handleGetBlockBodies(ctx context.Context, p p2p.ID, pmes *Message) (_ *Message, err error) {
	var resp *Message
	msg := pmes.DataMsg
	if msg == nil {
		return nil, errEmptyMsgContent
	}

	// collect bodies
	bodies := [][]byte{}
	for _, hbytes := range msg.Data {
		hash := common.Hash{}
		err = json.Unmarshal(hbytes, &hash)
		if err != nil {
			log.Error("failed to unmarshal bytes(%s) to common hash,error: %v", hex.EncodeToString(hbytes), err)
			continue
		}

		block := c.blockchain.GetBlockByHash(hash)
		if block == nil {
			log.Error("failed to get block %s from local,error: %v", hash.String(), err)
			continue
		}
		body := block.Body()

		bbytes, err := json.Marshal(body)
		if err != nil {
			log.Error("failed to marshal block body %d to bytes,error: %v", block.NumberU64(), err)
			continue
		}
		bodies = append(bodies, bbytes)

		blockBodiesMsg := DataMsg{
			Data: bodies,
		}
		resp = NewMessage(Message_BLOCKS_BODIES, blockBodiesMsg)
	}

	return resp, nil
}

func (c *Chain) handleGetBlockHeaders(ctx context.Context, p p2p.ID, pmes *Message) (_ *Message, err error) {
	var resp *Message
	msg := pmes.GetBlockHeadersMsg
	if msg == nil {
		return nil, errEmptyMsgContent
	}

	hashMode := len(msg.StartHash) == common.types.HashLength
	first := true
	maxNonCanonical := uint64(100)

	// Gather headers until the fetch or network limits is reached
	var (
		bytes   common.StorageSize
		headers []*types.Header
		unknown bool
	)

	originNum := msg.StartNum
	originHash := msg.StartHash

	for !unknown && len(headers) < int(msg.Amount) && bytes < softResponseLimit && len(headers) < MaxHeaderFetch {
		// Retrieve the next header satisfying the query
		var origin *types.Header
		if hashMode {
			if first {
				first = false
				origin = c.blockchain.GetHeaderByHash(msg.StartHash)
				if origin != nil {
					originNum = origin.Number.Uint64()
				}
			} else {
				origin = c.blockchain.GetHeader(originHash, originNum)
			}
		} else {
			origin = c.blockchain.GetHeaderByNumber(originNum)
		}
		if origin == nil {
			break
		}
		headers = append(headers, origin)
		bytes += estHeaderJsonSize

		// Advance to the next header of the query
		switch {
		case hashMode && msg.Reverse:
			// Hash based traversal towards the genesis block
			ancestor := msg.Skip + 1
			if ancestor == 0 {
				unknown = true
			} else {
				originHash, originNum = c.blockchain.GetAncestor(originHash, originNum, ancestor, &maxNonCanonical)
				unknown = (originHash == common.Hash{})
			}
		case hashMode && !msg.Reverse:
			// Hash based traversal towards the leaf block
			var (
				current = origin.Number.Uint64()
				next    = current + msg.Skip + 1
			)
			if next <= current {
				infos, _ := json.MarshalIndent(p.Peer.Info(), "", "  ")
				log.Warn("GetBlockHeaders skip overflow attack", "current", current, "skip", msg.Skip, "next", next, "attacker", infos)
				unknown = true
			} else {
				if header := c.blockchain.GetHeaderByNumber(next); header != nil {
					nextHash := header.Hash()
					expOldHash, _ := c.blockchain.GetAncestor(nextHash, next, msg.Skip+1, &maxNonCanonical)
					if expOldHash == originHash {
						originHash, originNum = nextHash, next
					} else {
						unknown = true
					}
				} else {
					unknown = true
				}
			}
		case msg.Reverse:
			// Number based traversal towards the genesis block
			if originNum >= msg.Skip+1 {
				originNum -= msg.Skip + 1
			} else {
				unknown = true
			}

		case !msg.Reverse:
			// Number based traversal towards the leaf block
			originNum += msg.Skip + 1
		}
	}

	// append headers from local blockchain
	headerBytes := [][]byte{}
	for _, header := range headers {
		hbytes, err := json.Marshal(header)
		if err != nil {
			log.Error("failed to marshal block header %d to bytes", i)
			continue // FIXME: pass through?
		}
		headerBytes = append(headerBytes, hbytes)	
	}

	// response block headers msg
	blockHeadersMsg := &DataMsg{
		Data: headerBytes,
	}
	resp = NewMessage(Message_BLOCK_HEADERS, blockHeadersMsg)
	return resp, nil
}

func (c *Chain) handleBlockBodies(ctx context.Context, p p2p.ID, pmes *Message) (_ *Message, err error) {
	msg := pmes.DataMsg
	if msg == nil {
		return nil, errEmptyMsgContent
	}

	for _, bbytes := range msg.Data {
		body := new(types.Body)
		err = json.Unmarshal(bbytes, body)
		if err != nil {
			log.Error("failed to unmarshal bytes to block body,error: %v", err)
			continue
		}
		log.Info("receive block body %v", body.Transactions)

		// TODO: execute received body tx to local chain

	}

	// no resp for block bodies
	return nil, nil
}

func (c *Chain) handleBlockHeaders(ctx context.Context, p p2p.ID, pmes *Message) (_ *Message, err error) {
	msg := pmes.DataMsg
	if msg == nil {
		return nil, errEmptyMsgContent
	}

	for _, hbytes := range msg.Data {
		header := new(types.Header)
		err = json.Unmarshal(hbytes, header)
		if err != nil {
			log.Error("failed to unmarshal bytes to block header,error: %v", err)
			continue
		}
		log.Info("receive header %s", header.Hash().String())

		// TODO: insert received header to local chain

	}

	// no response for block headers msg
	return nil, nil
}

func (c *Chain) handleTransactions(ctx context.Context, p p2p.ID, pmes *Message) (_ *Message, err error) {
	msg := pmes.DataMsg
	if msg == nil {
		return nil, errEmptyMsgContent
	}

	for _, txbytes := range msg.Data {
		tx := new(*types.Transaction)
		err = json.Unmarshal(txbytes, tx)
		if err != nil {
			log.Error("failed to unmarshal bytes to transaction,error: %v", err)
			continue
		}

		// TODO: execute received tx ..

	}

	// no resp
	return nil, nil
}

func (c *Chain) handleGetReceipts(ctx context.Context, p p2p.ID, pmes *Message) (_ *Message, err error) {
	var resp *Message
	msg := pmes.DataMsg
	if msg == nil {
		return nil, errEmptyMsgContent
	}

	receiptBytes := [][]byte{}
	for _, rhbytes := range msg.Data {
		receiptHash := common.Hash{}
		err = json.Unmarshal(rhbytes, &receiptHash)
		if err != nil {
			log.Error("failed to unmarshal bytes to transaction,error: %v", err)
			continue
		}

		receipt := c.blockchain.GetReceiptByHash(receiptHash)
		rbytes, err := json.Marshal(receipt)
		if err != nil {
			log.Error("failed to marshal receipt to bytes,error: %v", err)
			continue
		}

		receiptBytes = append(receiptBytes, rbytes)
	}
	receiptsMsg := &DataMsg{
		Data: receiptBytes,
	}
	resp = NewMessage(Message_RECEIPTS, receiptsMsg)
	return resp, nil
}

func (c *Chain) handleReceipts(ctx context.Context, p p2p.ID, pmes *Message) (_ *Message, err error) {
	msg := pmes.DataMsg
	if msg == nil {
		return nil, errEmptyMsgContent
	}

	for _, rbytes := range msg.Data {
		receipt := new(*types.Receipt)
		err = json.Unmarshal(rbytes, receipt)
		if err != nil {
			log.Error("failed to unmarshal bytes to receipt,error: %v", err)
			continue
		}

		// TODO: update local state ..

	}

	// no resp
	return nil, nil
}

func (c *Chain) handleNewBlockHashs(ctx context.Context, p p2p.ID, pmes *Message) (_ *Message, err error) {

	var resp *Message
	msg := pmes.DataMsg
	if msg == nil {
		return nil, errEmptyMsgContent
	}

	// collect block headers
	hashBytes := msg.Data
	headerBytes := [][]byte{}
	for _, hbytes := range hashBytes {
		hash := common.Hash{}
		err = json.Unmarshal(hbytes, &hash)
		if err != nil {
			log.Error("failed to unmarshal bytes to common hash,error: %v", err)
			continue
		}
		header := c.blockchain.GetHeaderByHash(hash)
		if header == nil {

			// maybe request this block from peers if local hasn't。
			//log.Error("failed to get header %s from local", hash.String())
			thisPeerID := hex.EncodeToString(p.PublicKey)
			for _, peer := range c.peers.peers {
				if peer.ID != thisPeerID {

					// TODO: request block Or header ?
					peer.SendNewBlockHashes([]common.Hash{hash})
				}
			}

			continue
		}
		hbytes, err := json.Marshal(header)
		if err != nil {
			log.Error("failed to marshal block header %s to bytes,error: %v", hash.String(), err)
			continue
		}
		headerBytes = append(headerBytes, hbytes)
	}

	// TODO：use fetcher or directly send p2p response ？

	// p2p resp: send block headers msg
	blockHeadersMsg := &DataMsg{
		Data: headerBytes,
	}
	resp = NewMessage(Message_BLOCK_HEADERS, blockHeadersMsg)
	return resp, nil
}

func (c *Chain) handleNewBlock(ctx context.Context, p p2p.ID, pmes *Message) (_ *Message, err error) {

	log.Info("#####enter into handleNewBlock...")
	//fmt.Println("接收到new block消息：%v", pmes.String())
	var resp *Message
	msg := pmes.DataMsg
	if msg == nil {
		return nil, errEmptyMsgContent
	}
	for _, bbytes := range msg.Data {

		receiveBlock := new(types.Block)
		err = json.Unmarshal(bbytes, receiveBlock)
		if err != nil {
			log.Error("failed to unmarshal bytes to block,error: %v", err)
			continue
		}
		//fmt.Println("反序列化接收到new block里的header消息：%v", receiveBlock.Header())
		fmt.Println("反序列化接收到new block里的number消息：%v", receiveBlock.Header().Number)
		fmt.Println("反序列化接收到new block里的hash消息：%v", receiveBlock.Hash().String())
		fmt.Println("反序列化接收到new block里的ParentHash消息：%v", receiveBlock.ParentHash().String())
		fmt.Println("反序列化接收到new block里的Difficulty消息：%v", receiveBlock.Difficulty())

		// mark remote peer hash known this block
		id := hex.EncodeToString(p.PublicKey)
		c.peers.Peer(id).MarkBlock(receiveBlock.Hash())

		// fmt.Printf(`
		// 	number: %d
		// 	{
		// 		hash: %s
		// 		parent: %s
		// 		state: %s
		// 		diff: 0x%x
		// 		gaslimit: %d
		// 		gasused: %d
		// 		nonce: 0x%x
		// 	}`+"\n", receiveBlock.Number(), receiveBlock.Hash().String(), receiveBlock.ParentHash().String(), receiveBlock.StateRoot().String(), "00", receiveBlock.GasLimit(), receiveBlock.GasUsed(), receiveBlock.Nonce())

		// TODO: insert received block to local chain
		//var localCurrentBlock = c.blockchain.CurrentBlock()
		//	var localCurrentBlockNum = localCurrentBlock.NumberU64()
		//	var localCurrentBlockHash = localCurrentBlock.Hash()
		//var localCurrentBlockParentHash = localCurrentBlock.ParentHash()
		//	var localCurrentBlockTD = rawdb.ReadTd(c.blockchain.GetDb(), localCurrentBlockHash, localCurrentBlockNum)

		//TODO:need to decode msg to get real receive block
		//var receiveBlockNum = receiveBlock.NumberU64()
		//var receiveBlockDifficult = receiveBlock.Difficulty()
		//var receiveBlockHash = receiveBlock.Hash()
		//var receiveBlockParentHash = receiveBlock.ParentHash()
		//var receiveBlockTD = receiveBlock.DeprecatedTd()                                   //just for test
		//var receiveParentBlockTD = new(big.Int).Sub(receiveBlockTD, receiveBlockDifficult) //just for test
		// remoteBlock.ReceivedAt = msg.ReceivedAt
		// remoteBlock.ReceivedFrom = p

		//1. validata receive block
		if err := c.blockchain.Validator().ValidateBody(receiveBlock); err != nil {
			//log.Error("Validator block error:", err)
			return resp, nil
		}

		//2.insert block and post Event
		//TODO:use queue to fetcher block data
		// broadcase Block
		//TODO:implement peer broadcast
		go c.BroadcastBlock(receiveBlock, true)
		//insert block and post Event
		c.blockchain.InsertChain([]*types.Block{receiveBlock})
		// broadcase Block hash
		go c.BroadcastBlock(receiveBlock, false)

		//3.synchronise data
		// Update the peers total difficulty if better than the previous
		//TODO1:set peer head,manager td and block head of peer conn
		//if _, td := p.Head(); trueTD.Cmp(td) > 0 {
		//	p.SetHead(trueHead, trueTD)
		//if receiveParentBlockTD.Cmp(localCurrentBlockTD) > 0 {
		//TODO2:implement synchronise data
		//go pm.synchronise(p)
		//}

		// TODO:
		return resp, nil

	}

	// no resp
	return nil, nil
}

func (c *Chain) handleGetBlocks(ctx context.Context, p p2p.ID, pmes *Message) (_ *Message, err error) {
	var resp *Message
	msg := pmes.DataMsg
	if msg == nil {
		return nil, errEmptyMsgContent
	}

	// TODO
	return resp, nil
}
