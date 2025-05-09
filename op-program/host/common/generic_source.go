package common

import (
	"context"

	hosttypes "github.com/ethereum-optimism/optimism/op-program/host/types"
	"github.com/ethereum-optimism/optimism/op-service/client"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/sources"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
)

type GenericSource struct {
	logger log.Logger

	canonicalEthClient   *sources.EthClient
	canonicalDebugClient *sources.DebugClient
}

var _ hosttypes.L2Source = &L2Source{}

func NewGenericSourceWithClient(logger log.Logger, canonicalClient *sources.EthClient, canonicalDebugClient *sources.DebugClient) *GenericSource {
	source := &GenericSource{
		logger:               logger,
		canonicalEthClient:   canonicalClient,
		canonicalDebugClient: canonicalDebugClient,
	}

	return source
}

func NewGenericSourceFromRPC(logger log.Logger, canonicalRPC client.RPC) (*GenericSource, error) {
	canonicalDebugClient := sources.NewDebugClient(canonicalRPC.CallContext)
	canonicalClient, err := sources.NewEthClient(canonicalRPC, logger, nil, sources.DefaultEthClientConfig(1000))
	if err != nil {
		return nil, err
	}

	source := NewGenericSourceWithClient(logger, canonicalClient, canonicalDebugClient)
	return source, nil
}

// CodeByHash implements prefetcher.L2Source.
func (l *GenericSource) CodeByHash(ctx context.Context, hash common.Hash) ([]byte, error) {
	return l.canonicalDebugClient.CodeByHash(ctx, hash)
}

// FetchReceipts implements prefetcher.L2Source.
func (l *GenericSource) FetchReceipts(ctx context.Context, blockHash common.Hash) (eth.BlockInfo, types.Receipts, error) {
	return l.canonicalEthClient.FetchReceipts(ctx, blockHash)
}

// NodeByHash implements prefetcher.L2Source.
func (l *GenericSource) NodeByHash(ctx context.Context, hash common.Hash) ([]byte, error) {
	return l.canonicalDebugClient.NodeByHash(ctx, hash)
}

// InfoAndTxsByHash implements prefetcher.L2Source.
func (l *GenericSource) InfoAndTxsByHash(ctx context.Context, blockHash common.Hash) (eth.BlockInfo, types.Transactions, error) {
	return l.canonicalEthClient.InfoAndTxsByHash(ctx, blockHash)
}

// GetProof implements prefetcher.L2Source.
func (l *GenericSource) GetProof(ctx context.Context, address common.Address, storage []common.Hash, blockTag string) (*eth.AccountResult, error) {
	proof, err := l.canonicalEthClient.GetProof(ctx, address, storage, blockTag)
	if err != nil {
		l.logger.Error("Failed to fetch proof from canonical source", "address", address, "storage", storage, "blockTag", blockTag, "err", err)
		return nil, ErrExperimentalPrefetchFailed
	}
	return proof, nil
}
