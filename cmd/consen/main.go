package main

import (
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"time"

	"github.com/invin/kkchain/common"
	"github.com/invin/kkchain/config"
	"github.com/invin/kkchain/consensus"
	"github.com/invin/kkchain/consensus/pow"
	"github.com/invin/kkchain/core"
	"github.com/invin/kkchain/crypto/blake2b"
	"github.com/invin/kkchain/crypto/ed25519"
	"github.com/invin/kkchain/klogging"
	"github.com/invin/kkchain/miner"
	"github.com/invin/kkchain/p2p"
	"github.com/invin/kkchain/p2p/impl"
	"github.com/invin/kkchain/params"
	logging "github.com/op/go-logging"
)

var log = logging.MustGetLogger("cmd/consensus")

// init config and loggingLoger
func Init(path *string) {
	if path != nil {
		fmt.Println("path:", *path)
		config.Init(*path)
	} else {
		fmt.Println("path:", *path)
		config.Init("")
	}

	klogging.Init(config.AppConfig.Log.GetString("log.level"), config.AppConfig.Log.GetString("log.format"))
}

func main() {
	port := flag.String("p", "9998", "")
	//sec := flag.Int("s", 120, "")
	keypath := flag.String("k", "", "")
	configpath := flag.String("c", "", "")
	mineFlag := flag.String("m", "", "")
	flag.Parse()
	log.Info("configpath:", *configpath)
	Init(configpath)
	//seconds := time.Duration(*sec)

	config := &core.Config{DataDir: ""}

	chainDb, _ := core.OpenDatabase(config, "chaindata")

	chainConfig, genesisHash, genesisErr := core.SetupGenesisBlock(chainDb, nil)
	if _, ok := genesisErr.(*params.ConfigCompatError); genesisErr != nil && !ok {
		log.Info("setup genesis failed： %s\n", genesisErr)
		return
	}
	log.Info("Initialised chain configuration", "config", chainConfig, "genesis", genesisHash.String())

	engine := newEngine()
	chain, _ := core.NewBlockChain(chainDb, engine)

	go func() {
		ticker := time.NewTicker(8 * time.Second)
		for _ = range ticker.C {
			block := chain.CurrentBlock()
			log.Info("!!!!!blockchain info: CurrentBlock:====>")
			fmt.Printf(`
				number: %d 
				{
					hash: %s
					parent: %s
					state: %s
					diff: 0x%x
					gaslimit: %d
					gasused: %d
					nonce: 0x%x
				}`+"\n", block.Number(), block.Hash().String(), block.ParentHash().String(), block.StateRoot().String(), block.Difficulty(), block.GasLimit(), block.GasUsed(), block.Nonce())
		}
	}()

	// TODO: start p2p ..
	go doP2P(chain, *port, *keypath)

	// TODO: start miner ..
	if *mineFlag == "true" {
		doMiner(chain, engine)
	}

	select {}
	//time.Sleep(time.Second * seconds)
}

func doP2P(bc *core.BlockChain, port, keypath string) {

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
		log.Info("remote peer: %s\n", node)
		net.BootstrapNodes = []string{node}
	}

	err := net.Start()
	if err != nil {
		log.Error("failed to start server: %s\n", err)
	}

}

func newEngine() consensus.Engine {
	home := os.Getenv("HOME")
	if home == "" {
		if user, err := user.Current(); err == nil {
			home = user.HomeDir
		}
	}
	powconfig := pow.Config{
		CacheDir:       "ethash",
		CachesInMem:    2,
		CachesOnDisk:   3,
		DatasetDir:     filepath.Join(home, ".ethash"),
		DatasetsInMem:  1,
		DatasetsOnDisk: 2,
		//PowMode:        pow.ModeNormal,
		PowMode: pow.ModeFake,
	}
	engine := pow.New(powconfig, nil)
	return engine

}

func doMiner(chain *core.BlockChain, engine consensus.Engine) {
	log.Info("\n进入doMiner 。。。\n")

	defer engine.Close()

	txpool := core.NewTxPool()

	miner := miner.New(chain, txpool, engine)
	defer miner.Close()

	miner.SetMiner(common.HexToAddress("0x67b1043995cf9fb7dd27f6f7521342498d473c05"))
	miner.Start()

	time.Sleep(time.Duration(2 * time.Second))
	chain.PostSyncDoneEvent(struct{}{})
	//time.Sleep(time.Duration(1 * time.Second))
	//chain.PostSyncDoneEvent(struct{}{})

	wait := make(chan interface{})
	select {
	case <-wait:

	}
}
