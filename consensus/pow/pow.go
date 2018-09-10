package pow

import (
	crand "crypto/rand"
	"math"
	"math/big"
	"math/rand"
	"runtime"
	"sync"

	"github.com/invin/kkchain/common"
	cmath "github.com/invin/kkchain/common/math"
	"github.com/invin/kkchain/consensus"
	"github.com/invin/kkchain/core/state"
	"github.com/invin/kkchain/core/types"

	log "github.com/sirupsen/logrus"
)

var (
	FrontierBlockReward *big.Int = big.NewInt(5e+18) // Block reward in wei for successfully mining a block

)

func (ethash *Ethash) Initialize(chain consensus.ChainReader, header *types.Header) error {
	parent := chain.GetHeader(header.ParentHash, header.Number.Uint64()-1)
	if parent == nil {
		return consensus.ErrUnknownAncestor
	}

	header.Difficulty = calcDifficultyFrontier(header.Time.Uint64(), parent)
	return nil
}

func (ethash *Ethash) Finalize(chain consensus.ChainReader, state *state.StateDB, block *types.Block) error {
	header := block.HeaderWithoutCopy()
	// Accumulate any block and uncle rewards and commit the final state root
	accumulateRewards(state, header)

	header.StateRoot = state.IntermediateRoot(false)

	return nil
}

func (ethash *Ethash) Execute(chain consensus.ChainReader, block *types.Block, stop <-chan struct{}) (*types.Block, error) {
	return ethash.Seal(chain, block, stop)
}

func (ethash *Ethash) PostExecute(chain consensus.ChainReader, block *types.Block) error {
	return nil
}

func (ethash *Ethash) VerifyHeader(header *types.Header) error {
	return nil
}

func (ethash *Ethash) Verify(block *types.Block) error {
	return nil
}

// Seal implements consensus.Engine, attempting to find a nonce that satisfies
// the block's difficulty requirements.
func (ethash *Ethash) Seal(chain consensus.ChainReader, block *types.Block, stop <-chan struct{}) (*types.Block, error) {
	// If we're running a fake PoW, simply return a 0 nonce immediately
	if ethash.config.PowMode == ModeFake || ethash.config.PowMode == ModeFullFake {
		header := block.Header()
		header.Nonce, header.MixDigest = types.BlockNonce{}, common.Hash{}
		return block.WithSeal(header), nil
	}
	// If we're running a shared PoW, delegate sealing to it
	if ethash.shared != nil {
		return ethash.shared.Seal(chain, block, stop)
	}
	// Create a runner and the multiple search threads it directs
	abort := make(chan struct{})

	ethash.lock.Lock()
	threads := ethash.threads
	if ethash.rand == nil {
		seed, err := crand.Int(crand.Reader, big.NewInt(math.MaxInt64))
		if err != nil {
			ethash.lock.Unlock()
			return nil, err
		}
		ethash.rand = rand.New(rand.NewSource(seed.Int64()))
	}
	ethash.lock.Unlock()
	if threads == 0 {
		threads = runtime.NumCPU()
	}
	if threads < 0 {
		threads = 0 // Allows disabling local mining without extra logic around local/remote
	}
	// Push new work to remote sealer
	//if ethash.workCh != nil {
	//	ethash.workCh <- block
	//}
	var pend sync.WaitGroup
	for i := 0; i < threads; i++ {
		pend.Add(1)
		go func(id int, nonce uint64) {
			defer pend.Done()
			ethash.mine(block, id, nonce, abort, ethash.resultCh)
		}(i, uint64(ethash.rand.Int63()))
	}
	// Wait until sealing is terminated or a nonce is found
	var result *types.Block
	select {
	case <-stop:
		// Outside abort, stop all miner threads
		close(abort)
	case result = <-ethash.resultCh:
		// One of the threads found a block, abort all others
		close(abort)
	case <-ethash.update:
		// Thread count was changed on user request, restart
		close(abort)
		pend.Wait()
		return ethash.Seal(chain, block, stop)
	}
	// Wait for all miners to terminate and return the block
	pend.Wait()
	return result, nil
}

