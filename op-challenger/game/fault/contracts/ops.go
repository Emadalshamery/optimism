package contracts

import (
	"context"
	"fmt"
	"math/big"

	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/types"
	"github.com/ethereum-optimism/optimism/op-service/sources/batching"
	"github.com/ethereum-optimism/optimism/op-service/sources/batching/rpcblock"
)

type ContractOp interface {
	NextCalls() ([]batching.Call, error)
}

type GetterContractOp[Result any] interface {
	ContractOp
	Result() Result
}

func ExecuteOps(ctx context.Context, block rpcblock.Block, caller *batching.MultiCaller, ops ...ContractOp) error {
	for {
		calls := make([]batching.Call, 0, len(ops))
		for _, op := range ops {
			opCalls, err := op.NextCalls()
			if err != nil {
				return err
			}
			calls = append(calls, opCalls...)
		}
		if len(calls) == 0 {
			return nil
		}
		_, err := caller.Call(ctx, block, calls...)
		if err != nil {
			return err
		}
	}
}

type GetClaimOp struct {
	contract *batching.BoundContract
	idx      uint64

	Claim types.Claim
}

func NewGetClaimOp(contract *batching.BoundContract, idx uint64) *GetClaimOp {
	return &GetClaimOp{
		contract: contract,
		idx:      idx,
	}
}

func (o *GetClaimOp) NextCalls() ([]batching.Call, error) {
	if o.Claim != (types.Claim{}) {
		return nil, nil
	}
	return []batching.Call{
		newCapturingCall(o.contract.Call(methodClaim, big.NewInt(int64(o.idx))), func(result *batching.CallResult) error {
			o.Claim = o.decodeClaim(result, o.idx)
			return nil
		}),
	}, nil
}

func (o *GetClaimOp) Result() types.Claim {
	return o.Claim
}

func (o *GetClaimOp) decodeClaim(result *batching.CallResult, contractIndex uint64) types.Claim {
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
		ContractIndex:       int(contractIndex),
		ParentContractIndex: int(parentIndex),
	}
}

var _ ContractOp = (*GetClaimOp)(nil)

type FixBondOp struct {
	source *GetClaimOp
}

func (o *FixBondOp) NextCalls() ([]batching.Call, error) {
	fmt.Println(o.source)
	fmt.Println(o.source.Claim)
	fmt.Println(o.source.Claim.Bond)
	// Set the required bond if required.
	if o.source.Claim.Bond.Cmp(resolvedBondAmount) == 0 {
		return []batching.Call{
			newCapturingCall(o.source.contract.Call(methodRequiredBond, o.source.Claim.Position.ToGIndex()), func(result *batching.CallResult) error {
				o.source.Claim.Bond = result.GetBigInt(0)
				return nil
			}),
		}, nil
	}
	// No further calls required
	return nil, nil
}

type SimpleContractGetterOp[Result any] struct {
	contract *batching.BoundContract
	method   string
	convert  func(result *batching.CallResult) (Result, error)

	called bool
	result Result
}

func NewSimpleGetterOp[Result any](contract *batching.BoundContract, method string, convert func(result *batching.CallResult) (Result, error)) *SimpleContractGetterOp[Result] {
	return &SimpleContractGetterOp[Result]{
		contract: contract,
		method:   method,
		convert:  convert,
	}
}

func (o *SimpleContractGetterOp[Result]) NextCalls() ([]batching.Call, error) {
	if o.called {
		return nil, nil
	}
	call := newCapturingCall(o.contract.Call(o.method), func(result *batching.CallResult) error {
		o.called = true
		converted, err := o.convert(result)
		if err != nil {
			return err
		}
		o.result = converted
		return nil
	})
	return []batching.Call{call}, nil
}

func (o *SimpleContractGetterOp[Result]) Result() Result {
	return o.result
}

type StaticGetterOp[Result any] struct {
	result Result
}

func NewStaticOp[Result any](result Result) GetterContractOp[Result] {
	return &StaticGetterOp[Result]{
		result: result,
	}
}

func (o *StaticGetterOp[Result]) NextCalls() ([]batching.Call, error) {
	return nil, nil
}

func (o *StaticGetterOp[Result]) Result() Result {
	return o.result
}

type chainedOps[Result any] struct {
	remaining    []ContractOp
	resultSource func() Result
}

func ChainOps[Result any](resultSource func() Result, ops ...ContractOp) GetterContractOp[Result] {
	return &chainedOps[Result]{
		remaining:    ops,
		resultSource: resultSource,
	}
}

func (o *chainedOps[Result]) NextCalls() ([]batching.Call, error) {
	for len(o.remaining) > 0 {
		calls, err := o.remaining[0].NextCalls()
		if err != nil {
			return nil, err
		}
		if len(calls) > 0 {
			return calls, nil
		}
		// No remaining calls required by this op, move on to the next
		o.remaining = o.remaining[1:]
	}
	// All calls from all ops complete.
	return nil, nil
}

func (o *chainedOps[Result]) Result() Result {
	return o.resultSource()
}
