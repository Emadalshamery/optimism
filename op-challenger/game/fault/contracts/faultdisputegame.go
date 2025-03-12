package contracts

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math"
	"math/big"
	"time"

	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/contracts/metrics"
	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/types"
	gameTypes "github.com/ethereum-optimism/optimism/op-challenger/game/types"
	"github.com/ethereum-optimism/optimism/op-service/sources/batching"
	"github.com/ethereum-optimism/optimism/op-service/sources/batching/rpcblock"
	"github.com/ethereum-optimism/optimism/op-service/txmgr"
	"github.com/ethereum-optimism/optimism/packages/contracts-bedrock/snapshots"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
)

// The maximum number of children that will be processed during a call to `resolveClaim`
var maxChildChecks = big.NewInt(512)

var (
	methodMaxClockDuration        = "maxClockDuration"
	methodMaxGameDepth            = "maxGameDepth"
	methodAbsolutePrestate        = "absolutePrestate"
	methodStatus                  = "status"
	methodRootClaim               = "rootClaim"
	methodClaimCount              = "claimDataLen"
	methodClaim                   = "claimData"
	methodL1Head                  = "l1Head"
	methodResolvedAt              = "resolvedAt"
	methodResolvedSubgames        = "resolvedSubgames"
	methodResolve                 = "resolve"
	methodResolveClaim            = "resolveClaim"
	methodAttack                  = "attack"
	methodDefend                  = "defend"
	methodStep                    = "step"
	methodAddLocalData            = "addLocalData"
	methodVM                      = "vm"
	methodStartingBlockNumber     = "startingBlockNumber"
	methodStartingRootHash        = "startingRootHash"
	methodSplitDepth              = "splitDepth"
	methodL2BlockNumber           = "l2BlockNumber"
	methodRequiredBond            = "getRequiredBond"
	methodClaimCredit             = "claimCredit"
	methodCredit                  = "credit"
	methodWETH                    = "weth"
	methodL2BlockNumberChallenged = "l2BlockNumberChallenged"
	methodL2BlockNumberChallenger = "l2BlockNumberChallenger"
	methodChallengeRootL2Block    = "challengeRootL2Block"
	methodBondDistributionMode    = "bondDistributionMode"
)

var (
	ErrSimulationFailed             = errors.New("tx simulation failed")
	ErrChallengeL2BlockNotSupported = errors.New("contract version does not support challenging L2 block number")
)

type FaultDisputeGameContractLatest struct {
	metrics     metrics.ContractMetricer
	multiCaller *batching.MultiCaller
	contract    *batching.BoundContract

	ops *faultDisputeGameOps
}

// outputRootProof is designed to match the solidity OutputRootProof struct.
type outputRootProof struct {
	Version                  [32]byte
	StateRoot                [32]byte
	MessagePasserStorageRoot [32]byte
	LatestBlockhash          [32]byte
}

/*
f.contract.Call(methodL1Head),
f.contract.Call(methodL2BlockNumber),
f.contract.Call(methodRootClaim),
f.contract.Call(methodStatus),
f.contract.Call(methodMaxClockDuration),
f.contract.Call(methodL2BlockNumberChallenged),
f.contract.Call(methodL2BlockNumberChallenger),
*/
type faultDisputeGameOps struct {
	GetClaim                   func(contract *batching.BoundContract, idx uint64) GetterContractOp[types.Claim]
	GetL1Head                  func(contract *batching.BoundContract) GetterContractOp[common.Hash]
	GetL2SequenceNumber        func(contract *batching.BoundContract) GetterContractOp[uint64]
	GetRootClaim               func(contract *batching.BoundContract) GetterContractOp[common.Hash]
	GetStatus                  func(contract *batching.BoundContract) GetterContractOp[gameTypes.GameStatus]
	GetMaxClockDuration        func(contract *batching.BoundContract) GetterContractOp[uint64]
	GetL2BlockNumberChallenged func(contract *batching.BoundContract) GetterContractOp[bool]
	GetL2BlockNumberChallenger func(contract *batching.BoundContract) GetterContractOp[common.Address]

	ChallengeL2BlockNumberTx func(contract *batching.BoundContract, challenge *types.InvalidL2BlockNumberChallenge) (txmgr.TxCandidate, error)
}

