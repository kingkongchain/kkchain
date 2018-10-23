package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"
	"sort"

	//"strconv"
	"syscall"
	//"time"

	"math/big"

	"github.com/invin/kkchain/accounts"
	"github.com/invin/kkchain/accounts/keystore"
	"github.com/invin/kkchain/config"
	"github.com/invin/kkchain/core/state"
	"github.com/invin/kkchain/node"
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

var (
	version   string
	commit    string
	branch    string
	compileAt string
)

func main() {
	app := cli.NewApp()
	app.Action = run
	app.Name = "kkchain"
	//app.Version = fmt.Sprintf("%s, branch %s, commit %s", version, branch, commit)
	//timestamp, _ := strconv.ParseInt(compileAt, 10, 64)
	//app.Compiled = time.Unix(timestamp, 0)
	app.Usage = "the kkchain command line interface"
	app.Copyright = "Copyright 2018-2019 The kkchain Authors"

	app.Flags = append(app.Flags, ConfigFlag)
	app.Flags = append(app.Flags, GeneralFlags...)
	app.Flags = append(app.Flags, NetworkFlags...)
	app.Flags = append(app.Flags, DhtFlags...)
	app.Flags = append(app.Flags, ConsensusFlags...)
	app.Flags = append(app.Flags, APIFlags...)

	sort.Sort(cli.FlagsByName(app.Flags))

	app.Commands = []cli.Command{}

	sort.Sort(cli.CommandsByName(app.Commands))

	if err := app.Run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx *cli.Context) error {

	cfg := makeConfig(ctx)

	n, err := makeNode(cfg)
	if err != nil {
		return err
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	quitCh := make(chan bool, 1)

	n.Start()

	go func() {
		<-c

		n.Stop()
		log.Error("exit")

		quitCh <- true
		return
	}()

	select {
	case <-quitCh:
		return nil
	}

}

func makeConfig(ctx *cli.Context) *config.Config {
	// Load defaults.
	cfg := config.Config{
		GeneralConfig: config.DefaultGeneralConfig,
		Network:       &config.DefaultNetworkConfig,
		Dht:           &config.DefaultDhtConfig,
		Api:           &config.DefaultAPIConfig,
		Consensus:     &config.DefaultConsensusConfig,
	}

	// Load config file.
	if file := ctx.GlobalString(ConfigFlag.Name); file != "" {
		if err := config.LoadConfig(file, &cfg); err != nil {
			log.Fatalf("%v", err)
		}
	}

	// load config from cli args
	if err := generalConfig(ctx, &cfg.GeneralConfig); err != nil {
		log.Fatalf("%v", err)
	}

	networkConfig(ctx, cfg.Network)
	dhtConfig(ctx, &cfg)
	consensusConfig(ctx, cfg.Consensus)
	apiConfig(ctx, cfg.Api)

	return &cfg
}

func makeNode(cfg *config.Config) (*node.Node, error) {

	// make keystore
	ks, keydir, _ := makeKeystore()

	// make account
	acc, _ := ks.NewAccount("123")

	fmt.Printf("\ncreated a new account: %s\n", acc.Address.String())
	node, err := node.New(cfg, keydir, ks)
	if err != nil {
		log.Fatalf("New node error:", err)
	}
	currentBlock := node.BlockChain().CurrentBlock()
	statedb, err := state.New(currentBlock.StateRoot(), state.NewDatabase(node.ChainDb()))
	if err != nil {
		log.Fatalf("New state error:", err)
	}
	statedb.AddBalance(acc.Address, new(big.Int).SetInt64(1e10))
	statedb.SetNonce(acc.Address, 1)
	currentBlock.Header().StateRoot = statedb.IntermediateRoot(true)

	return node, err
}

func makeAccountManager() (*accounts.Manager, string, error) {
	scryptN, scryptP := keystore.LightScryptN, keystore.LightScryptP

	keydir, err := ioutil.TempDir("", "kkchain-keystore")
	if err != nil {
		return nil, "", err
	}
	if err = os.MkdirAll(keydir, 0700); err != nil {
		return nil, "", err
	}
	// Assemble the account manager and supported backends
	backends := []accounts.Backend{
		keystore.NewKeyStore(keydir, scryptN, scryptP),
	}
	return accounts.NewManager(backends...), keydir, nil
}

func makeKeystore() (*keystore.KeyStore, string, error) {
	scryptN, scryptP := keystore.LightScryptN, keystore.LightScryptP
	keydir, err := ioutil.TempDir("", "kkchain-keystore")
	if err != nil {
		return nil, "", err
	}
	if err = os.MkdirAll(keydir, 0700); err != nil {
		return nil, "", err
	}
	return keystore.NewKeyStore(keydir, scryptN, scryptP), keydir, nil
}
