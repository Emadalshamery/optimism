package systest

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"testing"
	"time"

	"github.com/ethereum-optimism/optimism/op-service/testlog"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
	"github.com/stretchr/testify/require"
)

type mockGethClient struct {
	latestBlockNum int
	headersByNum   map[int]types.Header
}

func (m mockGethClient) HeaderByNumber(ctx context.Context, number *big.Int) (*types.Header, error) {
	idx := int(0)
	if number == nil {
		idx = m.latestBlockNum
	} else {
		idx = int(number.Int64())
	}
	h := m.headersByNum[idx]
	return &h, nil
}
func (mockGethClient) HeaderByHash(ctx context.Context, hash common.Hash) (*types.Header, error) {
	panic("unimplemented")
}
func (mockGethClient) BlockByNumber(ctx context.Context, number *big.Int) (*types.Block, error) {
	panic("unimplemented")
}
func (mockGethClient) Close() {}

var _ HeaderProvider = mockGethClient{}

func TestCheckForChainFork(t *testing.T) {
	leader := mockGethClient{latestBlockNum: 0, headersByNum: map[int]types.Header{
		0: {
			Number: big.NewInt(0),
			TxHash: common.HexToHash("0x0"),
		},
		1: {
			Number: big.NewInt(1),
			TxHash: common.HexToHash("0x1"),
		},
	},
	}

	followerA := mockGethClient{latestBlockNum: 0, headersByNum: map[int]types.Header{
		0: {
			Number: big.NewInt(0),
			TxHash: common.HexToHash("0x0"), // in sync with mockA at this block
		},
		1: {
			Number: big.NewInt(1),
			TxHash: common.HexToHash("0xb"), // forks off from leader at this block
		},
	},
	}

	followerB := mockGethClient{latestBlockNum: 0, headersByNum: map[int]types.Header{
		0: {
			Number: big.NewInt(0),
			TxHash: common.HexToHash("0x0"), // forks off from leader at this block
		},
		1: {
			Number: big.NewInt(1),
			TxHash: common.HexToHash("0xb"), // forks off from leader at this block
		},
	},
	}

	// First scenario is that the leader and follower are in sync initially, but then split:
	secondCheck, firstErr := checkForChainFork(context.Background(), []HeaderProvider{leader, followerA}, testlog.Logger(t, log.LevelDebug))
	require.NoError(t, firstErr)
	leader.latestBlockNum = 1    // advance the chain head
	followerA.latestBlockNum = 1 // advance the chain head
	require.Error(t, secondCheck(false), "expected chain split error")

	// Second scenario is that the leader and follower are forked immediately:
	_, firstErr = checkForChainFork(context.Background(), []HeaderProvider{leader, followerB}, testlog.Logger(t, log.LevelDebug))
	require.Error(t, firstErr, "expected chain split error")

}

func TestVerifyFollowersWithRetry(t *testing.T) {
	blockNum := big.NewInt(10)
	primaryHash := common.HexToHash("0xabc123")
	mc := &MultiClient{
		clients:    make([]HeaderProvider, 3),
		maxRetries: 3,
		retryDelay: 10 * time.Millisecond,
	}

	// Test case 1: All followers match primary hash
	t.Run("AllFollowersMatch", func(t *testing.T) {
		// Setup getHash function that always returns matching hash
		getHashFn := func(provider HeaderProvider, num *big.Int) (common.Hash, error) {
			return primaryHash, nil
		}

		mismatches, err := mc.verifyFollowersWithRetry(context.Background(), blockNum, primaryHash, getHashFn)
		require.NoError(t, err)
		require.Equal(t, 0, mismatches.Len())
	})

	// Test case 2: One follower has temporary error that resolves on retry
	t.Run("TemporaryErrorResolved", func(t *testing.T) {
		attempt := 0
		getHashFn := func(provider HeaderProvider, num *big.Int) (common.Hash, error) {
			// Client 1 fails on first attempt but succeeds after
			if provider == mc.clients[1] && attempt == 0 {
				attempt++
				return common.Hash{}, errors.New("not found")
			}
			return primaryHash, nil
		}

		mismatches, err := mc.verifyFollowersWithRetry(context.Background(), blockNum, primaryHash, getHashFn)
		require.NoError(t, err)
		require.Equal(t, 0, mismatches.Len())
	})

	// Test case 3: Chain split detected
	t.Run("ChainSplitDetected", func(t *testing.T) {
		followerHash := common.HexToHash("0xdef456")
		getHashFn := func(provider HeaderProvider, num *big.Int) (common.Hash, error) {
			if provider == mc.clients[1] {
				return followerHash, nil // Different hash indicates chain split
			}
			return primaryHash, nil
		}

		mismatches, err := mc.verifyFollowersWithRetry(context.Background(), blockNum, primaryHash, getHashFn)
		require.Error(t, err)
		require.Contains(t, err.Error(), "chain split detected")
		require.Equal(t, 1, mismatches.Len())
		require.Equal(t, 1, mismatches.clientIndices[0])
		require.Equal(t, followerHash, mismatches.hashes[0])
	})

	// Test case 4: Persistent error exceeding retry limit
	t.Run("PersistentError", func(t *testing.T) {
		mc = &MultiClient{
			clients:    make([]HeaderProvider, 3),
			maxRetries: 2,
			retryDelay: 10 * time.Millisecond,
		}
		failingClientIdx := 1

		getHashFn := func(provider HeaderProvider, num *big.Int) (common.Hash, error) {
			if provider == mc.clients[failingClientIdx] {
				return common.Hash{}, errors.New("not found")
			}
			return primaryHash, nil
		}

		mismatches, err := mc.verifyFollowersWithRetry(context.Background(), blockNum, primaryHash, getHashFn)
		require.Error(t, err)
		require.Contains(t, err.Error(), fmt.Sprintf("client %d failed", failingClientIdx))
		require.Equal(t, 0, mismatches.Len())
	})

	// Test case 5: Context cancellation
	t.Run("ContextCancellation", func(t *testing.T) {
		mc.retryDelay = 100 * time.Millisecond
		ctx, cancel := context.WithCancel(context.Background())

		getHashFn := func(provider HeaderProvider, num *big.Int) (common.Hash, error) {
			if provider == mc.clients[1] {
				// Cancel context after first attempt
				cancel()
				// Return error to trigger retry
				return common.Hash{}, errors.New("temporary error")
			}
			return primaryHash, nil
		}

		_, err := mc.verifyFollowersWithRetry(ctx, blockNum, primaryHash, getHashFn)
		require.Error(t, err)
		require.Equal(t, context.Canceled, err)
	})
}
