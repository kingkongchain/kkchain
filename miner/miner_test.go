package miner

import (
	"github.com/invin/kkchain/common"
	"github.com/invin/kkchain/consensus/pow"
	"github.com/invin/kkchain/core"
	"os"
	"os/user"
	"path/filepath"
	"testing"
	"time"
)

func TestMiner_Start(t *testing.T) {

}

func TestMiner_Stop(t *testing.T) {

}

func TestMine(t *testing.T) {

	home := os.Getenv("HOME")
	if home == "" {
		if user, err := user.Current(); err == nil {
			home = user.HomeDir
		}
	}

	chain := core.NewBlockChain()
	config := pow.Config{
		CacheDir:       "ethash",
		CachesInMem:    2,
		CachesOnDisk:   3,
		DatasetDir:     filepath.Join(home, ".ethash"),
		DatasetsInMem:  1,
		DatasetsOnDisk: 2,
		PowMode:        pow.ModeNormal,
	}

	logger.Info(config.DatasetDir)
	engine := pow.New(config, nil)
	defer engine.Close()

	miner := New(chain, engine)
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