func latestFaultDisputeGameOps() *faultDisputeGameOps {
	return &faultDisputeGameOps{
		GetClaim: func(contract *batching.BoundContract, idx uint64) GetterContractOp[types.Claim] {
			return NewGetClaimOp(contract, idx)
		},
		GetL1Head: func(contract *batching.BoundContract) GetterContractOp[common.Hash] {
			return NewSimpleGetterOp(contract, methodL1Head, func(result *batching.CallResult) (common.Hash, error) {
				return result.GetHash(0), nil
			})
		},
		GetL2SequenceNumber: func(contract *batching.BoundContract) GetterContractOp[uint64] {
			return NewSimpleGetterOp(contract, methodL2BlockNumber, func(result *batching.CallResult) (uint64, error) {
				return result.GetBigInt(0).Uint64(), nil
			})
		},
		GetRootClaim: func(contract *batching.BoundContract) GetterContractOp[common.Hash] {
			return NewSimpleGetterOp(contract, methodRootClaim, func(result *batching.CallResult) (common.Hash, error) {
				return result.GetHash(0), nil
			})
		},
		GetStatus: func(contract *batching.BoundContract) GetterContractOp[gameTypes.GameStatus] {
			return NewSimpleGetterOp(contract, methodStatus, func(result *batching.CallResult) (gameTypes.GameStatus, error) {
				statusCode := result.GetUint8(0)
				return gameTypes.GameStatusFromUint8(statusCode)
			})
		},
		GetMaxClockDuration: func(contract *batching.BoundContract) GetterContractOp[uint64] {
			return NewSimpleGetterOp(contract, methodMaxClockDuration, func(result *batching.CallResult) (uint64, error) {
				return result.GetUint64(0), nil
			})
		},
		GetL2BlockNumberChallenged: func(contract *batching.BoundContract) GetterContractOp[bool] {
			return NewSimpleGetterOp(contract, methodL2BlockNumberChallenged, func(result *batching.CallResult) (bool, error) {
				return result.GetBool(0), nil
			})
		},
		GetL2BlockNumberChallenger: func(contract *batching.BoundContract) GetterContractOp[common.Address] {
			return NewSimpleGetterOp(contract, methodL2BlockNumberChallenger, func(result *batching.CallResult) (common.Address, error) {
				return result.GetAddress(0), nil
			})
		},
		ChallengeL2BlockNumberTx: func(contract *batching.BoundContract, challenge *types.InvalidL2BlockNumberChallenge) (txmgr.TxCandidate, error) {
			headerRlp, err := rlp.EncodeToBytes(challenge.Header)
			if err != nil {
				return txmgr.TxCandidate{}, fmt.Errorf("failed to serialize header: %w", err)
			}
			return contract.Call(methodChallengeRootL2Block, outputRootProof{
				Version:                  challenge.Output.Version,
				StateRoot:                challenge.Output.StateRoot,
				MessagePasserStorageRoot: challenge.Output.WithdrawalStorageRoot,
				LatestBlockhash:          challenge.Output.BlockRef.Hash,
			}, headerRlp).ToTxCandidate()
		},
	}
}

func NewFaultDisputeGameContract(ctx context.Context, metrics metrics.ContractMetricer, addr common.Address, caller *batching.MultiCaller) (FaultDisputeGameContract, error) {
	gameType, err := DetectGameType(ctx, addr, caller)
	if err != nil {
		return nil, fmt.Errorf("failed to detect game type: %w", err)
	}
	switch gameType {
	case types.SuperCannonGameType, types.SuperPermissionedGameType:
		return NewSuperFaultDisputeGameContract(ctx, metrics, addr, caller)
	default:
		return NewPreInteropFaultDisputeGameContract(ctx, metrics, addr, caller)
	}
}

