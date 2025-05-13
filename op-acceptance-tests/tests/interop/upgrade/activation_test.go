package upgrade

import (
	"context"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"

	"github.com/ethereum-optimism/optimism/op-chain-ops/genesis"
	"github.com/ethereum-optimism/optimism/op-devstack/devtest"
	"github.com/ethereum-optimism/optimism/op-devstack/dsl"
	"github.com/ethereum-optimism/optimism/op-devstack/stack/match"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/predeploys"
)

// TestInteropActivation consolidates all interop activation checks into a single test
// to minimize resource overhead and test runtime by performing all checks with a single system setup.
func TestInteropActivation(gt *testing.T) {
	t := devtest.ParallelT(gt)
	// Create a system using the standard interop configuration defined in init_test.go
	// This is defined as: SimpleInterop = presets.NewSimpleInterop
	sys := SimpleInterop(t)

	t.Logger().Info("Starting consolidated interop activation test")

	devtest.RunParallel(t, sys.L2Networks(), func(t devtest.T, net *dsl.L2Network) {
		interopTime := net.Escape().ChainConfig().InteropTime
		t.Require().NotNil(interopTime)

		// =====================================================================
		// Step 1: Verify we can get a block before activation
		// =====================================================================
		t.Logger().Info("Step 1: Finding pre-activation block")
		preActivationBlock := net.LatestBlockBeforeTimestamp(t, *interopTime)
		t.Logger().Info("Found pre-activation block", 
			"number", preActivationBlock.Number, 
			"timestamp", preActivationBlock.Time)
		
		// Verify pre-activation block has timestamp before interop time
		t.Require().Less(preActivationBlock.Time, *interopTime, 
			"Pre-activation block timestamp should be before interop time")
		
		// =====================================================================
		// Step 2: Wait for and verify the activation block
		// =====================================================================
		t.Logger().Info("Step 2: Waiting for activation block")
		activationBlock := net.AwaitActivation(t, interopTime)
		t.Require().NotNil(activationBlock)
		t.Logger().Info("Found activation block", 
			"number", activationBlock.Number, 
			"timestamp", activationBlock.Time)
		
		// Verify activation block has timestamp >= interop time
		t.Require().GreaterOrEqual(activationBlock.Time, *interopTime, 
			"Activation block timestamp should be at or after interop time")
		
		// Verify activation block is a successor to pre-activation
		t.Require().Greater(activationBlock.Number, preActivationBlock.Number,
			"Activation block should have a higher number than pre-activation block")
		
		// =====================================================================
		// Step 3: Verify post-activation block production
		// =====================================================================
		ctx, cancel := context.WithTimeout(t.Ctx(), 30*time.Second)
		defer cancel()
		
		// Get the block right after activation
		l2_el := net.Escape().L2ELNode(match.FirstL2EL)
		initialLatest, err := l2_el.EthClient().InfoByLabel(ctx, eth.Unsafe)
		t.Require().NoError(err, "Expected to get latest block info")
		initialBlockNum := initialLatest.NumberU64()
		
		t.Logger().Info("Step 3: Verifying post-activation block production")
		t.Logger().Info("Initial post-activation block", "number", initialBlockNum)
		
		// Wait for a few more blocks to ensure the system continues to operate
		var latestBlockNum uint64
		for i := 0; i < 5; i++ {
			net.WaitForBlock()
			
			latest, err := l2_el.EthClient().InfoByLabel(ctx, eth.Unsafe)
			t.Require().NoError(err, "Expected to get latest block info")
			
			latestBlockNum = latest.NumberU64()
			t.Logger().Info("Additional post-activation block produced", "number", latestBlockNum)
		}
		
		// Verify blocks continue to be produced after activation
		t.Require().Greater(latestBlockNum, initialBlockNum, 
			"Expected to produce more blocks after activation")
		
		t.Logger().Info("Verified post-activation block production", 
			"initialBlock", initialBlockNum, 
			"latestBlock", latestBlockNum)

		// =====================================================================
		// Step 4: Verify CrossL2Inbox contract deployment
		// =====================================================================
		t.Logger().Info("Step 4: Verifying CrossL2Inbox contract deployment")
		
		el := net.Escape().L2ELNode(match.FirstL2EL)
		implAddrBytes, err := el.EthClient().GetStorageAt(t.Ctx(), predeploys.CrossL2InboxAddr,
			genesis.ImplementationSlot, activationBlock.Hash.String())
		t.Require().NoError(err, "Failed to get CrossL2Inbox implementation address")
		
		implAddr := common.BytesToAddress(implAddrBytes[:])
		t.Require().NotEqual(common.Address{}, implAddr, "CrossL2Inbox implementation address should not be zero")
		
		code, err := el.EthClient().CodeAtHash(t.Ctx(), implAddr, activationBlock.Hash)
		t.Require().NoError(err, "Failed to get code at implementation address")
		t.Require().NotEmpty(code, "CrossL2Inbox implementation should have code")
		
		t.Logger().Info("Successfully verified CrossL2Inbox deployment", 
			"implAddr", implAddr.Hex(),
			"codeSize", len(code))

		// =====================================================================
		// Summary of all verification steps for this chain
		// =====================================================================
		t.Logger().Info("All interop activation checks passed successfully",
			"chain", net.ChainID(),
			"preActivationBlock", preActivationBlock.Number,
			"preActivationTime", preActivationBlock.Time,
			"activationBlock", activationBlock.Number,
			"activationTime", activationBlock.Time,
			"interopTime", *interopTime,
			"postBlocksProduced", latestBlockNum-initialBlockNum+1)
	})

	t.Logger().Info("Consolidated interop activation test completed successfully")
}