package db

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ethereum-optimism/optimism/op-node/rollup/event"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/locks"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/activation"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/depset"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/superevents"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
)

// func TestActivationCheckAndActivateInterop(t *testing.T) {
// 	db := setupTestDB(t)
// 	chain := eth.ChainID{1}

// 	block := eth.BlockRef{
// 		Hash:   common.HexToHash("0xabcd"),
// 		Number: 100,
// 		Time:   2000,
// 	}

// 	db.initialized.Set(chain, struct{}{})

// 	// Create an activation manager for testing
// 	logger := testlog.Logger(t, log.LvlInfo)
// 	mock := &depset.MockDependencySet{
// 		CanInitiateAtFn: func(chainID eth.ChainID, timestamp uint64) (bool, error) {
// 			return timestamp >= 2000, nil
// 		},
// 	}
// 	activationMgr := activation.NewCheckFn(mock, logger)
// 	db.SetActivationManager(activationMgr)

// 	// Test DetectAndActivateInterop
// 	err := activationMgr.DetectAndActivateInterop(
// 		context.Background(),
// 		chain,
// 		block,
// 		db.syncSources,
// 		db,
// 	)
// 	require.NoError(t, err)

// 	assert.True(t, db.isInitialized(chain))
// }

func TestActivationEventFiltering(t *testing.T) {
	db := setupTestDB(t)
	chain := eth.ChainID{1}

	// Create a mock anchor for the tests to pass boundary checks
	mockAnchor := types.DerivedBlockRefPair{
		Source: eth.BlockRef{
			Hash:   common.HexToHash("0x1234"),
			Number: 1,
			Time:   100,
		},
		Derived: eth.BlockRef{
			Hash:   common.HexToHash("0x5678"),
			Number: 1, // Use a smaller number than the test block
			Time:   1000,
		},
	}
	db.anchorBlocks.Set(chain, mockAnchor)

	// Create an activation manager that considers blocks inactive
	mock := &depset.MockDependencySet{
		CanInitiateAtFn: func(chainID eth.ChainID, timestamp uint64) (bool, error) {
			return false, nil // All blocks are inactive
		},
		MessageExpiryWindowFn: func() uint64 {
			return 14 * 24 * 60 * 60 // 14 days in seconds
		},
	}
	db.activationCheckFn = activation.NewCheckFn(mock, testlog.Logger(t, log.LvlInfo))

	// Add a mock emitter to prevent nil pointer panic
	db.emitter = &mockEmitter{}

	// Initialize the database
	db.initialized.Set(chain, struct{}{})

	// Since we've moved the activation checks to the event level, the database methods
	// now only perform boundary checks. We're testing that they don't error when
	// given a block that passes the boundary check.

	block := eth.BlockRef{
		Hash:   common.HexToHash("0xabcd"),
		Number: 100, // Higher than anchor block number
		Time:   1000,
	}

	// All of these should pass the boundary check
	err := db.SealBlock(chain, block)
	assert.NoError(t, err, "SealBlock should pass boundary check")

	db.UpdateLocalSafe(chain, block, block, "test")

	err = db.UpdateCrossSafe(chain, block, block)
	assert.NoError(t, err, "UpdateCrossSafe should pass boundary check")

	db.crossUnsafe.Set(chain, &locks.RWValue[types.BlockSeal]{})

	seal := types.BlockSealFromRef(block)
	err = db.UpdateCrossUnsafe(chain, seal)
	assert.NoError(t, err, "UpdateCrossUnsafe should pass boundary check")
}

// Mock emitter that just records events
type mockEmitter struct {
	events []event.Event
}

func (m *mockEmitter) Emit(ev event.Event) {
	m.events = append(m.events, ev)
}