func NewPreInteropFaultDisputeGameContract(ctx context.Context, metrics metrics.ContractMetricer, addr common.Address, caller *batching.MultiCaller) (FaultDisputeGameContract, error) {
	contractAbi := snapshots.LoadFaultDisputeGameABI()
	ops := latestFaultDisputeGameOps()

	var builder VersionedBuilder[FaultDisputeGameContract]
	builder.AddVersion(0, 8, func() (FaultDisputeGameContract, error) {
		legacyAbi := mustParseAbi(faultDisputeGameAbi020)
		// Replace GetClaim with a version that also restores the original bond amount even for resolved claims.
		// We could make these overrides reusable as well, since 0.8.0 typically has all the overrides from 0.18.0
		// plus its unique changes. So we could start by applying the ops changes from 0.18.0 then only need to
		// apply 0.8.0 specific changes here.
		ops.GetClaim = func(contract *batching.BoundContract, idx uint64) GetterContractOp[types.Claim] {
			getClaimOp := NewGetClaimOp(contract, idx)
			fixBondOp := &FixBondOp{getClaimOp}
			op := ChainOps(getClaimOp.Result, getClaimOp, fixBondOp)
			return op
		}
		ops.GetMaxClockDuration = func(contract *batching.BoundContract) GetterContractOp[uint64] {
			return NewSimpleGetterOp(contract, methodGameDuration, func(result *batching.CallResult) (uint64, error) {
				return result.GetUint64(0) / 2, nil
			})
		}
		ops.GetL2BlockNumberChallenged = func(contract *batching.BoundContract) GetterContractOp[bool] {
			return NewStaticOp(false)
		}
		ops.GetL2BlockNumberChallenger = func(contract *batching.BoundContract) GetterContractOp[common.Address] {
			return NewStaticOp(common.Address{})
		}
		// Note: If we made everything that needs to be overridden a ContractOp, we wouldn't need
		// FaultDisputeGameContract080, we'd just use FaultDisputeGameContractLatest all the time.
		// And we could potentially then remove the FaultDisputeGame interface as well, since the ops could be
		// replaced with ones that return static results for mocking.
		return &FaultDisputeGameContract080{
			FaultDisputeGameContractLatest: FaultDisputeGameContractLatest{
				metrics:     metrics,
				multiCaller: caller,
				contract:    batching.NewBoundContract(legacyAbi, addr),
				ops:         ops,
			},
		}, nil
	})
	builder.AddVersion(0, 18, func() (FaultDisputeGameContract, error) {
		legacyAbi := mustParseAbi(faultDisputeGameAbi0180)
		return &FaultDisputeGameContract0180{
			FaultDisputeGameContractLatest: FaultDisputeGameContractLatest{
				metrics:     metrics,
				multiCaller: caller,
				contract:    batching.NewBoundContract(legacyAbi, addr),
				ops:         ops,
			},
		}, nil
	})
	builder.AddVersion(1, 0, func() (FaultDisputeGameContract, error) {
		legacyAbi := mustParseAbi(faultDisputeGameAbi0180)
		return &FaultDisputeGameContract0180{
			FaultDisputeGameContractLatest: FaultDisputeGameContractLatest{
				metrics:     metrics,
				multiCaller: caller,
				contract:    batching.NewBoundContract(legacyAbi, addr),
				ops:         ops,
			},
		}, nil
	})
	builder.AddVersion(1, 1, func() (FaultDisputeGameContract, error) {
		legacyAbi := mustParseAbi(faultDisputeGameAbi111)
		return &FaultDisputeGameContract111{
			FaultDisputeGameContractLatest: FaultDisputeGameContractLatest{
				metrics:     metrics,
				multiCaller: caller,
				contract:    batching.NewBoundContract(legacyAbi, addr),
				ops:         ops,
			},
		}, nil
	})
	v131Factory := func() (FaultDisputeGameContract, error) {
		legacyAbi := mustParseAbi(faultDisputeGameAbi131)
		return &FaultDisputeGameContract131{
			FaultDisputeGameContractLatest: FaultDisputeGameContractLatest{
				metrics:     metrics,
				multiCaller: caller,
				contract:    batching.NewBoundContract(legacyAbi, addr),
				ops:         ops,
			},
		}, nil
	}
	// The ABI is equivalent between 1.2.x and 1.3.x - there were just changes to the constructor validation.
	builder.AddVersion(1, 2, v131Factory)
	builder.AddVersion(1, 3, v131Factory)
	return builder.Build(ctx, caller, contractAbi, addr, func() (FaultDisputeGameContract, error) {
		return &FaultDisputeGameContractLatest{
			metrics:     metrics,
			multiCaller: caller,
			contract:    batching.NewBoundContract(contractAbi, addr),
			ops:         ops,
		}, nil
	})
}

func mustParseAbi(json []byte) *abi.ABI {
	loaded, err := abi.JSON(bytes.NewReader(json))
	if err != nil {
		panic(err)
	}
	return &loaded
}

// GetBalanceAndDelay returns the total amount of ETH controlled by this contract.
// Note that the ETH is actually held by the DelayedWETH contract which may be shared by multiple games.
// Returns the balance and the address of the contract that actually holds the balance.
func (f *FaultDisputeGameContractLatest) GetBalanceAndDelay(ctx context.Context, block rpcblock.Block) (*big.Int, time.Duration, common.Address, error) {
	defer f.metrics.StartContractRequest("GetBalanceAndDelay")()
	weth, err := f.getDelayedWETH(ctx, block)
	if err != nil {
		return nil, 0, common.Address{}, fmt.Errorf("failed to get DelayedWETH contract: %w", err)
	}
	balance, delay, err := weth.GetBalanceAndDelay(ctx, block)
	if err != nil {
		return nil, 0, common.Address{}, fmt.Errorf("failed to get WETH balance and delay: %w", err)
	}
	return balance, delay, weth.Addr(), nil
}

// GetBlockRange returns the block numbers of the absolute pre-state block (typically genesis or the bedrock activation block)
// and the post-state block (that the proposed output root is for).
func (f *FaultDisputeGameContractLatest) GetBlockRange(ctx context.Context) (prestateBlock uint64, poststateBlock uint64, retErr error) {
	defer f.metrics.StartContractRequest("GetBlockRange")()
	results, err := f.multiCaller.Call(ctx, rpcblock.Latest,
		f.contract.Call(methodStartingBlockNumber),
		f.contract.Call(methodL2BlockNumber))
	if err != nil {
		retErr = fmt.Errorf("failed to retrieve game block range: %w", err)
		return
	}
	if len(results) != 2 {
		retErr = fmt.Errorf("expected 2 results but got %v", len(results))
		return
	}
	prestateBlock = results[0].GetBigInt(0).Uint64()
	poststateBlock = results[1].GetBigInt(0).Uint64()
	return
}

