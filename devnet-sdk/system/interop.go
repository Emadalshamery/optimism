package system

import (
	"context"
	"fmt"
	"math/big"

	"github.com/ethereum-optimism/optimism/devnet-sdk/contracts/constants"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/plan"
	"github.com/ethereum-optimism/optimism/op-service/txplan"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/lmittmann/w3"

	suptypes "github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
)

type InitTrigger struct {
	Emitter    common.Address // address of the EventLogger contract
	Topics     []common.Hash
	OpaqueData []byte
}

func (v *InitTrigger) To() (*common.Address, error) {
	return &v.Emitter, nil
}

func (v *InitTrigger) Data() ([]byte, error) {
	data := []byte{}

	return data, nil
}

func (v *InitTrigger) AccessList() (types.AccessList, error) {
	return nil, nil
}

type SendTrigger struct {
	Emitter    common.Address
	OpaqueData []byte
}

func (v *SendTrigger) To() (*common.Address, error) {
	return &v.Emitter, nil
}

func (v *SendTrigger) Data() ([]byte, error) {
	return v.OpaqueData, nil
}

func (v *SendTrigger) AccessList() (types.AccessList, error) {
	return nil, nil
}

type ExecTrigger struct {
	Executor common.Address // address of the EventLogger contract
	Msg      suptypes.Message
}

func (v *ExecTrigger) To() (*common.Address, error) {
	return &v.Executor, nil
}

func (v *ExecTrigger) Data() ([]byte, error) {
	// TODO: Need to do better construct call input than this
	validateMessage := w3.MustNewFunc("validateMessage((address Origin, uint256 BlockNumber, uint256 LogIndex, uint256 Timestamp, uint256 ChainId), bytes32)", "")
	type Identifier struct {
		Origin      common.Address
		BlockNumber *big.Int
		LogIndex    *big.Int
		Timestamp   *big.Int
		ChainId     *big.Int
	}
	identifier := &Identifier{
		v.Msg.Identifier.Origin,
		big.NewInt(int64(v.Msg.Identifier.BlockNumber)),
		big.NewInt(int64(v.Msg.Identifier.LogIndex)),
		big.NewInt(int64(v.Msg.Identifier.Timestamp)),
		v.Msg.Identifier.ChainID.ToBig(),
	}
	validateMessageCalldata, err := validateMessage.EncodeArgs(
		identifier,
		v.Msg.PayloadHash,
	)
	if err != nil {
		return nil, err
	}
	return validateMessageCalldata, nil
}

func (v *ExecTrigger) AccessList() (types.AccessList, error) {
	access := v.Msg.Access()
	accessList := types.AccessList{{
		Address:     constants.CrossL2Inbox,
		StorageKeys: suptypes.EncodeAccessList([]suptypes.Access{access}),
	}}
	return accessList, nil
}

type RelayTrigger struct {
	ExecTrigger
	Payload []byte
}

func (v *RelayTrigger) Data() ([]byte, error) {
	// TODO: Need to do better construct call input than this
	relayMessage := w3.MustNewFunc("relayMessage((address Origin, uint256 BlockNumber, uint256 LogIndex, uint256 Timestamp, uint256 ChainId), bytes sentMessage)", "bytes returnData")
	type Identifier struct {
		Origin      common.Address
		BlockNumber *big.Int
		LogIndex    *big.Int
		Timestamp   *big.Int
		ChainId     *big.Int
	}
	identifier := &Identifier{
		v.Msg.Identifier.Origin,
		big.NewInt(int64(v.Msg.Identifier.BlockNumber)),
		big.NewInt(int64(v.Msg.Identifier.LogIndex)),
		big.NewInt(int64(v.Msg.Identifier.Timestamp)),
		v.Msg.Identifier.ChainID.ToBig(),
	}
	relayMessageCalldata, err := relayMessage.EncodeArgs(
		identifier,
		v.Payload,
	)
	if err != nil {
		return nil, err
	}
	return relayMessageCalldata, nil
}

type Call interface {
	To() (*common.Address, error)
	Data() ([]byte, error)
	AccessList() (types.AccessList, error)
}

type MultiTrigger struct {
	Calls []Call
}

func (v *MultiTrigger) Data() ([]byte, error) {
	// TODO format multi-call
	return nil, nil
}

