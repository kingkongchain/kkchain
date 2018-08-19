package chain

import (
	"context"

	"github.com/gogo/protobuf/proto"
	"github.com/golang/glog"
	"github.com/invin/kkchain/p2p"
)

const (
	protocolChain = "/kkchain/p2p/chain/1.0.0"
)

// Chain implements protocol for chain related messages
type Chain struct {
	// self
	host p2p.Host
}

// New creates a new Chain object
func New(host p2p.Host) *Chain {
	c := &Chain{
		host: host,
	}

	if err := host.SetMessageHandler(protocolChain, c.handleMessage); err != nil {
		panic(err)
	}

	return c
}

// handleMessage handles messages within the stream
func (c *Chain) handleMessage(conn p2p.Conn, msg proto.Message) {
	// check message type
	switch message := msg.(type) {
	case *Message:
		c.doHandleMessage(conn, message)
	default:
		conn.Close()
		glog.Errorf("unexpected message: %v", msg)
	}
}

// doHandleMessage handles messsage
func (c *Chain) doHandleMessage(conn p2p.Conn, msg *Message) {
	// get handler
	handler := c.handlerForMsgType(msg.GetType())
	if handler == nil {
		conn.Close()
		glog.Errorf("unknown message type: %v", msg.GetType())
		return
	}

	// dispatch handler
	// TODO: get context and peer id
	ctx := context.Background()
	pid := conn.RemotePeer()

	rpmes, err := handler(ctx, pid, msg)

	// if nil response, return it before serializing
	if rpmes == nil {
		glog.Warning("got back nil response from request")
		return
	}

	// send out response msg
	if err = conn.WriteMessage(rpmes, protocolChain); err != nil {
		conn.Close()
		glog.Errorf("send response error: %s", err)
		return
	}
}