type GameMetadata struct {
	L1Head                  common.Hash
	L2BlockNum              uint64
	RootClaim               common.Hash
	Status                  gameTypes.GameStatus
	MaxClockDuration        uint64
	L2BlockNumberChallenged bool
	L2BlockNumberChallenger common.Address
}

// GetGameMetadata returns the game's L1 head, L2 block number, root claim, status, max clock duration, and is l2 block number challenged.
func (f *FaultDisputeGameContractLatest) GetGameMetadata(ctx context.Context, block rpcblock.Block) (GameMetadata, error) {
	defer f.metrics.StartContractRequest("GetGameMetadata")()
	l1Head := f.ops.GetL1Head(f.contract)
	l2BlockNumber := f.ops.GetL2SequenceNumber(f.contract)
	rootClaim := f.ops.GetRootClaim(f.contract)
	status := f.ops.GetStatus(f.contract)
	maxClockDuration := f.ops.GetMaxClockDuration(f.contract)
	l2BlockNumberChallenged := f.ops.GetL2BlockNumberChallenged(f.contract)
	l2BlockNumberChallenger := f.ops.GetL2BlockNumberChallenger(f.contract)

	err := ExecuteOps(ctx, block, f.multiCaller, l1Head, l2BlockNumber, rootClaim, status, maxClockDuration, l2BlockNumberChallenged, l2BlockNumberChallenger)
	if err != nil {
		return GameMetadata{}, fmt.Errorf("failed to retrieve game metadata: %w", err)
	}
	return GameMetadata{
		L1Head:                  l1Head.Result(),
		L2BlockNum:              l2BlockNumber.Result(),
		RootClaim:               rootClaim.Result(),
		Status:                  status.Result(),
		MaxClockDuration:        maxClockDuration.Result(),
		L2BlockNumberChallenged: l2BlockNumberChallenged.Result(),
		L2BlockNumberChallenger: l2BlockNumberChallenger.Result(),
	}, nil
}

func (f *FaultDisputeGameContractLatest) GetResolvedAt(ctx context.Context, block rpcblock.Block) (time.Time, error) {
	defer f.metrics.StartContractRequest("GetResolvedAt")()
	result, err := f.multiCaller.SingleCall(ctx, block, f.contract.Call(methodResolvedAt))
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to retrieve resolution time: %w", err)
	}
	resolvedAt := time.Unix(int64(result.GetUint64(0)), 0)
	return resolvedAt, nil
}

func (f *FaultDisputeGameContractLatest) GetStartingRootHash(ctx context.Context) (common.Hash, error) {
	defer f.metrics.StartContractRequest("GetStartingRootHash")()
	startingRootHash, err := f.multiCaller.SingleCall(ctx, rpcblock.Latest, f.contract.Call(methodStartingRootHash))
	if err != nil {
		return common.Hash{}, fmt.Errorf("failed to retrieve genesis output root: %w", err)
	}
	return startingRootHash.GetHash(0), nil
}

func (f *FaultDisputeGameContractLatest) GetSplitDepth(ctx context.Context) (types.Depth, error) {
	defer f.metrics.StartContractRequest("GetSplitDepth")()
	splitDepth, err := f.multiCaller.SingleCall(ctx, rpcblock.Latest, f.contract.Call(methodSplitDepth))
	if err != nil {
		return 0, fmt.Errorf("failed to retrieve split depth: %w", err)
	}
	return types.Depth(splitDepth.GetBigInt(0).Uint64()), nil
}

func (f *FaultDisputeGameContractLatest) GetCredit(ctx context.Context, recipient common.Address) (*big.Int, gameTypes.GameStatus, error) {
	defer f.metrics.StartContractRequest("GetCredit")()
	results, err := f.multiCaller.Call(ctx, rpcblock.Latest,
		f.contract.Call(methodCredit, recipient),
		f.contract.Call(methodStatus))
	if err != nil {
		return nil, gameTypes.GameStatusInProgress, err
	}
	if len(results) != 2 {
		return nil, gameTypes.GameStatusInProgress, fmt.Errorf("expected 2 results but got %v", len(results))
	}
	credit := results[0].GetBigInt(0)
	status, err := gameTypes.GameStatusFromUint8(results[1].GetUint8(0))
	if err != nil {
		return nil, gameTypes.GameStatusInProgress, fmt.Errorf("invalid game status %v: %w", status, err)
	}
	return credit, status, nil
}

