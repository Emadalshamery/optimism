package l1builder

import (
	"context"
	"encoding/binary"
	"errors"
	"math/big"
	"math/rand"
	"sync"
	"time"

	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils/fakebeacon"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils/geth"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	opeth "github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/testutils"
	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/backend/work"
	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/seqtypes"
	"github.com/ethereum/go-ethereum/beacon/engine"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/eth/catalyst"
	"github.com/ethereum/go-ethereum/log"
)

// TODO: remove from globals
var blocksByParent map[common.Hash]*engine.ExecutionPayloadEnvelope

type L1Envelope struct {
	engine.ExecutionPayloadEnvelope
}

func (e *L1Envelope) ID() eth.BlockID {
	return eth.BlockID{Hash: e.ExecutionPayload.BlockHash, Number: e.ExecutionPayload.Number}
}

func (e *L1Envelope) String() string {
	return e.ID().String()
}

type Job struct {
	id seqtypes.BuildJobID

	engine *catalyst.ConsensusAPI
	geth   *geth.GethInstance
	beacon *fakebeacon.FakeBeacon

	mu     sync.Mutex
	logger log.Logger

	parent common.Hash

	withdrawalsIndex *uint64

	envelope engine.ExecutionPayloadEnvelope
}

func (job *Job) ID() seqtypes.BuildJobID {
	return job.id
}

func (job *Job) Cancel(ctx context.Context) error {
	return nil
}

