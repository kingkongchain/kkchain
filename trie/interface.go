package trie

import "github.com/invin/kkchain/common"

type Trie interface {
	Hash() common.Hash
	TryGet(key []byte) ([]byte, error)
	TryUpdate(key, value []byte) error
	TryDelete(key []byte) error
	Commit() (common.Hash, error)
}