// mine is the actual proof-of-work miner that searches for a nonce starting from
// seed that results in correct final block difficulty.
func (ethash *Ethash) mine(block *types.Block, id int, seed uint64, abort chan struct{}, found chan *types.Block) {
	// Extract some data from the header
	var (
		header  = block.Header()
		hash    = header.HashNoNonce().Bytes()
		target  = new(big.Int).Div(two256, header.Difficulty)
		number  = header.Number.Uint64()
		dataset = ethash.dataset(number, false)
	)
	// Start generating random nonces until we abort or find a good one
	var (
		attempts = int64(0)
		nonce    = seed
	)
	//logger := log.New("miner", id)
	log.Debug("Started ethash search for new nonces", "seed", seed)
search:
	for {
		select {
		case <-abort:
			// Mining terminated, update stats and abort
			log.Debug("Ethash nonce search aborted", "attempts", nonce-seed)
			//ethash.hashrate.Mark(attempts)
			break search

		default:
			// We don't have to update hash rate on every nonce, so update after after 2^X nonces
			attempts++
			if (attempts % (1 << 15)) == 0 {
				//ethash.hashrate.Mark(attempts)
				attempts = 0
			}
			// Compute the PoW value of this nonce
			digest, result := hashimotoFull(dataset.dataset, hash, nonce)
			if new(big.Int).SetBytes(result).Cmp(target) <= 0 {
				// Correct nonce found, create a new header with it
				header = types.CopyHeader(header)
				header.Nonce = types.EncodeNonce(nonce)
				header.MixDigest = common.BytesToHash(digest)

				// Seal and return a block (if still needed)
				select {
				case found <- block.WithSeal(header):
					log.Debug("Ethash nonce found and reported", "attempts", nonce-seed, "nonce", nonce)
				case <-abort:
					log.Debug("Ethash nonce found but discarded", "attempts", nonce-seed, "nonce", nonce)
				}
				break search
			}
			nonce++
		}
	}
	// Datasets are unmapped in a finalizer. Ensure that the dataset stays live
	// during sealing so it's not unmapped while being read.
	runtime.KeepAlive(dataset)
}

// calcDifficultyFrontier is the difficulty adjustment algorithm. It returns the
// difficulty that a new block should have when created at time given the parent
// block's time and difficulty. The calculation uses the Frontier rules.
func calcDifficultyFrontier(time uint64, parent *types.Header) *big.Int {

	var (
		expDiffPeriod          = big.NewInt(100000)
		DifficultyBoundDivisor = big.NewInt(2048)   // The bound divisor of the difficulty, used in the update calculations.
		MinimumDifficulty      = big.NewInt(131072) // The minimum that the difficulty may ever be.
		DurationLimit          = big.NewInt(13)     // The decision boundary on the blocktime duration used to determine whether difficulty should go up or not.
	)

	diff := new(big.Int)
	adjust := new(big.Int).Div(parent.Difficulty, DifficultyBoundDivisor)
	bigTime := new(big.Int)
	bigParentTime := new(big.Int)

	bigTime.SetUint64(time)
	bigParentTime.Set(parent.Time)

	if bigTime.Sub(bigTime, bigParentTime).Cmp(DurationLimit) < 0 {
		diff.Add(parent.Difficulty, adjust)
	} else {
		diff.Sub(parent.Difficulty, adjust)
	}
	if diff.Cmp(MinimumDifficulty) < 0 {
		diff.Set(MinimumDifficulty)
	}

	periodCount := new(big.Int).Add(parent.Number, common.Big1)
	periodCount.Div(periodCount, expDiffPeriod)
	if periodCount.Cmp(common.Big1) > 0 {
		// diff = diff + 2^(periodCount - 2)
		expDiff := periodCount.Sub(periodCount, common.Big2)
		expDiff.Exp(common.Big2, expDiff, nil)
		diff.Add(diff, expDiff)
		diff = cmath.BigMax(diff, MinimumDifficulty)
	}
	return diff
}

// AccumulateRewards credits the coinbase of the given block with the mining
// reward. The total reward consists of the static block reward and rewards for
// included uncles. The coinbase of each uncle block is also rewarded.
func accumulateRewards(state *state.StateDB, header *types.Header) {
	// Select the correct block reward based on chain progression
	blockReward := FrontierBlockReward

	// Accumulate the rewards for the miner and any included uncles
	reward := new(big.Int).Set(blockReward)
	state.AddBalance(header.Miner, reward)
}
