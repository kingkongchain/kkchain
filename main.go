package main

import (
	"syscall"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"time"
	"os/signal"

	"github.com/invin/kkchain/common"
	"github.com/invin/kkchain/consensus"
	"github.com/invin/kkchain/consensus/pow"
	"github.com/invin/kkchain/core"
	"github.com/invin/kkchain/crypto/blake2b"
	"github.com/invin/kkchain/crypto/ed25519"
	"github.com/invin/kkchain/miner"
	"github.com/invin/kkchain/p2p"
	"github.com/invin/kkchain/p2p/impl"
	"github.com/invin/kkchain/params"
	log "github.com/sirupsen/logrus"
	"github.com/jbenet/goprocess"
)

func main() {
	port := flag.String("p", "9998", "")
	keypath := flag.String("k", "", "")
	mineFlag := flag.String("m", "", "")
	fakeFlag := flag.String("f", "fakeMineFlag", "true is fake mine")
	dataStoreDir := flag.String("d", "dataStoreDir", "A directory that stores block data")
	flag.Parse()

	c := make(chan os.Signal, 2)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	log.Println("Setup signal notifiers for os.Interrupt and syscall.SIGTERM")

	config := &core.Config{DataDir: "data"}

	start := time.Now()
	chainDb, _ := core.OpenDatabase(config, *dataStoreDir)
	log.Printf("Opening rodcksdb takes %s\n", time.Since(start))

	chainConfig, genesisHash, genesisErr := core.SetupGenesisBlock(chainDb, nil)
	if _, ok := genesisErr.(*params.ConfigCompatError); genesisErr != nil && !ok {
		log.Errorf("setup genesis failed： %s\n", genesisErr)
		return
	}
	log.WithFields(log.Fields{
		"config":  chainConfig,
		"genesis": genesisHash.String(),
	}).Info("Initialised chain configuration")

	engine := newEngine(fakeFlag)
	chain, _ := core.NewBlockChain(chainDb, engine)

	// Use goprocess to setup process tree
	proc := goprocess.WithTeardown(func() error {
		log.Info("Tear down kkchain")
		chainDb.Close()
		log.Printf("Chaindb closed")	
		return nil
	})

	proc.Go(func(p goprocess.Process) {
		ticker := time.NewTicker(8 * time.Second)
		// loop to print current block info
		for {
			select {
			case <- ticker.C:
				block := chain.CurrentBlock()
				fmt.Printf("!!!!!blockchain info: CurrentBlock:====> %s", block.String())
			case <- p.Closing():
				log.Println("Exit block ticker")
				return
			default:
				time.Sleep(1 * time.Second)
			}
			
		}
	})

	// Setup handler for interrupt or terminate
	go func(){
		for sig := range c {
			// sig is a ^C, handle it
			log.Printf("Asked to quit now %v", sig)
			proc.Close()
			// TODO: add other clean code
			time.Sleep(3 * time.Second)
			log.Println("Everything is shutdown properly, exit now")		
			os.Exit(1)
		}
	}()

	// Run p2p module
	proc.Go(func(p goprocess.Process) {
		doP2P(chain, *port, *keypath, p)
	})

	// Run mining if requested
	if *mineFlag == "true" {
		proc.Go(func(p goprocess.Process) {
			doMiner(chain, engine, p)
			engine.Close()
			log.Println("Engine closed")
		})
	}

	// Wait for break 
	select {}
}

func doP2P(bc *core.BlockChain, port, keypath string, p goprocess.Process) {

	p2pconfig := p2p.Config{
		SignaturePolicy: ed25519.New(),
		HashPolicy:      blake2b.New(),
	}

	listen := "/ip4/127.0.0.1/tcp/" + port

	net := impl.NewNetwork(keypath, listen, p2pconfig, bc)

	if port != "9998" {
		remoteKeyPath := "node1.key"
		pri, _ := p2p.LoadNodeKeyFromFile(remoteKeyPath)
		pub, _ := ed25519.New().PrivateToPublic(pri)
		node := "/ip4/127.0.0.1/tcp/9998"
		node = hex.EncodeToString(pub) + "@" + node
		log.Infof("remote peer: %s", node)
		net.BootstrapNodes = []string{node}
	}

	err := net.Start()
	if err != nil {
		log.Errorf("failed to start server: %s\n", err)
		return
	}

	select {
	case <-p.Closing():
		net.Stop()
		log.Print("Network stopped")
	}
}

func newEngine(fakeFlag *string) consensus.Engine {
	home := os.Getenv("HOME")
	if home == "" {
		if user, err := user.Current(); err == nil {
			home = user.HomeDir
		}
	}
	var engine *pow.Ethash
	if *fakeFlag == "true" {
		engine = pow.NewFakeDelayer(15 * time.Second)
	} else {
		powconfig := pow.Config{
			CacheDir:       "ethash",
			CachesInMem:    2,
			CachesOnDisk:   3,
			DatasetDir:     filepath.Join(home, ".ethash"),
			DatasetsInMem:  1,
			DatasetsOnDisk: 2,
			PowMode:        pow.ModeNormal,
		}
		engine = pow.New(powconfig, nil)
	}
	return engine

}

func doMiner(chain *core.BlockChain, engine consensus.Engine, p goprocess.Process) {
	txpool := core.NewTxPool()
	miner := miner.New(chain, txpool, engine)

	miner.SetMiner(common.HexToAddress("0x67b1043995cf9fb7dd27f6f7521342498d473c05"))
	miner.Start()

	select {
	case <-p.Closing():
		miner.Stop()
		miner.Close()
		log.Print("Miner stopped and closed")
	}
}