func (f *FaultDisputeGameContractLatest) GetRequiredBonds(ctx context.Context, block rpcblock.Block, positions ...*big.Int) ([]*big.Int, error) {
	calls := make([]batching.Call, 0, len(positions))
	for _, position := range positions {
		calls = append(calls, f.contract.Call(methodRequiredBond, position))
	}
	results, err := f.multiCaller.Call(ctx, block, calls...)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve required bonds: %w", err)
	}
	requiredBonds := make([]*big.Int, 0, len(positions))
	for _, result := range results {
		requiredBonds = append(requiredBonds, result.GetBigInt(0))
	}
	return requiredBonds, nil
}

func (f *FaultDisputeGameContractLatest) GetCredits(ctx context.Context, block rpcblock.Block, recipients ...common.Address) ([]*big.Int, error) {
	defer f.metrics.StartContractRequest("GetCredits")()
	calls := make([]batching.Call, 0, len(recipients))
	for _, recipient := range recipients {
		calls = append(calls, f.contract.Call(methodCredit, recipient))
	}
	results, err := f.multiCaller.Call(ctx, block, calls...)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve credit: %w", err)
	}
	credits := make([]*big.Int, 0, len(recipients))
	for _, result := range results {
		credits = append(credits, result.GetBigInt(0))
	}
	return credits, nil
}

func (f *FaultDisputeGameContractLatest) ClaimCreditTx(ctx context.Context, recipient common.Address) (txmgr.TxCandidate, error) {
	defer f.metrics.StartContractRequest("ClaimCredit")()
	call := f.contract.Call(methodClaimCredit, recipient)
	_, err := f.multiCaller.SingleCall(ctx, rpcblock.Latest, call)
	if err != nil {
		return txmgr.TxCandidate{}, fmt.Errorf("%w: %w", ErrSimulationFailed, err)
	}
	return call.ToTxCandidate()
}

func (f *FaultDisputeGameContractLatest) GetRequiredBond(ctx context.Context, position types.Position) (*big.Int, error) {
	defer f.metrics.StartContractRequest("GetRequiredBond")()
	bond, err := f.multiCaller.SingleCall(ctx, rpcblock.Latest, f.contract.Call(methodRequiredBond, position.ToGIndex()))
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve required bond: %w", err)
	}
	return bond.GetBigInt(0), nil
}

func (f *FaultDisputeGameContractLatest) UpdateOracleTx(ctx context.Context, claimIdx uint64, data *types.PreimageOracleData) (txmgr.TxCandidate, error) {
	if data.IsLocal {
		return f.addLocalDataTx(claimIdx, data)
	}
	return f.addGlobalDataTx(ctx, data)
}

func (f *FaultDisputeGameContractLatest) addLocalDataTx(claimIdx uint64, data *types.PreimageOracleData) (txmgr.TxCandidate, error) {
	call := f.contract.Call(
		methodAddLocalData,
		data.GetIdent(),
		new(big.Int).SetUint64(claimIdx),
		new(big.Int).SetUint64(uint64(data.OracleOffset)),
	)
	return call.ToTxCandidate()
}

func (f *FaultDisputeGameContractLatest) addGlobalDataTx(ctx context.Context, data *types.PreimageOracleData) (txmgr.TxCandidate, error) {
	oracle, err := f.GetOracle(ctx)
	if err != nil {
		return txmgr.TxCandidate{}, err
	}
	return oracle.AddGlobalDataTx(data)
}

func (f *FaultDisputeGameContractLatest) GetWithdrawals(ctx context.Context, block rpcblock.Block, recipients ...common.Address) ([]*WithdrawalRequest, error) {
	defer f.metrics.StartContractRequest("GetWithdrawals")()
	delayedWETH, err := f.getDelayedWETH(ctx, block)
	if err != nil {
		return nil, err
	}
	return delayedWETH.GetWithdrawals(ctx, block, f.contract.Addr(), recipients...)
}

func (f *FaultDisputeGameContractLatest) getDelayedWETH(ctx context.Context, block rpcblock.Block) (*DelayedWETHContract, error) {
	defer f.metrics.StartContractRequest("GetDelayedWETH")()
	result, err := f.multiCaller.SingleCall(ctx, block, f.contract.Call(methodWETH))
	if err != nil {
		return nil, fmt.Errorf("failed to fetch WETH addr: %w", err)
	}
	return NewDelayedWETHContract(f.metrics, result.GetAddress(0), f.multiCaller), nil
}

func (f *FaultDisputeGameContractLatest) GetOracle(ctx context.Context) (PreimageOracleContract, error) {
	defer f.metrics.StartContractRequest("GetOracle")()
	vm, err := f.Vm(ctx)
	if err != nil {
		return nil, err
	}
	return vm.Oracle(ctx)
}

func (f *FaultDisputeGameContractLatest) GetMaxClockDuration(ctx context.Context) (time.Duration, error) {
	defer f.metrics.StartContractRequest("GetMaxClockDuration")()
	result, err := f.multiCaller.SingleCall(ctx, rpcblock.Latest, f.contract.Call(methodMaxClockDuration))
	if err != nil {
		return 0, fmt.Errorf("failed to fetch max clock duration: %w", err)
	}
	return time.Duration(result.GetUint64(0)) * time.Second, nil
}

