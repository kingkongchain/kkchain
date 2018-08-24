package miner

import (
	"fmt"
	"sync/atomic"
	"time"

	"github.com/invin/kkchain/consensus"
	"github.com/invin/kkchain/event"
	"github.com/invin/kkchain/types"

	"github.com/op/go-logging"
)

var (
	logger = logging.MustGetLogger("miner")
)

// task contains all information for consensus engine sealing and result submitting.
type task struct {
	block     *types.Block
	createdAt time.Time
}

type Miner struct {
	quitCh chan bool

	coinbase []byte //address

	//TODO: blockchain impl ChainReader interface
	chain  *consensus.ChainReader
	engine consensus.Engine

	mining   int32
	syncDone int32

	mineLoopCh chan interface{}

	// sync over
	syncDoneCh  chan struct{}
	syncDoneSub event.Subscription

	//tx pool add new txs
	txsCh  chan []*types.Transaction
	txsSub event.Subscription

	//new block from inbound msg
	newBlockCh  chan *types.Block
	newBlockSub event.Subscription

	taskCh   chan *task
	resultCh chan *task
}

func New() *Miner {
	miner := &Miner{}

	// Subscribe events from downloader
	miner.syncDoneCh = make(chan struct{})

	// Subscribe events from tx pool

	// Subscribe events from inbound handler

	go miner.onEvent()
	go miner.waitResult()

	return miner
}

// Start start mine service.
func (m *Miner) Start() {

	if atomic.LoadInt32(&m.syncDone) == 0 {
		logger.Info("Sync complete, start mining...")
		return
	}

	atomic.StoreInt32(&m.mining, 1)
	//Starting Mining...
	go m.mineLoop(m.mineLoopCh)

}

// Stop stop mine service.
func (m *Miner) Stop() {
	//Stopping  Mining...
	close(m.quitCh)

	atomic.StoreInt32(&m.mining, 0)
}

func (m *Miner) Mining() bool {
	return atomic.LoadInt32(&m.mining) > 0
}

func (m *Miner) onEvent() {
	defer m.syncDoneSub.Unsubscribe()
	defer m.txsSub.Unsubscribe()
	defer m.newBlockSub.Unsubscribe()

	for {
		select {
		case <-m.syncDoneCh:
			atomic.StoreInt32(&m.syncDone, 1)
			m.syncDoneSub.Unsubscribe()
			m.Start()
		case txs := <-m.txsCh:
			for _, tx := range txs {
				fmt.Println(tx)
			}
		case block := <-m.newBlockCh:
			fmt.Println(block)

		//
		case <-m.syncDoneSub.Err():
			return
		case <-m.txsSub.Err():
			return
		case <-m.newBlockSub.Err():
			return
		case <-m.quitCh:
			return
		}
	}
}

func (m *Miner) waitResult() {
	for {
		for result := range m.resultCh {
			// Short circuit when receiving empty result.
			if result == nil {
				continue
			}
			// Short circuit when receiving duplicate result caused by resubmitting.

			// Update the block hash in all logs since it is now available and not when the
			// receipt/log of individual transactions were created.

			// Commit block and state to database.

			// Broadcast the block and announce chain insertion event

			//trigger next mine loop
			m.mineLoopCh <- result
		}
	}
}

//
func (m *Miner) mineLoop(newLoop <-chan interface{}) {

	for {
		select {
		case now := <-newLoop:
			fmt.Println(now)
			//Start new mine task
			m.commitTask()
		case <-m.quitCh:
			//Stopped Mining
			return
		}
	}
}

func (m *Miner) commitTask() {
	if atomic.LoadInt32(&m.mining) == 0 {
		return
	}

	//get txs from pending pool
	txs := []types.Transaction{}

	//Initialize ctx
	block, _ := m.engine.Initialize(m.chain, txs)

	//commit new task
	m.taskCh <- &task{block: block}

}

// seal pushes a sealing task to consensus engine and submits the result.
func (m *Miner) seal(t *task, stop <-chan struct{}) {
	var (
		err error
		res *task
	)

	if t.block, err = m.engine.Execute(m.chain, t.block, stop); t.block != nil {
		//logger.Info("Successfully sealed new block", "number", t.block.Number(), "hash", t.block.Hash(),
		//	"elapsed", time.Since(t.createdAt))
		res = t
	} else {
		if err != nil {
			logger.Debug("Block sealing failed", "err", err)
		}
		res = nil
	}

	select {
	case m.resultCh <- res:
	case <-m.quitCh:
	}
}

// taskLoop is a standalone goroutine to fetch sealing task from the generator and
// push them to consensus engine.
func (m *Miner) taskLoop() {
	var (
		stopCh chan struct{}
	)

	// interrupt aborts the in-flight sealing task.
	interrupt := func() {
		if stopCh != nil {
			close(stopCh)
			stopCh = nil
		}
	}
	for {
		select {
		case task := <-m.taskCh:

			// Reject duplicate sealing work due to resubmitting.

			interrupt()
			stopCh = make(chan struct{})
			go m.seal(task, stopCh)
		case <-m.quitCh:
			interrupt()
			return
		}
	}
}
