package miner

import (
	"fmt"
	"github.com/invin/kkchain/consensus"
	"github.com/invin/kkchain/types"
)

type Miner struct {
	quitCh chan bool

	coinbase []byte //address

	engine consensus.Engine

	mineLoopCh chan interface{}

	resultCh chan *types.Block
}

func New() *Miner {
	miner := &Miner{}

	return miner
}

// Start start mine service.
func (m *Miner) Start() {
	//Starting Mining...
	go m.mineLoop(m.mineLoopCh)
	go m.waitResult()

}

// Stop stop mine service.
func (m *Miner) Stop() {
	//Stopping  Mining...
	m.quitCh <- true
}

func (m *Miner) mineLoop(newLoop <-chan interface{}) {

	for {
		select {
		case now := <-newLoop:
			fmt.Println(now)
			//Start new mine task
			m.restartMining()
		case <-m.quitCh:
			//Stopped Mining
			return
		}
	}
}

func (m *Miner) restartMining() {
	//get txs from pending pool
	txs := []types.Transaction{}

	//Initialize ctx
	block, _ := m.engine.Initialize(nil, txs)

	//start to settle and wait settled channel
	settleCh := make(chan *types.Block)
	go m.waitSettled(settleCh)
	m.engine.Execute(nil, block, settleCh)
}

func (m *Miner) waitSettled(settleCh <-chan *types.Block) {

	select {
	case newBlock := <-settleCh:
		fmt.Println(newBlock)
		//Finalize if needed, like: update LIB etc

		//return result
		m.resultCh <- newBlock
	}

}

func (m *Miner) waitResult() {
	for {
		for result := range m.resultCh {
			fmt.Println(result)
			//state changes

			//trigger next mine loop
			m.mineLoopCh <- result
		}
	}

}