func (f *FaultDisputeGameContractLatest) GetMaxGameDepth(ctx context.Context) (types.Depth, error) {
	defer f.metrics.StartContractRequest("GetMaxGameDepth")()
	result, err := f.multiCaller.SingleCall(ctx, rpcblock.Latest, f.contract.Call(methodMaxGameDepth))
	if err != nil {
		return 0, fmt.Errorf("failed to fetch max game depth: %w", err)
	}
	return types.Depth(result.GetBigInt(0).Uint64()), nil
}

func (f *FaultDisputeGameContractLatest) GetAbsolutePrestateHash(ctx context.Context) (common.Hash, error) {
	defer f.metrics.StartContractRequest("GetAbsolutePrestateHash")()
	result, err := f.multiCaller.SingleCall(ctx, rpcblock.Latest, f.contract.Call(methodAbsolutePrestate))
	if err != nil {
		return common.Hash{}, fmt.Errorf("failed to fetch absolute prestate hash: %w", err)
	}
	return result.GetHash(0), nil
}

func (f *FaultDisputeGameContractLatest) GetL1Head(ctx context.Context) (common.Hash, error) {
	defer f.metrics.StartContractRequest("GetL1Head")()
	result, err := f.multiCaller.SingleCall(ctx, rpcblock.Latest, f.contract.Call(methodL1Head))
	if err != nil {
		return common.Hash{}, fmt.Errorf("failed to fetch L1 head: %w", err)
	}
	return result.GetHash(0), nil
}

func (f *FaultDisputeGameContractLatest) GetStatus(ctx context.Context) (gameTypes.GameStatus, error) {
	defer f.metrics.StartContractRequest("GetStatus")()
	result, err := f.multiCaller.SingleCall(ctx, rpcblock.Latest, f.contract.Call(methodStatus))
	if err != nil {
		return 0, fmt.Errorf("failed to fetch status: %w", err)
	}
	return gameTypes.GameStatusFromUint8(result.GetUint8(0))
}

func (f *FaultDisputeGameContractLatest) GetClaimCount(ctx context.Context) (uint64, error) {
	defer f.metrics.StartContractRequest("GetClaimCount")()
	result, err := f.multiCaller.SingleCall(ctx, rpcblock.Latest, f.contract.Call(methodClaimCount))
	if err != nil {
		return 0, fmt.Errorf("failed to fetch claim count: %w", err)
	}
	return result.GetBigInt(0).Uint64(), nil
}

func (f *FaultDisputeGameContractLatest) GetClaim(ctx context.Context, idx uint64) (types.Claim, error) {
	defer f.metrics.StartContractRequest("GetClaim")()
	op := f.ops.GetClaim(f.contract, idx)
	err := ExecuteOps(ctx, rpcblock.Latest, f.multiCaller, op)
	if err != nil {
		return types.Claim{}, fmt.Errorf("failed to fetch claim %v: %w", idx, err)
	}
	return op.Result(), nil
}

func (f *FaultDisputeGameContractLatest) GetAllClaims(ctx context.Context, block rpcblock.Block) ([]types.Claim, error) {
	defer f.metrics.StartContractRequest("GetAllClaims")()
	results, err := batching.ReadArray(ctx, f.multiCaller, block, f.contract.Call(methodClaimCount), func(i *big.Int) *batching.ContractCall {
		return f.contract.Call(methodClaim, i)
	})
	if err != nil {
		return nil, fmt.Errorf("failed to load claims: %w", err)
	}

	var claims []types.Claim
	for idx, result := range results {
		claims = append(claims, f.decodeClaim(result, idx))
	}
	return claims, nil
}

func (f *FaultDisputeGameContractLatest) GetBondDistributionMode(ctx context.Context, block rpcblock.Block) (types.BondDistributionMode, error) {
	result, err := f.multiCaller.SingleCall(ctx, block, f.contract.Call(methodBondDistributionMode))
	if err != nil {
		return 0, fmt.Errorf("failed to fetch bond mode: %w", err)
	}
	return types.BondDistributionMode(result.GetUint8(0)), nil
}

func (f *FaultDisputeGameContractLatest) IsResolved(ctx context.Context, block rpcblock.Block, claims ...types.Claim) ([]bool, error) {
	defer f.metrics.StartContractRequest("IsResolved")()
	calls := make([]batching.Call, 0, len(claims))
	for _, claim := range claims {
		calls = append(calls, f.contract.Call(methodResolvedSubgames, big.NewInt(int64(claim.ContractIndex))))
	}
	results, err := f.multiCaller.Call(ctx, block, calls...)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve resolved subgames: %w", err)
	}
	resolved := make([]bool, 0, len(claims))
	for _, result := range results {
		resolved = append(resolved, result.GetBool(0))
	}
	return resolved, nil
}

