package config

import (
	"os"
	"os/user"
	"path/filepath"
	"runtime"

	"github.com/invin/kkchain/storage"
	"github.com/invin/kkchain/storage/memdb"
	"github.com/invin/kkchain/storage/rocksdb"
	"github.com/spf13/viper"
)

var (
	DefaultGeneralConfig = GeneralConfig{
		DataDir: DefaultDataDir(),
	}

	DefaultNetworkConfig = NetworkConfig{
		Listen:     "/ip4/127.0.0.1/tcp/9998",
		PrivateKey: "node1.key",
		NetworkId:  1,
		MaxPeers:   20,
	}

	DefaultDhtConfig = DhtConfig{
		BucketSize: 16,
	}

	DefaultConsensusConfig = ConsensusConfig{
		Mine: true,
		Type: "pow",
	}

	DefaultAPIConfig = ApiConfig{
		Rpc:     true,
		RpcAddr: "/ip4/127.0.0.1/tcp/8545",
	}
)

// kkchain global configurations.
type Config struct {
	GeneralConfig `mapstructure:",squash"`

	// network config.
	Network *NetworkConfig `mapstructure:"network"`
	// dht config.
	Dht *DhtConfig `mapstructure:"dht"`

	Consensus *ConsensusConfig `mapstructure:"consensus"`

	Api *ApiConfig `mapstructure:"api"`
}

// General settings
type GeneralConfig struct {
	DataDir string `mapstructure:"datadir"`
}

type NetworkConfig struct {
	//seed node address.
	Seeds []string `mapstructure:"seeds"`
	// Listen address.
	Listen string `mapstructure:"listen"`
	// Network node privateKey address. If nil, generate a new node.
	PrivateKey string `mapstructure:"privatekey"`
	// Network ID
	NetworkId uint32 `mapstructure:"networkid"`
	// Max peers connect with self node
	MaxPeers int32 `mapstructure:"maxpeers"`
}

type DhtConfig struct {
	BucketSize      int `mapstructure:"bucketsize"`
	RoutingTableDir string
	Seeds           []string
}

type ConsensusConfig struct {
	Mine bool   `mapstructure:"mine"`
	Type string `mapstructure:"type"`
}

type ApiConfig struct {
	Rpc     bool   `mapstructure:"rpc"`
	RpcAddr string `mapstructure:"rpcaddr"`
}

func LoadConfig(file string, cfg *Config) error {
	viper.SetConfigFile(file)
	viper.SetConfigType("toml")

	if err := viper.ReadInConfig(); err != nil {
		return err
	}

	err := viper.Unmarshal(cfg)
	if err != nil {
		return err
	}
	return nil
}

// DefaultDataDir is the default data directory to use for the databases and other
// persistence requirements.
func DefaultDataDir() string {
	// Try to place the data folder in the user's home dir
	home := homeDir()
	if home != "" {
		if runtime.GOOS == "darwin" {
			return filepath.Join(home, "Library", "KKchain")
		} else if runtime.GOOS == "windows" {
			return filepath.Join(home, "AppData", "Roaming", "KKchain")
		} else {
			return filepath.Join(home, ".kkchain")
		}
	}
	// As we cannot guess a stable location, return empty and handle later
	return ""
}

func homeDir() string {
	if home := os.Getenv("HOME"); home != "" {
		return home
	}
	if usr, err := user.Current(); err == nil {
		return usr.HomeDir
	}
	return ""
}

// ResolvePath resolves path in the instance directory.
func (c *Config) ResolvePath(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	if c.DataDir == "" {
		return ""
	}

	return filepath.Join(c.DataDir, path)
}

// OpenDatabase opens an existing database with the given name (or creates one if no
// previous can be found) from within the node's instance directory. If the node is
// ephemeral, a memory database is returned.
func OpenDatabase(c *Config, name string) (storage.Database, error) {
	if c.DataDir == "" {
		return memdb.New(), nil
	}

	db, err := rocksdb.New(c.ResolvePath(name), nil)
	if err != nil {
		return nil, err
	}
	return db, nil
}
