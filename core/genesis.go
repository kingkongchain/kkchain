package core

import (
	"encoding/json"
	"fmt"
	"github.com/invin/kkchain/common"
	"github.com/invin/kkchain/common/hexutil"
	"github.com/invin/kkchain/core/types"
	"math/big"
	"os"
)

//for test
//type Genesis struct {
//	Nonce      uint64         `json:"nonce"`
//	Timestamp  uint64         `json:"timestamp"`
//	ExtraData  []byte         `json:"extraData"`
//	GasLimit   uint64         `json:"gasLimit"   gencodec:"required"`
//	Difficulty *big.Int       `json:"difficulty" gencodec:"required"`
//	Mixhash    common.Hash    `json:"mixHash"`
//	Coinbase   common.Address `json:"coinbase"`
//
//	// These fields are used for consensus tests. Please don't use them
//	// in actual genesis blocks.
//	Number     uint64      `json:"number"`
//	GasUsed    uint64      `json:"gasUsed"`
//	ParentHash common.Hash `json:"parentHash"`
//}

type Genesis struct {
	Nonce      uint64         `json:"nonce"`
	Timestamp  uint64         `json:"timestamp"`
	ExtraData  []byte         `json:"extraData"`
	GasLimit   uint64         `json:"gasLimit"   gencodec:"required"`
	Difficulty *big.Int       `json:"difficulty" gencodec:"required"`
	Mixhash    common.Hash    `json:"mixHash"`
	Coinbase   common.Address `json:"coinbase"`

	// These fields are used for consensus tests. Please don't use them
	// in actual genesis blocks.
	Number     uint64      `json:"number"`
	GasUsed    uint64      `json:"gasUsed"`
	ParentHash common.Hash `json:"parentHash"`
}

// readGenesis will read the given JSON format genesis file and return
// the initialized Genesis structure
func readGenesis(genesisPath string) *Genesis {
	// Make sure we have a valid genesis JSON
	//genesisPath := ctx.Args().First()
	if len(genesisPath) == 0 {
		fmt.Printf("Must supply path to genesis JSON file\n")
	}
	file, err := os.Open(genesisPath)
	if err != nil {
		fmt.Printf("Failed to read genesis file: %v\n", err)
	}
	defer file.Close()

	genesis := new(Genesis)
	if err := json.NewDecoder(file).Decode(genesis); err != nil {
		fmt.Printf("invalid genesis file: %v\n", err)
	}
	return genesis
}

func (g *Genesis) ToBlock() *types.Block {

	head := &types.Header{
		Number:     new(big.Int).SetUint64(g.Number),
		Nonce:      types.EncodeNonce(g.Nonce),
		Time:       new(big.Int).SetUint64(g.Timestamp),
		ParentHash: g.ParentHash,
		Extra:      g.ExtraData,
		Difficulty: g.Difficulty,
		MixDigest:  g.Mixhash,
		Miner:      g.Coinbase,
		StateRoot:  common.Hash{},
	}

	return types.NewBlock(head, nil)
}

// DefaultGenesisBlock returns the Ethereum main net genesis block.
func DefaultGenesisBlock() *Genesis {

	return &Genesis{
		Nonce:      66,
		ExtraData:  hexutil.MustDecode("0x11bbe8db4e347b4e8c937c1c8370e4b5ed33adb3db69cbdb7a38e1e50b1b82fa"),
		GasLimit:   5000,
		Difficulty: new(big.Int).SetBytes([]byte{02, 0, 0}),
		//Difficulty: big.NewInt(17179869184),
	}
}