func (f *FaultDisputeGameContractLatest) Vm(ctx context.Context) (*VMContract, error) {
	result, err := f.multiCaller.SingleCall(ctx, rpcblock.Latest, f.contract.Call(methodVM))
	if err != nil {
		return nil, fmt.Errorf("failed to fetch VM addr: %w", err)
	}
	vmAddr := result.GetAddress(0)
	return NewVMContract(vmAddr, f.multiCaller), nil
}

func (f *FaultDisputeGameContractLatest) IsL2BlockNumberChallenged(ctx context.Context, block rpcblock.Block) (bool, error) {
	defer f.metrics.StartContractRequest("IsL2BlockNumberChallenged")()
	op := f.ops.GetL2BlockNumberChallenged(f.contract)
	if err := ExecuteOps(ctx, block, f.multiCaller, op); err != nil {
		return false, fmt.Errorf("failed to fetch block number challenged: %w", err)
	}
	return op.Result(), nil
}

func (f *FaultDisputeGameContractLatest) ChallengeL2BlockNumberTx(challenge *types.InvalidL2BlockNumberChallenge) (txmgr.TxCandidate, error) {
	return f.ops.ChallengeL2BlockNumberTx(f.contract, challenge)
}

func (f *FaultDisputeGameContractLatest) AttackTx(ctx context.Context, parent types.Claim, pivot common.Hash) (txmgr.TxCandidate, error) {
	call := f.contract.Call(methodAttack, parent.Value, big.NewInt(int64(parent.ContractIndex)), pivot)
	return f.txWithBond(ctx, parent.Position.Attack(), call)
}

func (f *FaultDisputeGameContractLatest) DefendTx(ctx context.Context, parent types.Claim, pivot common.Hash) (txmgr.TxCandidate, error) {
	call := f.contract.Call(methodDefend, parent.Value, big.NewInt(int64(parent.ContractIndex)), pivot)
	return f.txWithBond(ctx, parent.Position.Defend(), call)
}

func (f *FaultDisputeGameContractLatest) txWithBond(ctx context.Context, position types.Position, call *batching.ContractCall) (txmgr.TxCandidate, error) {
	tx, err := call.ToTxCandidate()
	if err != nil {
		return txmgr.TxCandidate{}, fmt.Errorf("failed to create transaction: %w", err)
	}
	tx.Value, err = f.GetRequiredBond(ctx, position)
	if err != nil {
		return txmgr.TxCandidate{}, fmt.Errorf("failed to fetch required bond: %w", err)
	}
	return tx, nil
}

func (f *FaultDisputeGameContractLatest) StepTx(claimIdx uint64, isAttack bool, stateData []byte, proof []byte) (txmgr.TxCandidate, error) {
	call := f.contract.Call(methodStep, new(big.Int).SetUint64(claimIdx), isAttack, stateData, proof)
	return call.ToTxCandidate()
}

func (f *FaultDisputeGameContractLatest) CallResolveClaim(ctx context.Context, claimIdx uint64) error {
	defer f.metrics.StartContractRequest("CallResolveClaim")()
	call := f.resolveClaimCall(claimIdx)
	_, err := f.multiCaller.SingleCall(ctx, rpcblock.Latest, call)
	if err != nil {
		return fmt.Errorf("failed to call resolve claim: %w", err)
	}
	return nil
}

func (f *FaultDisputeGameContractLatest) ResolveClaimTx(claimIdx uint64) (txmgr.TxCandidate, error) {
	call := f.resolveClaimCall(claimIdx)
	return call.ToTxCandidate()
}

func (f *FaultDisputeGameContractLatest) resolveClaimCall(claimIdx uint64) *batching.ContractCall {
	return f.contract.Call(methodResolveClaim, new(big.Int).SetUint64(claimIdx), maxChildChecks)
}

func (f *FaultDisputeGameContractLatest) CallResolve(ctx context.Context) (gameTypes.GameStatus, error) {
	defer f.metrics.StartContractRequest("CallResolve")()
	call := f.resolveCall()
	result, err := f.multiCaller.SingleCall(ctx, rpcblock.Latest, call)
	if err != nil {
		return gameTypes.GameStatusInProgress, fmt.Errorf("failed to call resolve: %w", err)
	}
	return gameTypes.GameStatusFromUint8(result.GetUint8(0))
}

func (f *FaultDisputeGameContractLatest) ResolveTx() (txmgr.TxCandidate, error) {
	call := f.resolveCall()
	return call.ToTxCandidate()
}

func (f *FaultDisputeGameContractLatest) resolveCall() *batching.ContractCall {
	return f.contract.Call(methodResolve)
}

