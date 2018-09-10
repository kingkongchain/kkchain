package miner

import (
	"sync/atomic"

	"github.com/invin/kkchain/consensus"
	"github.com/invin/kkchain/event"

	"github.com/invin/kkchain/common"
	"github.com/invin/kkchain/core"
	"github.com/op/go-logging"
)

var (
	logger = logging.MustGetLogger("miner")
)

type Miner struct {
	quitCh chan struct{}

	worker *worker

	chain *core.BlockChain

	syncDone      int32
	isLocalMining int32

	// sync start event
	syncStartCh  chan struct{}
	syncStartSub event.Subscription
	// sync over event
	syncDoneCh  chan struct{}
	syncDoneSub event.Subscription
}

func New(bc *core.BlockChain, txpool *core.TxPool, engine consensus.Engine) *Miner {
	miner := &Miner{
		quitCh:      make(chan struct{}),
		syncStartCh: make(chan struct{}),
		syncDoneCh:  make(chan struct{}),
		worker:      newWorker(bc, txpool, engine),
		chain:       bc,
	}

	go miner.onEvent()
	return miner
}

func (m *Miner) onEvent() {
	// Subscribe events from syncmgr, now implement in blockchain for test
	//TODO: syncmgr must support subscribe interface
	m.syncStartSub = m.chain.SubscribeSyncStartEvent(m.syncStartCh)
	defer m.syncStartSub.Unsubscribe()

	m.syncDoneSub = m.chain.SubscribeSyncDoneEvent(m.syncDoneCh)
	defer m.syncDoneSub.Unsubscribe()

	for {
		select {
		case <-m.syncStartCh:
			logger.Debug("sync start....")
			if m.Mining() {
				localMining := atomic.LoadInt32(&m.isLocalMining) == 1
				m.Stop()
				//if started before，set isLocalMing to 1
				if localMining {
					atomic.StoreInt32(&m.isLocalMining, 1)
				}
			}
		case <-m.syncDoneCh:
			logger.Debug("sync done....")
			atomic.StoreInt32(&m.syncDone, 1)
			m.syncDoneSub.Unsubscribe()
			if atomic.LoadInt32(&m.isLocalMining) == 1 {
				m.Start()
			}
			//
		case <-m.syncStartSub.Err():
			return
		case <-m.syncDoneSub.Err():
			return
		case <-m.quitCh:
			return
		}
	}
}

// Start start mine service.
func (m *Miner) Start() {
	atomic.StoreInt32(&m.isLocalMining, 1)

	if atomic.LoadInt32(&m.syncDone) == 0 {
		logger.Info("syncing, will start miner afterwards")
		return
	}

	if m.worker.isRunning() {
		logger.Info("mining")
		return
	}

	m.worker.start()
}

// Stop stop mine service.
func (m *Miner) Stop() {
	m.worker.stop()
	atomic.StoreInt32(&m.isLocalMining, 0)
}

func (m *Miner) Close() {
	m.worker.close()
	close(m.quitCh)
}

func (m *Miner) Mining() bool {
	return m.worker.isRunning()
}

func (m *Miner) SetMiner(addr common.Address) {
	m.worker.setMiner(addr)
}
