package interop

import (
	"testing"

	"github.com/ethereum-optimism/optimism/devnet-sdk/contracts/constants"
	"github.com/ethereum-optimism/optimism/devnet-sdk/system"
	"github.com/ethereum-optimism/optimism/devnet-sdk/testing/systest"
	"github.com/ethereum-optimism/optimism/devnet-sdk/testing/testlib/validators"
	"github.com/stretchr/testify/require"
)

// initAndExecMsg tests below scenario:
// Transaction initiates, and then executes message
func initAndExecMsg(
	lowLevelSystemGetter validators.LowLevelSystemGetter,
	l2ChainNums int,
	chainIdxs []uint64,
	walletGetters []validators.WalletGetter,
) systest.InteropSystemTestFunc {
	return func(t systest.T, sys system.InteropSystem) {
		ctx, rng, logger, _, wallets, opts := DefaultSetup(t, lowLevelSystemGetter, l2ChainNums, chainIdxs, walletGetters)

		eventLoggerAddress, err := DeployEventLogger(ctx, wallets[0], logger)
		require.NoError(t, err)

		// Intent to initiate message(or emit event) on chain A
		txA := system.NewIntent[*system.InitTrigger, *system.InteropOutput](opts[0])
		randomInitTrigger := RandomInitTrigger(rng, eventLoggerAddress, 3, 10)
		txA.Content.Set(randomInitTrigger)

		// Trigger single event
		receiptA, err := txA.PlannedTx.Included.Eval(ctx)
		require.NoError(t, err)
		logger.Info("initiate message included", "block", receiptA.BlockHash)

		// Intent to validate message on chain B
		txB := system.NewIntent[*system.ExecTrigger, *system.InteropOutput](opts[1])
		txB.Content.DependOn(&txA.Result)

		// Single event in tx so index is 0
		txB.Content.Fn(system.ExecuteIndexed(constants.CrossL2Inbox, &txA.Result, 0))

		receiptB, err := txB.PlannedTx.Included.Eval(ctx)
		require.NoError(t, err)
		logger.Info("validate message included", "block", receiptB.BlockHash)
	}
}

// initAndExecMultipleMsg tests below scenario:
// Transaction initiates and executes multiple messages of self
func initAndExecMultipleMsg(
	lowLevelSystemGetter validators.LowLevelSystemGetter,
	l2ChainNums int,
	chainIdxs []uint64,
	walletGetters []validators.WalletGetter,
) systest.InteropSystemTestFunc {
	return func(t systest.T, sys system.InteropSystem) {
		ctx, rng, logger, _, wallets, opts := DefaultSetup(t, lowLevelSystemGetter, l2ChainNums, chainIdxs, walletGetters)

		eventLoggerAddress, err := DeployEventLogger(ctx, wallets[0], logger)
		require.NoError(t, err)

		// Intent to initiate two message(or emit event) on chain A
		initCalls := []system.Call{
			RandomInitTrigger(rng, eventLoggerAddress, 1, 15),
			RandomInitTrigger(rng, eventLoggerAddress, 2, 13),
		}
		txA := system.NewIntent[*system.MultiTrigger, *system.InteropOutput](opts[0])
		txA.Content.Set(&system.MultiTrigger{Executor: constants.MultiCall3, Calls: initCalls})

		// Trigger two events
		receiptA, err := txA.PlannedTx.Included.Eval(ctx)
		require.NoError(t, err)
		logger.Info("initiate messages included", "block", receiptA.BlockHash)
		require.Equal(t, 2, len(receiptA.Logs))

		// Intent to validate messages on chain B
		txB := system.NewIntent[*system.MultiTrigger, *system.InteropOutput](opts[1])
		txB.Content.DependOn(&txA.Result)

		// Two events in tx so use every index, 0, 1
		indexes := []int{0, 1}
		txB.Content.Fn(system.ExecuteIndexeds(constants.MultiCall3, constants.CrossL2Inbox, &txA.Result, indexes))

		receiptB, err := txB.PlannedTx.Included.Eval(ctx)
		require.NoError(t, err)
		logger.Info("validate messages included", "block", receiptB.BlockHash)
	}
}

// execSameMsgTwice tests below scenario:
// Transaction that executes the same message twice.
func execSameMsgTwice(
	lowLevelSystemGetter validators.LowLevelSystemGetter,
	l2ChainNums int,
	chainIdxs []uint64,
	walletGetters []validators.WalletGetter,
) systest.InteropSystemTestFunc {
	return func(t systest.T, sys system.InteropSystem) {
		ctx, rng, logger, _, wallets, opts := DefaultSetup(t, lowLevelSystemGetter, l2ChainNums, chainIdxs, walletGetters)

		eventLoggerAddress, err := DeployEventLogger(ctx, wallets[0], logger)
		require.NoError(t, err)

		// Intent to initiate message(or emit event) on chain A
		txA := system.NewIntent[*system.InitTrigger, *system.InteropOutput](opts[0])
		randomInitTrigger := RandomInitTrigger(rng, eventLoggerAddress, 3, 10)
		txA.Content.Set(randomInitTrigger)

		// Trigger single event
		receiptA, err := txA.PlannedTx.Included.Eval(ctx)
		require.NoError(t, err)
		logger.Info("initiate message included", "block", receiptA.BlockHash)

		// Intent to validate same message two times on chain B
		txB := system.NewIntent[*system.MultiTrigger, *system.InteropOutput](opts[1])
		txB.Content.DependOn(&txA.Result)

		// Single event in tx so index is 0
		indexes := []int{0, 0}
		txB.Content.Fn(system.ExecuteIndexeds(constants.MultiCall3, constants.CrossL2Inbox, &txA.Result, indexes))

		receiptB, err := txB.PlannedTx.Included.Eval(ctx)
		require.NoError(t, err)
		logger.Info("validate messages included", "block", receiptB.BlockHash)
	}
}

func TestInteropTxTest(t *testing.T) {
	l2ChainNums := 2
	chainIdxs, walletGetters, totalValidators, lowLevelSystemGetter := SetupDefaultInteropSystemTest(l2ChainNums)

	tests := []struct {
		name     string
		testFunc systest.InteropSystemTestFunc
	}{
		{"initAndExecMsg", initAndExecMsg(lowLevelSystemGetter, l2ChainNums, chainIdxs, walletGetters)},
		{"initAndExecMultipleMsg", initAndExecMultipleMsg(lowLevelSystemGetter, l2ChainNums, chainIdxs, walletGetters)},
		{"execSameMsgTwice", execSameMsgTwice(lowLevelSystemGetter, l2ChainNums, chainIdxs, walletGetters)},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			systest.InteropSystemTest(t,
				test.testFunc,
				totalValidators...,
			)
		})
	}
}
