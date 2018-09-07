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
	"github.com/invin/kkchain/consensus/pow"
	"github.com/invin/kkchain/core"
	"github.com/invin/kkchain/crypto/blake2b"
	"github.com/invin/kkchain/crypto/ed25519"
	"github.com/invin/kkchain/klogging"
	"github.com/invin/kkchain/miner"
	"github.com/invin/kkchain/p2p"
	"github.com/invin/kkchain/p2p/impl"
	"github.com/invin/kkchain/params"
)

// init config and loggingLoger
func init() {
	config.Init("")
	klogging.Init(config.AppConfig.Log.GetString("log.level"), config.AppConfig.Log.GetString("log.format"))
}

func main() {
	port := flag.String("p", "9998", "")
	sec := flag.Int("s", 120, "")
	keypath := flag.String("k", "", "")
	flag.Parse()
	seconds := time.Duration(*sec)

	config := &core.Config{DataDir: ""}

	chainDb, _ := core.OpenDatabase(config, "chaindata")

	chainConfig, genesisHash, genesisErr := core.SetupGenesisBlock(chainDb, nil)
	if _, ok := genesisErr.(*params.ConfigCompatError); genesisErr != nil && !ok {
		fmt.Printf("setup genesis failed： %s\n", genesisErr)
		return
	}
	fmt.Printf("Initialised chain configuration", "config", chainConfig, "genesis", genesisHash.String())

	chain, _ := core.NewBlockChain(chainDb)

	// TODO: start p2p ..
	doP2P(chain, *port, *keypath)

	// TODO: start miner ..
	doMiner(chain)

	time.Sleep(time.Second * seconds)
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
		fmt.Printf("remote peer: %s\n", node)
		net.BootstrapNodes = []string{node}
	}

	err := net.Start()
	if err != nil {
		fmt.Printf("failed to start server: %s\n", err)
	}

}

func doMiner(chain *core.BlockChain) {
	fmt.Println("\n进入doMiner 。。。\n")
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
		PowMode:        pow.ModeNormal,
	}

	engine := pow.New(powconfig, nil)
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