func TestActivationAnchorEventHandling(t *testing.T) {
	chain := eth.ChainID{1}

	t.Run("Interop mode with valid anchor", func(t *testing.T) {
		db := setupTestDB(t)

		mock := &depset.MockDependencySet{
			CanInitiateAtFn: func(chainID eth.ChainID, timestamp uint64) (bool, error) {
				return true, nil
			},
		}
		db.activationCheckFn = activation.NewCheckFn(mock, db.logger)

		db.initialized.Set(chain, struct{}{})

		db.OnEvent(superevents.AnchorEvent{
			ChainID: chain,
			Anchor: types.DerivedBlockRefPair{
				Source: eth.BlockRef{
					Hash:   common.HexToHash("0x1234"),
					Number: 1,
					Time:   100,
				},
				Derived: eth.BlockRef{
					Hash:   common.HexToHash("0x5678"),
					Number: 10,
					Time:   1000,
				},
			},
		})

		assert.True(t, db.isInitialized(chain), "database should be initialized")
	})
}

func TestActivationManagerIsActiveForChain(t *testing.T) {
	logger := testlog.Logger(t, log.LvlInfo)
	chain := eth.ChainID{1}

	// Test with nil depSet
	activationCheckFn := activation.NewCheckFn(nil, logger)
	assert.False(t, activationCheckFn(chain, 1234), "With nil depSet, should default to false")

	activationTime := uint64(1000)

	for _, tc := range []struct {
		name      string
		timestamp uint64
		expected  bool
	}{
		{
			name:      "Before interop time",
			timestamp: 900,
			expected:  false,
		},
		{
			name:      "At interop time",
			timestamp: 1000,
			expected:  false,
		},
		{
			name:      "After interop time",
			timestamp: 1100,
			expected:  true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mock := &depset.MockDependencySet{
				CanInitiateAtFn: func(chainID eth.ChainID, timestamp uint64) (bool, error) {
					return timestamp > activationTime, nil
				},
			}
			activationCheckFn := activation.NewCheckFn(mock, logger)
			actual := activationCheckFn(chain, tc.timestamp)
			assert.Equal(t, tc.expected, actual, "Timestamp %d expected %v but got %v", tc.timestamp, tc.expected, actual)
		})
	}
}

func TestActivationAnchorBlockTracking(t *testing.T) {
	logger := log.New()
	chainID := eth.ChainID{1}

	// Create test chains DB
	chainsDB := NewChainsDB(logger, nil, nil)

	// Verify chain is not initialized and has no anchor block initially
	require.False(t, chainsDB.IsInitialized(chainID))
	_, hasAnchor := chainsDB.GetAnchorBlock(chainID)
	require.False(t, hasAnchor)
	require.False(t, chainsDB.IsInInteropMode(chainID))

	// Create a test anchor block pair
	source := eth.L1BlockRef{
		Number: 100,
		Hash:   common.Hash{0x01},
		Time:   1000,
	}
	derived := eth.BlockRef{
		Number: 200,
		Hash:   common.Hash{0x02},
		Time:   2000,
	}
	anchorPair := types.DerivedBlockRefPair{
		Source:  source,
		Derived: derived,
	}

	// Initialize with anchor block
	chainsDB.anchorBlocks.Set(chainID, anchorPair)
	chainsDB.initialized.Set(chainID, struct{}{})

	// Verify state after initialization with anchor block
	require.True(t, chainsDB.IsInitialized(chainID))
	_, hasAnchor = chainsDB.GetAnchorBlock(chainID)
	require.True(t, hasAnchor)
	require.True(t, chainsDB.IsInInteropMode(chainID))

	// Get the anchor block and verify it matches what we set
	retrievedAnchor, ok := chainsDB.GetAnchorBlock(chainID)
	require.True(t, ok)
	require.Equal(t, anchorPair, retrievedAnchor)

	// Initialize another chain in pre-interop mode (initialized but no anchor block)
	preInteropChainID := eth.ChainID{2}
	chainsDB.initialized.Set(preInteropChainID, struct{}{})

	// Verify pre-interop state
	require.True(t, chainsDB.IsInitialized(preInteropChainID))
	_, hasAnchor = chainsDB.GetAnchorBlock(preInteropChainID)
	require.False(t, hasAnchor)
	require.False(t, chainsDB.IsInInteropMode(preInteropChainID))
}
