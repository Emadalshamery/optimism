package contracts

import (
	"context"

	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/contracts/metrics"
	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/types"
	"github.com/ethereum-optimism/optimism/op-service/sources/batching"
	"github.com/ethereum-optimism/optimism/op-service/txmgr"
	"github.com/ethereum-optimism/optimism/packages/contracts-bedrock/snapshots"
	"github.com/ethereum/go-ethereum/common"
)

var (
	methodL2SequenceNumber = "l2SequenceNumber"
)

func NewSuperFaultDisputeGameContract(ctx context.Context, metrics metrics.ContractMetricer, addr common.Address, caller *batching.MultiCaller) (FaultDisputeGameContract, error) {
	contractAbi := snapshots.LoadFaultDisputeGameABI()
	contract := batching.NewBoundContract(contractAbi, addr)
	ops := latestFaultDisputeGameOps()
	ops.GetL2BlockNumberChallenged = func(contract *batching.BoundContract) GetterContractOp[bool] {
		return NewStaticOp(false)
	}
	ops.GetL2BlockNumberChallenger = func(contract *batching.BoundContract) GetterContractOp[common.Address] {
		return NewStaticOp(common.Address{})
	}
	ops.GetL2SequenceNumber = func(contract *batching.BoundContract) GetterContractOp[uint64] {
		return NewSimpleGetterOp(contract, methodL2SequenceNumber, func(result *batching.CallResult) (uint64, error) {
			return result.GetBigInt(0).Uint64(), nil
		})
	}
	ops.ChallengeL2BlockNumberTx = func(_ *batching.BoundContract, _ *types.InvalidL2BlockNumberChallenge) (txmgr.TxCandidate, error) {
		return txmgr.TxCandidate{}, ErrChallengeL2BlockNotSupported
	}
	return &FaultDisputeGameContractLatest{
		metrics:     metrics,
		multiCaller: caller,
		contract:    contract,
		ops:         ops,
	}, nil
}