func (job *Job) Open(ctx context.Context) error {
	job.mu.Lock()
	defer job.mu.Unlock()

	job.logger.Info("Open job", "id", job.id)

	if blocksByParent == nil {
		blocksByParent = make(map[common.Hash]*engine.ExecutionPayloadEnvelope)
	}

	// TODO: move to config
	finalizedDistance := uint64(3)
	safeDistance := uint64(4)
	blockTime := 6

	eth := job.geth.Backend
	now := time.Now()

	chain := eth.BlockChain()
	head := chain.CurrentBlock() // default head
	if job.parent != (common.Hash{}) {
		head = eth.BlockChain().GetHeaderByHash(job.parent) // override head if parent is set
	}

	var parentBeaconBlockRoot common.Hash
	var isCancun bool
	var isPrague bool

	finalized := chain.CurrentFinalBlock()
	if finalized == nil { // fallback to genesis if nothing is finalized
		finalized = chain.Genesis().Header()
	}
	safe := chain.CurrentSafeBlock()
	if safe == nil { // fallback to finalized if nothing is safe
		safe = finalized
	}
	if head.Number.Uint64() > finalizedDistance { // progress finalized block, if we can
		finalized = eth.BlockChain().GetHeaderByNumber(head.Number.Uint64() - finalizedDistance)
	}
	if head.Number.Uint64() > safeDistance { // progress safe block, if we can
		safe = eth.BlockChain().GetHeaderByNumber(head.Number.Uint64() - safeDistance)
	}

	var withdrawals []*types.Withdrawal
	var envelope *engine.ExecutionPayloadEnvelope
	var ok bool
	envelope, ok = blocksByParent[head.Hash()]
	if !ok { // we haven't build a block with this parent yet, so we need to build one

		newBlockTime := head.Time + uint64(blockTime)
		if time.Unix(int64(newBlockTime), 0).Add(5 * time.Minute).Before(now) {
			// We're a long way behind, let's skip some blocks...
			newBlockTime = uint64(now.Unix())
		}
		// create some random withdrawals
		withdrawalsRNG := rand.New(rand.NewSource(time.Now().UnixNano()))
		withdrawals = make([]*types.Withdrawal, withdrawalsRNG.Intn(4))
		for i := 0; i < len(withdrawals); i++ {
			withdrawals[i] = &types.Withdrawal{
				Index:     *job.withdrawalsIndex + uint64(i),
				Validator: withdrawalsRNG.Uint64() % 100_000_000, // 100 million fake validators
				Address:   testutils.RandomAddress(withdrawalsRNG),
				// in gwei, consensus-layer quirk. withdraw non-zero value up to 50 ETH
				Amount: uint64(withdrawalsRNG.Intn(50_000_000_000) + 1),
			}
		}
		attrs := &engine.PayloadAttributes{
			Timestamp:             newBlockTime,
			Random:                common.Hash{},
			SuggestedFeeRecipient: head.Coinbase,
			Withdrawals:           withdrawals,
		}
		parentBeaconBlockRoot = fakeBeaconBlockRoot(head.Time) // parent beacon block root
		isCancun = eth.BlockChain().Config().IsCancun(new(big.Int).SetUint64(head.Number.Uint64()+1), newBlockTime)
		isPrague = eth.BlockChain().Config().IsPrague(new(big.Int).SetUint64(head.Number.Uint64()+1), newBlockTime)
		if isCancun {
			attrs.BeaconRoot = &parentBeaconBlockRoot
		}
		fcState := engine.ForkchoiceStateV1{
			HeadBlockHash:      head.Hash(),
			SafeBlockHash:      safe.Hash(),
			FinalizedBlockHash: finalized.Hash(),
		}
		job.logger.Info("ForkchoiceUpdatedV3", "isCancun", isCancun, "isPrague", isPrague, "fcState", fcState)
		var err error
		var res engine.ForkChoiceResponse
		if isCancun {
			res, err = job.engine.ForkchoiceUpdatedV3(fcState, attrs)
		} else {
			res, err = job.engine.ForkchoiceUpdatedV2(fcState, attrs)
		}
		if err != nil {
			job.logger.Error("failed to start building L1 block", "err", err)
			return err
		}
		if res.PayloadID == nil {
			job.logger.Error("failed to start block building", "res", res)
			return errors.New("failed to start block building")
		}

		job.logger.Info("got res.payloadID", "res.payloadID", res.PayloadID)

		if isPrague {
			envelope, err = job.engine.GetPayloadV4(*res.PayloadID)
		} else if isCancun {
			envelope, err = job.engine.GetPayloadV3(*res.PayloadID)
		} else {
			envelope, err = job.engine.GetPayloadV2(*res.PayloadID)
		}
		if err != nil {
			job.logger.Error("failed to finish building L1 block", "err", err)
			return err
		}

		blocksByParent[envelope.ExecutionPayload.ParentHash] = envelope
	} else {
		job.logger.Warn("already had a block with that parent", "parent", head.Hash(), "number", head.Number.Uint64())
		envelope.ExecutionPayload.FeeRecipient = common.HexToAddress("0x101") // update fee recipient to a random address so that we trigger a reorg

		parentBeaconBlockRoot = fakeBeaconBlockRoot(head.Time) // parent beacon block root
		isCancun = eth.BlockChain().Config().IsCancun(new(big.Int).SetUint64(head.Number.Uint64()+1), envelope.ExecutionPayload.Timestamp)
		isPrague = eth.BlockChain().Config().IsPrague(new(big.Int).SetUint64(head.Number.Uint64()+1), envelope.ExecutionPayload.Timestamp)
		withdrawals = envelope.ExecutionPayload.Withdrawals
		job.logger.Warn("updating block hash", "pre", envelope.ExecutionPayload.BlockHash)
		envelope.ExecutionPayload.Transactions = make([][]byte, 0) // removing txs just in case
		block, err := engine.ExecutableDataToBlockNoHash(*envelope.ExecutionPayload, make([]common.Hash, 0), &parentBeaconBlockRoot, make([][]byte, 0), job.geth.Backend.BlockChain().Config())
		if err != nil {
			job.logger.Error("failed to convert executable data to block", "err", err)
			return err
		}
		envelope.ExecutionPayload.BlockHash = block.Hash()
		// TODO: maybe remove txs or at least replace them, so that we certainly reorg L2 DA txs
		job.logger.Warn("updated block hash", "post", envelope.ExecutionPayload.BlockHash)
	}

	job.logger.Info("final envelope", "envelope", envelope)

	blobHashes := make([]common.Hash, 0) // must be non-nil even when empty, due to geth engine API checks
	if envelope.BlobsBundle != nil {
		for _, commitment := range envelope.BlobsBundle.Commitments {
			if len(commitment) != 48 {
				job.logger.Error("got malformed kzg commitment from engine", "commitment", commitment)
				break
			}
			blobHashes = append(blobHashes, opeth.KZGToVersionedHash(*(*[48]byte)(commitment)))
		}
		if len(blobHashes) != len(envelope.BlobsBundle.Commitments) {
			job.logger.Error("invalid or incomplete blob data", "collected", len(blobHashes), "engine", len(envelope.BlobsBundle.Commitments))
			return errors.New("invalid or incomplete blob data")
		}
	}

	job.logger.Info("about to insert payload into the chain", "envelope-hash", envelope.ExecutionPayload.BlockHash, "txs", len(envelope.ExecutionPayload.Transactions))

	var err error
	if isPrague {
		_, err = job.engine.NewPayloadV4(*envelope.ExecutionPayload, blobHashes, &parentBeaconBlockRoot, make([]hexutil.Bytes, 0))
	} else if isCancun {
		_, err = job.engine.NewPayloadV3(*envelope.ExecutionPayload, blobHashes, &parentBeaconBlockRoot)
	} else {
		_, err = job.engine.NewPayloadV2(*envelope.ExecutionPayload)
	}
	if err != nil {
		job.logger.Error("failed to insert built L1 block", "err", err)
		return err
	}

	if envelope.BlobsBundle != nil {
		slot := (envelope.ExecutionPayload.Timestamp - eth.BlockChain().Genesis().Time()) / uint64(blockTime)
		if job.beacon == nil {
			job.logger.Error("no blobs storage available")
			return errors.New("no blobs storage available")
		}
		if err := job.beacon.StoreBlobsBundle(slot, envelope.BlobsBundle); err != nil {
			job.logger.Error("failed to persist blobs-bundle of block, not making block canonical now", "err", err)
			return err
		}
	}
	job.logger.Info("about to forkchoice update", "safe", safe.Hash(), "finalized", finalized.Hash(), "head", envelope.ExecutionPayload.BlockHash)

	if _, err := job.engine.ForkchoiceUpdatedV3(engine.ForkchoiceStateV1{
		HeadBlockHash:      envelope.ExecutionPayload.BlockHash,
		SafeBlockHash:      safe.Hash(),
		FinalizedBlockHash: finalized.Hash(),
	}, nil); err != nil {
		job.logger.Error("failed to make built L1 block canonical", "err", err)
		return err
	}

	*job.withdrawalsIndex += uint64(len(withdrawals))

	job.logger.Info("incrementing withdrawals index", "index", *job.withdrawalsIndex)

	job.envelope = *envelope

	return nil
}

func (job *Job) Seal(ctx context.Context) (work.Block, error) {
	job.mu.Lock()
	defer job.mu.Unlock()

	job.logger.Debug("Wait for block to be sealed")

	return &L1Envelope{ExecutionPayloadEnvelope: job.envelope}, nil
}

func (job *Job) String() string {
	return job.id.String()
}

func (job *Job) Close() {
}

func (job *Job) IncludeTx(ctx context.Context, tx hexutil.Bytes) error {
	return errors.New("not implemented")
}

var _ work.BuildJob = (*Job)(nil)

func fakeBeaconBlockRoot(time uint64) common.Hash {
	var dat [8]byte
	binary.LittleEndian.PutUint64(dat[:], time)
	return crypto.Keccak256Hash(dat[:])
}
