package impl

import (
	"encoding/hex"
	"fmt"
	"net"

	"github.com/invin/kkchain/crypto"
	"github.com/invin/kkchain/p2p"
	"github.com/invin/kkchain/p2p/chain"
	"github.com/invin/kkchain/p2p/dht"
	"github.com/jbenet/goprocess"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

var (
	errServerStopped = errors.New("server stopped")
	log              = logrus.New()
)

// Network represents the whole stack of p2p communication between peers
type Network struct {
	// Configuration
	conf p2p.Config

	// Host to manage connections
	host p2p.Host

	// Address to listen on
	listenAddr string

	// Node's keypair.
	keys *crypto.KeyPair

	// Message modules
	dht   *dht.DHT
	chain *chain.Chain

	// Bootstrap seed nodes
	BootstrapNodes []string

	// process to manager other child processes
	proc goprocess.Process
}

// NewNetwork creates a new Network instance with the specified configuration
func NewNetwork(privateKeyPath, address string, conf p2p.Config) *Network {
	keys, _ := p2p.LoadNodeKeyFromFileOrCreateNew(privateKeyPath)
	id := p2p.CreateID(address, keys.PublicKey)

	n := &Network{
		conf:       conf,
		keys:       keys,
		listenAddr: address,
	}

	n.host = NewHost(id, n)

	// Create submodules
	n.chain = chain.New(n.host)
	n.dht = dht.New(dht.DefaultConfig(), n, n.host)

	return n
}

// Start kicks off the p2p stack
func (n *Network) Start() error {
	// TODO: use singleton mode

	log.Info("start P2P network")

	if n.keys == nil {
		return fmt.Errorf("Server.PrivateKey must be set to a non-nil key")
	}

	// Use goprocess to setup process tree
	n.proc = goprocess.WithTeardown(func() error {
		log.Info("Tear down network")
		// TODO: add other clean code
		return nil
	})

	// Start DHT as child of network process
	n.proc.Go(func(p goprocess.Process) {
		n.dht.Start()
		select {
		case <-p.Closing():
			log.Info("closing dht")
			n.dht.Stop()
			n.host.RemoveAllConnection()
		}
	})

	// Listen
	if n.listenAddr != "" {
		if err := n.startListening(); err != nil {
			return err
		}
	} else {
		log.Warn("P2P server will be useless, not listening")
	}

	// Start process to connect seed nodes
	n.proc.Go(func(p goprocess.Process) {
		n.bootstrap(p)
	})

	return nil
}

// Conf gets configurations
func (n *Network) Conf() p2p.Config {
	return n.conf
}

// Bootstraps returns seed nodes
func (n *Network) Bootstraps() []string {
	return n.BootstrapNodes
}

// Stop stops the p2p stack
func (n *Network) Stop() {
	n.host.RemoveAllConnection()
	n.proc.Close()
}

// startListening starts listen
func (n *Network) startListening() error {
	addr, err := dht.ToNetAddr(n.listenAddr)
	if err != nil {
		return err
	}

	// Enter listen mode
	listener, err := net.Listen(addr.Network(), addr.String())
	if err != nil {
		return err
	}

	laddr := listener.Addr().(*net.TCPAddr)
	n.listenAddr = laddr.String()

	// Setup process to kill listener on demand
	n.proc.Go(func(p goprocess.Process) {
		select {
		case <-p.Closing():
			// cause listener.Accept to stop blocking so it can breakout the loop
			log.Info("close listener")
			listener.Close()
		}
	})

	// Run listenr process
	n.proc.Go(func(p goprocess.Process) {
		// TODO: add addr info
		log.Info("Loop for incoming connections")
		for {
			if conn, err := listener.Accept(); err == nil {
				c := NewConnection(conn, n, n.host)
				n.host.OnConnectionCreated(c, p2p.Inbound)
			} else {
				// if we're about to shutdown, no need to continue with the loop
				select {
				case <-p.Closing():
					log.Info("Shutting down server")
					return
				default:
					log.Error(err)
				}
			}
		}
	})

	return nil
}

// Bootstrap connects to seed nodes
func (n *Network) bootstrap(p goprocess.Process) {
	for _, node := range n.BootstrapNodes {
		// Check if we're asked to shutdown
		select {
		case <-p.Closing():
			return
		default:
			log.Info("connect to ", node)
		}

		// Parse peer address to get IP
		peer, err := dht.ParsePeerAddr(node)
		if err != nil {
			log.Error(err)
			continue
		}

		// Reuse connection if it's already connected
		conn, _ := n.host.Connection(peer.ID)
		if conn != nil {
			log.Info("reuse connection??")
			continue
		}

		// Connect to peer node
		go func() {
			_, err := n.host.Connect(peer.Address)
			if err != nil {
				log.WithFields(logrus.Fields{
					"address": peer.Address,
					"nodeID":  hex.EncodeToString(peer.ID.PublicKey),
				}).Error("failed to connect boost node")
				return
			}

			// TODO: optimize
			log.WithFields(logrus.Fields{
				"address": peer.Address,
				"nodeID":  hex.EncodeToString(peer.ID.PublicKey),
			}).Info("success to connect boost node")
		}()
	return nil
}

// Sign signs a message
// TODO: move to another package??
func (n *Network) Sign(message []byte) ([]byte, error) {
	return n.keys.Sign(n.conf.SignaturePolicy, n.conf.HashPolicy, message)
}

// Verify verifies the message
// TODO: move to another package??
func (n *Network) Verify(publicKey []byte, message []byte, signature []byte) bool {
	return crypto.Verify(n.conf.SignaturePolicy, n.conf.HashPolicy, publicKey, message, signature)
}

// Proc returns network process
func (n *Network) Proc() goprocess.Process {
	return n.proc
}
