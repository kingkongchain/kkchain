package miner

import (
	"fmt"
	"sync/atomic"
	"time"

	"github.com/invin/kkchain/common"
	"github.com/invin/kkchain/consensus"
	"github.com/invin/kkchain/core"
	"github.com/invin/kkchain/core/types"
	"github.com/invin/kkchain/event"
	"sync"
)

// task contains all information for consensus engine sealing and result submitting.
type task struct {
	block     *types.Block
	createdAt time.Time
}

type worker struct {
	startCh chan struct{}
	quitCh  chan struct{}
	miner   common.Address

	running int32

	mu *sync.RWMutex // The lock used to protect the coinbase and extra fields

	//TODO: blockchain impl ChainReader interface
	chain  *core.BlockChain
	engine consensus.Engine

	mineLoopCh chan struct{}

	//tx pool add new txs
	txsCh  chan []*types.Transaction
	txsSub event.Subscription

	//new block inserted to chain
	newBlockCh  chan *types.Block
	newBlockSub event.Subscription

	taskCh   chan *task
	resultCh chan *task
}

func newWorker(bc *core.BlockChain, engine consensus.Engine) *worker {
	w := &worker{
		startCh:    make(chan struct{}, 1),
		quitCh:     make(chan struct{}),
		mu:         &sync.RWMutex{},
		chain:      bc,
		engine:     engine,
		txsCh:      make(chan []*types.Transaction),
		newBlockCh: make(chan *types.Block),
		mineLoopCh: make(chan struct{}),
		taskCh:     make(chan *task),
		resultCh:   make(chan *task),
	}

	// Subscribe events from tx pool
	w.txsSub = bc.SubscribeTxsEvent(w.txsCh)

	// Subscribe events from inbound handler
	w.newBlockSub = bc.SubscribeNewBlockEvent(w.newBlockCh)

	go w.mineLoop()
	go w.taskLoop()
	go w.waitResult()

	return w
}

// setMiner sets the miner used to initialize the block miner field.
func (w *worker) setMiner(addr common.Address) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.miner = addr
}

func (w *worker) mineLoop() {
	defer w.txsSub.Unsubscribe()
	defer w.newBlockSub.Unsubscribe()

	for {
		select {
		case txs := <-w.txsCh:
			for _, tx := range txs {
				fmt.Println(tx)
			}
		case <-w.newBlockCh:
			if w.isRunning() {
				w.commitTask()
			}
		case <-w.mineLoopCh:
			//Start new mine task
			w.commitTask()
		case <-w.startCh:
			w.commitTask()
		case <-w.quitCh:
			//Stopped Mining
			return
		}
	}

}

// start sets the running status as 1 and triggers new work submitting.
func (w *worker) start() {
	atomic.StoreInt32(&w.running, 1)
	//start first mining
	w.startCh <- struct{}{}
}

// stop sets the running status as 0.
func (w *worker) stop() {
	atomic.StoreInt32(&w.running, 0)
}

// isRunning returns an indicator whether worker is running or not.
func (w *worker) isRunning() bool {
	return atomic.LoadInt32(&w.running) == 1
}

// close terminates all background threads maintained by the worker and cleans up buffered channels.
// Note the worker does not support being closed multiple times.
func (w *worker) close() {
	close(w.quitCh)
	// Clean up buffered channels
	for empty := false; !empty; {
		select {
		case <-w.resultCh:
		default:
			empty = true
		}
	}
}

func (w *worker) waitResult() {
	for {
		logger.Debug("waitResult....")
		select {
		case result := <-w.resultCh:
			// Short circuit when receiving empty result.
			if result == nil {
				continue
			}

			logger.Info("result=> number: %d, nonce: 0x%x", result.block.Number(), result.block.Nonce())

			// Short circuit when receiving duplicate result caused by resubmitting.

			// Update the block hash in all logs since it is now available and not when the
			// receipt/log of individual transactions were created.

			// Commit block and state to database.

			// Broadcast the block and announce chain insertion event

			//trigger next mine loop
			w.chain.PostNewBlockEvent(result.block)
		case <-w.quitCh:
			return

		}
	}
}

func (w *worker) commitTask() {
	w.mu.RLock()
	defer w.mu.RUnlock()
	logger.Debug("commitTask...")
	if !w.isRunning() {
		return
	}

	//get txs from pending pool
	txs := []types.Transaction{}

	//Initialize ctx
	block, _ := w.engine.Initialize(w.chain, txs)

	//commit new task
	w.taskCh <- &task{block: block, createdAt: time.Now()}

}

// taskLoop is a standalone goroutine to fetch sealing task from the generator and
// push them to consensus engine.
func (w *worker) taskLoop() {
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
		logger.Debug("taskLoop....")
		select {
		case task := <-w.taskCh:

			// Reject duplicate sealing work due to resubmitting.

			interrupt()
			stopCh = make(chan struct{})
			go w.seal(task, stopCh)
		case <-w.quitCh:
			interrupt()
			return
		}
	}
}

// seal pushes a sealing task to consensus engine and submits the result.
func (w *worker) seal(t *task, stop <-chan struct{}) {
	var (
		err error
		res *task
	)

	if t.block, err = w.engine.Execute(w.chain, t.block, stop); t.block != nil {
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
	case w.resultCh <- res:
	case <-w.quitCh:
	}
}