// decodeClock decodes a uint128 into a Clock duration and timestamp.
func decodeClock(clock *big.Int) types.Clock {
	maxUint64 := new(big.Int).Add(new(big.Int).SetUint64(math.MaxUint64), big.NewInt(1))
	remainder := new(big.Int)
	quotient, _ := new(big.Int).QuoRem(clock, maxUint64, remainder)
	return types.NewClock(time.Duration(quotient.Int64())*time.Second, time.Unix(remainder.Int64(), 0))
}

// packClock packs the Clock duration and timestamp into a uint128.
func packClock(c types.Clock) *big.Int {
	duration := big.NewInt(int64(c.Duration.Seconds()))
	encoded := new(big.Int).Lsh(duration, 64)
	return new(big.Int).Or(encoded, big.NewInt(c.Timestamp.Unix()))
}

func (f *FaultDisputeGameContractLatest) decodeClaim(result *batching.CallResult, contractIndex int) types.Claim {
	parentIndex := result.GetUint32(0)
	counteredBy := result.GetAddress(1)
	claimant := result.GetAddress(2)
	bond := result.GetBigInt(3)
	claim := result.GetHash(4)
	position := result.GetBigInt(5)
	clock := result.GetBigInt(6)
	return types.Claim{
		ClaimData: types.ClaimData{
			Value:    claim,
			Position: types.NewPositionFromGIndex(position),
			Bond:     bond,
		},
		CounteredBy:         counteredBy,
		Claimant:            claimant,
		Clock:               decodeClock(clock),
		ContractIndex:       contractIndex,
		ParentContractIndex: int(parentIndex),
	}
}

type FaultDisputeGameContract interface {
	GetBalanceAndDelay(ctx context.Context, block rpcblock.Block) (*big.Int, time.Duration, common.Address, error)
	GetBlockRange(ctx context.Context) (prestateBlock uint64, poststateBlock uint64, retErr error)
	GetGameMetadata(ctx context.Context, block rpcblock.Block) (GameMetadata, error)
	GetResolvedAt(ctx context.Context, block rpcblock.Block) (time.Time, error)
	GetStartingRootHash(ctx context.Context) (common.Hash, error)
	GetSplitDepth(ctx context.Context) (types.Depth, error)
	GetCredit(ctx context.Context, recipient common.Address) (*big.Int, gameTypes.GameStatus, error)
	GetRequiredBonds(ctx context.Context, block rpcblock.Block, positions ...*big.Int) ([]*big.Int, error)
	GetCredits(ctx context.Context, block rpcblock.Block, recipients ...common.Address) ([]*big.Int, error)
	ClaimCreditTx(ctx context.Context, recipient common.Address) (txmgr.TxCandidate, error)
	GetRequiredBond(ctx context.Context, position types.Position) (*big.Int, error)
	UpdateOracleTx(ctx context.Context, claimIdx uint64, data *types.PreimageOracleData) (txmgr.TxCandidate, error)
	GetWithdrawals(ctx context.Context, block rpcblock.Block, recipients ...common.Address) ([]*WithdrawalRequest, error)
	GetOracle(ctx context.Context) (PreimageOracleContract, error)
	GetMaxClockDuration(ctx context.Context) (time.Duration, error)
	GetMaxGameDepth(ctx context.Context) (types.Depth, error)
	GetAbsolutePrestateHash(ctx context.Context) (common.Hash, error)
	GetL1Head(ctx context.Context) (common.Hash, error)
	GetStatus(ctx context.Context) (gameTypes.GameStatus, error)
	GetClaimCount(ctx context.Context) (uint64, error)
	GetClaim(ctx context.Context, idx uint64) (types.Claim, error)
	GetAllClaims(ctx context.Context, block rpcblock.Block) ([]types.Claim, error)
	IsResolved(ctx context.Context, block rpcblock.Block, claims ...types.Claim) ([]bool, error)
	IsL2BlockNumberChallenged(ctx context.Context, block rpcblock.Block) (bool, error)
	ChallengeL2BlockNumberTx(challenge *types.InvalidL2BlockNumberChallenge) (txmgr.TxCandidate, error)
	AttackTx(ctx context.Context, parent types.Claim, pivot common.Hash) (txmgr.TxCandidate, error)
	DefendTx(ctx context.Context, parent types.Claim, pivot common.Hash) (txmgr.TxCandidate, error)
	StepTx(claimIdx uint64, isAttack bool, stateData []byte, proof []byte) (txmgr.TxCandidate, error)
	CallResolveClaim(ctx context.Context, claimIdx uint64) error
	ResolveClaimTx(claimIdx uint64) (txmgr.TxCandidate, error)
	CallResolve(ctx context.Context) (gameTypes.GameStatus, error)
	ResolveTx() (txmgr.TxCandidate, error)
	Vm(ctx context.Context) (*VMContract, error)
	GetBondDistributionMode(ctx context.Context, block rpcblock.Block) (types.BondDistributionMode, error)
}