type Result interface {
	FromReceipt(ctx context.Context, rec *types.Receipt, includedIn eth.BlockRef, chainID eth.ChainID) error
	Init() Result
}

type InteropOutput struct {
	Entries []suptypes.Message
}

func (i *InteropOutput) Init() Result {
	return &InteropOutput{}
}

func (i *InteropOutput) FromReceipt(ctx context.Context, rec *types.Receipt, includedIn eth.BlockRef, chainID eth.ChainID) error {
	entries := []suptypes.Message{}
	for _, logEvent := range rec.Logs {
		payload := suptypes.LogToMessagePayload(logEvent)
		id := suptypes.Identifier{
			Origin:      logEvent.Address,
			BlockNumber: logEvent.BlockNumber,
			LogIndex:    uint32(logEvent.Index),
			Timestamp:   includedIn.Time,
			ChainID:     chainID,
		}
		payloadHash := crypto.Keccak256Hash(payload)
		entries = append(entries, suptypes.Message{
			Identifier:  id,
			PayloadHash: payloadHash,
		})
	}
	i.Entries = entries
	return nil
}

type IntentTx[V Call, R Result] struct {
	PlannedTx *txplan.PlannedTx
	Content   plan.Lazy[V]
	Result    plan.Lazy[R]
}

func NewIntent[V Call, R Result](opts ...txplan.Option) *IntentTx[V, R] {
	v := &IntentTx[V, R]{
		PlannedTx: txplan.NewPlannedTx(opts...),
	}
	v.PlannedTx.To.DependOn(&v.Content)
	v.PlannedTx.To.Fn(func(ctx context.Context) (*common.Address, error) {
		return v.Content.Value().To()
	})
	v.PlannedTx.Data.DependOn(&v.Content)
	v.PlannedTx.Data.Fn(func(ctx context.Context) (hexutil.Bytes, error) {
		return v.Content.Value().Data()
	})
	v.PlannedTx.AccessList.DependOn(&v.Content)
	v.PlannedTx.AccessList.Fn(func(ctx context.Context) (types.AccessList, error) {
		return v.Content.Value().AccessList()
	})
	v.Result.DependOn(&v.PlannedTx.Included, &v.PlannedTx.IncludedBlock, &v.PlannedTx.ChainID)
	v.Result.Fn(func(ctx context.Context) (R, error) {
		r := (*new(R)).Init().(R)
		err := r.FromReceipt(ctx, v.PlannedTx.Included.Value(), v.PlannedTx.IncludedBlock.Value(), v.PlannedTx.ChainID.Value())
		return r, err
	})
	return v
}

func ExecuteIndexed(executor common.Address, events *plan.Lazy[*InteropOutput], index int) func(ctx context.Context) (*ExecTrigger, error) {
	return func(ctx context.Context) (*ExecTrigger, error) {
		if x := len(events.Value().Entries); x <= index {
			return nil, fmt.Errorf("invalid index: %d, only have %d events", index, x)
		}
		return &ExecTrigger{
			Executor: executor,
			Msg:      events.Value().Entries[index],
		}, nil
	}
}

func RelayIndexed(executor common.Address, events *plan.Lazy[*InteropOutput], receipt *plan.Lazy[*types.Receipt], index int) func(ctx context.Context) (*RelayTrigger, error) {
	return func(ctx context.Context) (*RelayTrigger, error) {
		if x := len(events.Value().Entries); x <= index {
			return nil, fmt.Errorf("invalid entry index: %d, only have %d events", index, x)
		}
		if x := len(receipt.Value().Logs); x <= index {
			return nil, fmt.Errorf("invalid log index: %d, only have %d events", index, x)
		}
		msg := events.Value().Entries[index]
		payload := suptypes.LogToMessagePayload(receipt.Value().Logs[index])
		payloadHash := crypto.Keccak256Hash(payload)
		if msg.PayloadHash != payloadHash {
			return nil, fmt.Errorf("payload hash does not match, want %s but got %s", msg.PayloadHash.Hex(), payloadHash.Hex())
		}
		return &RelayTrigger{
			ExecTrigger: ExecTrigger{
				Executor: executor,
				Msg:      msg,
			},
			Payload: payload,
		}, nil
	}
}
