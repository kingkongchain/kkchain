package types

import (
	"bytes"
	"fmt"
	"unsafe"

	"github.com/invin/kkchain/common"
)

const (
	// ReceiptStatusFailed is the status code of a transaction if execution failed.
	ReceiptStatusFailed = uint64(0)

	// ReceiptStatusSuccessful is the status code of a transaction if execution succeeded.
	ReceiptStatusSuccessful = uint64(1)
)

// Receipt represents the results of a transaction.
type Receipt struct {
	// Consensus fields
	PostState []byte `json:"root"`
	Status    uint64 `json:"status"`
	//CumulativeGasUsed uint64 `json:"cumulativeGasUsed" gencodec:"required"`

	// TODO：open when add VM
	//Bloom             Bloom  `json:"logsBloom"         gencodec:"required"`
	//Logs              []*Log `json:"logs"              gencodec:"required"`

	// Implementation fields (don't reorder!)
	TxHash common.Hash `json:"transactionHash" gencodec:"required"`
	//ContractAddress common.Address `json:"contractAddress"`
	//GasUsed uint64 `json:"gasUsed" gencodec:"required"`
}

// NewReceipt creates a barebone transaction receipt, copying the init fields.
func NewReceipt(root []byte, failed bool, cumulativeGasUsed uint64) *Receipt {
	r := &Receipt{PostState: common.CopyBytes(root)}
	if failed {
		r.Status = ReceiptStatusFailed
	} else {
		r.Status = ReceiptStatusSuccessful
	}
	return r
}

func (r *Receipt) setStatus(postStateOrStatus []byte) error {
	switch {
	case bytes.Equal(postStateOrStatus, []byte{0x01}):
		r.Status = ReceiptStatusSuccessful
	case bytes.Equal(postStateOrStatus, []byte{}):
		r.Status = ReceiptStatusFailed
	case len(postStateOrStatus) == len(common.Hash{}):
		r.PostState = postStateOrStatus
	default:
		return fmt.Errorf("invalid receipt status %x", postStateOrStatus)
	}
	return nil
}

func (r *Receipt) statusEncoding() []byte {
	if len(r.PostState) == 0 {
		if r.Status == ReceiptStatusFailed {
			return []byte{}
		}
		return []byte{0x01}
	}
	return r.PostState
}

// Size returns the approximate memory used by all internal contents. It is used
// to approximate and limit the memory consumption of various caches.
func (r *Receipt) Size() float64 {
	size := float64(unsafe.Sizeof(*r)) + float64(len(r.PostState))

	//size += common.StorageSize(len(r.Logs)) * common.StorageSize(unsafe.Sizeof(Log{}))
	//for _, log := range r.Logs {
	//	size += common.StorageSize(len(log.Topics)*common.HashLength + len(log.Data))
	//}
	return size
}
