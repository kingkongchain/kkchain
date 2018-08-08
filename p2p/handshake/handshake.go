package handshake 

import (
	"context"

	"github.com/invin/kkchain/p2p"
	"github.com/invin/kkchain/p2p/handshake/pb"
	"github.com/gogo/protobuf/proto"
	"github.com/golang/glog"
)

const (
	protocolHandshake = "/kkchain/p2p/handshake/1.0.0"
)

// Handshake implements protocol for handshaking
type Handshake struct {
	// self
	host p2p.Host
}

// NewHandshake creates a new Handshake object with the given peer as as the 'local' host
func NewHandshake(host p2p.Host) *Handshake {
	hs := &Handshake {
		host: host,
	}

	if err := host.SetStreamHandler(protocolHandshake, hs.handleNewStream); err != nil {
		panic(err)
	}

	return hs
}

// handleNewStream handles messages within the stream
func (hs *Handshake) handleNewStream(s p2p.Stream, msg proto.Message) {
	// check message type
	switch message := msg.(type) {
	case *pb.Message:
		hs.handleMessage(s, message)
	default:
		s.Reset()
		glog.Errorf("unexpected message: %v", msg)
	}	
}

// handleMessage handles messsage 
func (hs *Handshake) handleMessage(s p2p.Stream, msg *pb.Message) {
	// get handler
	handler := hs.handlerForMsgType(msg.GetType())
	if handler == nil {
		s.Reset()
		glog.Errorf("unknown message type: %v", msg.GetType())
		return	
	}

	// dispatch handler
	// TODO: get context and peer id
	ctx := context.Background()
	pid := s.RemotePeer()

	rpmes, err := handler(ctx, pid, msg)

	// if nil response, return it before serializing
	if rpmes == nil {
		glog.Warning("got back nil response from request")
		return
	}

	// send out response msg
	if err = s.Write(rpmes); err != nil {
		s.Reset()
		glog.Errorf("send response error: %s", err)
		return
	}
}