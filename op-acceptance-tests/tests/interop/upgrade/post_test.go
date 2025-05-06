package upgrade

import (
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"

	"github.com/ethereum-optimism/optimism/devnet-sdk/devstack/devtest"
	"github.com/ethereum-optimism/optimism/devnet-sdk/devstack/dsl"
	"github.com/ethereum-optimism/optimism/devnet-sdk/devstack/stack/match"
	"github.com/ethereum-optimism/optimism/op-chain-ops/genesis"
	"github.com/ethereum-optimism/optimism/op-service/predeploys"
)

func TestPostInbox(gt *testing.T) {
	t := devtest.ParallelT(gt)
	sys := SimpleInterop(t)
	devtest.RunParallel(t, sys.L2Networks(), func(t devtest.T, net *dsl.L2Network) {
		require := t.Require()
		activationBlock := net.AwaitActivation(t, net.Escape().ChainConfig().InteropTime)

		el := net.Escape().L2ELNode(match.FirstL2EL)
		implAddrBytes, err := el.EthClient().GetStorageAt(t.Ctx(), predeploys.CrossL2InboxAddr,
			genesis.ImplementationSlot, activationBlock.Hash.String())
		require.NoError(err)
		implAddr := common.BytesToAddress(implAddrBytes[:])
		require.NotEqual(common.Address{}, implAddr)
		code, err := el.EthClient().CodeAtHash(t.Ctx(), implAddr, activationBlock.Hash)
		require.NoError(err)
		require.NotEmpty(code)
	})
}

func TestLongRun(gt *testing.T) {
	t := devtest.ParallelT(gt)
	sys := SimpleInterop(t)

	net := sys.L2ChainA
	activationBlock := net.AwaitActivation(t, net.Escape().ChainConfig().InteropTime)

	sys.Supervisor.VerifySyncStatus(dsl.WithAllLocalUnsafeHeadsAdvancedBy(100))
	api := sys.Supervisor.Escape().QueryAPI()
	t.Require().Eventually(func() bool {
		status, err := api.SyncStatus(t.Ctx())
		t.Require().NoError(err)
		for chID, chStatus := range status.Chains {
			if chStatus.Finalized.Number < activationBlock.Number {
				t.Logger().Info("Chain not yet finalized past activation",
					"chain", chID, "final", chStatus.Finalized, "head", chStatus.LocalUnsafe)
				return false
			}
			t.Logger().Info("Chain finalized!",
				"chain", chID, "final", chStatus.Finalized, "head", chStatus.LocalUnsafe)
		}
		// All chains finalized past activation block
		return true
	}, time.Second*100, time.Second, "all chains must finalize a L2 block past the activation block")
	t.Logger().Info("Done")
}
