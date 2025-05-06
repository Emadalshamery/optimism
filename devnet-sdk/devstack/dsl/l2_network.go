package dsl

import (
	"time"

	"github.com/ethereum-optimism/optimism/devnet-sdk/devstack/devtest"
	"github.com/ethereum-optimism/optimism/devnet-sdk/devstack/stack"
	"github.com/ethereum-optimism/optimism/devnet-sdk/devstack/stack/match"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/retry"
)

// L2Network wraps a stack.L2Network interface for DSL operations
type L2Network struct {
	commonImpl
	inner stack.L2Network
}

// NewL2Network creates a new L2Network DSL wrapper
func NewL2Network(inner stack.L2Network) *L2Network {
	return &L2Network{
		commonImpl: commonFromT(inner.T()),
		inner:      inner,
	}
}

func (n *L2Network) String() string {
	return n.inner.ID().String()
}

func (n *L2Network) ChainID() eth.ChainID {
	return n.inner.ChainID()
}

// Escape returns the underlying stack.L2Network
func (n *L2Network) Escape() stack.L2Network {
	return n.inner
}

// LatestPreActivation finds the latest block before fork activation
func (n *L2Network) LatestPreActivation(t devtest.T, forkTimestamp *uint64) eth.BlockRef {
	require := t.Require()

	t.Gate().NotNil(forkTimestamp, "Must have fork configured")
	t.Gate().Greater(*forkTimestamp, uint64(0), "Must not start fork at genesis")

	activationBlockNum, err := n.Escape().RollupConfig().TargetBlockNumber(*forkTimestamp)
	require.NoError(err)

	el := n.Escape().L2ELNode(match.FirstL2EL)
	head, err := el.EthClient().BlockRefByLabel(t.Ctx(), eth.Unsafe)
	require.NoError(err)

	t.Logger().Info("Preparing",
		"head", head, "head_time", head.Time,
		"activationNum", activationBlockNum, "activationTime", *forkTimestamp)

	if head.Number < activationBlockNum {
		t.Logger().Info("No activation yet, checking head block instead")
		return head
	} else {
		t.Logger().Info("Reached activation block already, proceeding with last block before activation")
		v, err := el.EthClient().BlockRefByNumber(t.Ctx(), activationBlockNum-1)
		require.NoError(err)
		return v
	}
}

// AwaitActivation awaits the fork activation time, and returns the activation block
func (n *L2Network) AwaitActivation(t devtest.T, forkTimestamp *uint64) eth.BlockRef {
	require := t.Require()

	t.Gate().NotNil(forkTimestamp, "Must have fork configured")
	t.Gate().Greater(*forkTimestamp, uint64(0), "Must not start frok at genesis")

	upgradeTime := time.Unix(int64(*forkTimestamp), 0)

	if deadline, hasDeadline := t.Deadline(); hasDeadline {
		t.Gate().True(upgradeTime.Before(deadline), "test must not time out before upgrade happens")
	}

	activationBlockNum, err := n.Escape().RollupConfig().TargetBlockNumber(*forkTimestamp)
	require.NoError(err)

	now := time.Now()
	fromNow := upgradeTime.Sub(now)
	if fromNow > 0 {
		t.Logger().Info("Awaiting upgrade", "fromNow", fromNow,
			"upgradeTime", upgradeTime,
			"timestamp", *forkTimestamp,
			"activationBlock", activationBlockNum)

		select {
		case <-time.After(fromNow):
		case <-t.Ctx().Done():
			t.Require().FailNow("failed to await fork within test time")
		}
	}

	el := n.Escape().L2ELNode(match.FirstL2EL)
	// After activation time it may take some time for the activation block to show up
	activationBlock, err := retry.Do(t.Ctx(), 10, retry.Fixed(time.Second), func() (eth.BlockRef, error) {
		return el.EthClient().BlockRefByNumber(t.Ctx(), activationBlockNum)
	})
	require.NoError(err)

	t.Logger().Info("Activation block", "block", activationBlock)
	return activationBlock
}
