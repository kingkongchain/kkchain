package main

import (
	"github.com/invin/kkchain/config"

	"github.com/urfave/cli"
	"os"
	"path/filepath"
)

var (

	// ConfigFlag config file path
	ConfigFlag = cli.StringFlag{
		Name:  "config",
		Usage: "TOML configuration file",
	}

	DataDirFlag = cli.StringFlag{
		Name:  "datadir",
		Usage: "Data directory for the databases and keystore",
		Value: "",
	}

	// GeneralFlags config list
	GeneralFlags = []cli.Flag{
		DataDirFlag,
	}

	// NetworkSeedFlag network seed
	NetworkSeedsFlag = cli.StringSliceFlag{
		Name:  "network.seed",
		Usage: "network seed addresses, multi-value support.",
	}

	// NetworkListenFlag network listen
	NetworkListenFlag = cli.StringFlag{
		Name:  "network.listen",
		Usage: "network listen addresses, multi-value support.",
	}

	// NetworkKeyPathFlag network key
	NetworkKeyPathFlag = cli.StringFlag{
		Name:  "network.nodekey",
		Usage: "network node private key file path",
	}

	NetworkMaxPeersFlag = cli.IntFlag{
		Name:  "network.maxpeers",
		Usage: "Maximum number of network peers",
		Value: 20,
	}

	// NetworkFlags config list
	NetworkFlags = []cli.Flag{
		NetworkSeedsFlag,
		NetworkListenFlag,
		NetworkKeyPathFlag,
		NetworkMaxPeersFlag,
	}

	DhtBucketSizeFlag = cli.IntFlag{
		Name:  "dht.bucketsize",
		Usage: "Bucket size of routine table",
		Value: 16,
	}

	DhtFlags = []cli.Flag{
		DhtBucketSizeFlag,
	}

	ConsensusMineFlag = cli.BoolFlag{
		Name:  "consensus.mine",
		Usage: "Enable mining",
	}

	ConsensusTypeFlag = cli.StringFlag{
		Name:  "consensus.type",
		Usage: "Type of consensus",
		Value: "pow",
	}

	ConsensusFlags = []cli.Flag{
		ConsensusMineFlag,
		ConsensusTypeFlag,
	}

	RPCEnabledFlag = cli.BoolFlag{
		Name:  "api.rpc",
		Usage: "Enable the HTTP-RPC server",
	}
	RPCListenAddrFlag = cli.StringFlag{
		Name:  "api.rpcaddr",
		Usage: "HTTP-RPC server listening interface",
	}

	APIFlags = []cli.Flag{
		RPCEnabledFlag,
		RPCListenAddrFlag,
	}
)

func generalConfig(ctx *cli.Context, cfg *config.GeneralConfig) error {
	if ctx.GlobalIsSet(DataDirFlag.Name) {
		cfg.DataDir = ctx.GlobalString(DataDirFlag.Name)
	}

	if cfg.DataDir != "" {
		absdatadir, err := filepath.Abs(cfg.DataDir)
		if err != nil {
			return err
		}
		cfg.DataDir = absdatadir

		if err := os.MkdirAll(cfg.DataDir, 0700); err != nil {
			return err
		}
	}

	return nil
}

func networkConfig(ctx *cli.Context, cfg *config.NetworkConfig) {
	if ctx.GlobalIsSet(NetworkSeedsFlag.Name) {
		cfg.Seeds = ctx.GlobalStringSlice(NetworkSeedsFlag.Name)
	}
	if ctx.GlobalIsSet(NetworkListenFlag.Name) {
		cfg.Listen = ctx.GlobalString(NetworkListenFlag.Name)
	}
	if ctx.GlobalIsSet(NetworkKeyPathFlag.Name) {
		cfg.PrivateKey = ctx.GlobalString(NetworkKeyPathFlag.Name)
	}

	if ctx.GlobalIsSet(NetworkMaxPeersFlag.Name) {
		cfg.MaxPeers = int32(ctx.GlobalInt(NetworkMaxPeersFlag.Name))
	}
}

func dhtConfig(ctx *cli.Context, cfg *config.Config) {
	cfg.Dht.RoutingTableDir = filepath.Join(cfg.DataDir, "nodes")

	cfg.Dht.Seeds = cfg.Network.Seeds

}

func consensusConfig(ctx *cli.Context, cfg *config.ConsensusConfig) {
	if ctx.GlobalIsSet(ConsensusMineFlag.Name) {
		cfg.Mine = ctx.GlobalBool(ConsensusMineFlag.Name)
	}

	if ctx.GlobalIsSet(ConsensusMineFlag.Name) {
		cfg.Type = ctx.GlobalString(ConsensusTypeFlag.Name)
	}
}

func apiConfig(ctx *cli.Context, cfg *config.ApiConfig) {
	if ctx.GlobalIsSet(RPCEnabledFlag.Name) {
		cfg.Rpc = ctx.GlobalBool(RPCEnabledFlag.Name)
	}

	if ctx.GlobalIsSet(RPCListenAddrFlag.Name) {
		cfg.RpcAddr = ctx.GlobalString(RPCListenAddrFlag.Name)
	}
}
