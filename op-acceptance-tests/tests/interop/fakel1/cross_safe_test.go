package fakel1

import (
	"fmt"
	"testing"
	"time"

	"github.com/ethereum-optimism/optimism/op-devstack/devtest"
	"github.com/ethereum-optimism/optimism/op-devstack/presets"
	"github.com/ethereum-optimism/optimism/op-devstack/stack/match"
	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/seqtypes"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

var SimpleInterop presets.TestSetup[*presets.SimpleInterop]
var sequencingControl chan string

// TestMain creates the test-setups against the shared backend
func TestMain(m *testing.M) {
	// Other setups may be added here, hydrated from the same orchestrator
	sequencingControl = make(chan string)
	presets.DoMain(m, presets.NewInteropWithFakeL1(&SimpleInterop, sequencingControl))
}

// TestReorgCrossSafeHead starts an interop chain with an op-test-sequencer, which takes control over sequencing the L1 chain,
// introduces a reorg on the L1 block, which the cross-safe head is pointing to, and then checks that the L2 chain is reorged
func TestReorgCrossSafeHead(gt *testing.T) {
	// heavy wip -- at the moment this test is not checking for cross-safe head, we are just reorging the L1 chain head without regard to the cross-safe head
	t := devtest.SerialT(gt)
	ctx := t.Ctx()

	sys := SimpleInterop(t)
	l := sys.Log

	el := sys.L1Network.Escape().L1ELNode(match.FirstL1EL)

	sys.L1Network.WaitForBlock()
	sys.L1Network.WaitForBlock()

	// reorg the L1 chain
	{
		sequencingControl <- "pause"

		sequenceL1Block(t, sys, common.Hash{})

		sys.L2ChainA.WaitForBlock()
		sys.L2ChainA.WaitForBlock()
		sys.L2ChainA.WaitForBlock()
		sys.L2ChainA.WaitForBlock()

		// print the chains before the reorg
		sys.L2ChainA.PrintChain()
		sys.L1Network.PrintChain()

		headL1, err := el.EthClient().InfoByLabel(ctx, "latest")
		require.NoError(t, err)

		l.Info("sequence an L1 blog with the same parent as the latest head", "number", headL1.NumberU64(), "hash", headL1.Hash(), "parent", headL1.ParentHash())
		sequenceL1Block(t, sys, headL1.ParentHash())

		sequencingControl <- "resume"
	}

	time.Sleep(30 * time.Second)

	// print the chains after the reorg
	sys.L2ChainA.PrintChain()
	sys.L1Network.PrintChain()
}

func TestL1Reorg(gt *testing.T) {
	t := devtest.SerialT(gt)
	ctx := t.Ctx()

	sys := SimpleInterop(t)
	l := sys.Log

	el := sys.L1Network.Escape().L1ELNode(match.FirstL1EL)

	sequencingControl <- "pause"

	var parent common.Hash
	for range 2 {
		l.Info("sequence an L1 blog")
		sequenceL1Block(t, sys, parent)
	}

	head, err := el.EthClient().InfoByLabel(ctx, "latest")
	require.NoError(t, err)

	sys.L1Network.PrintChain()

	l.Info("sequence an L1 blog with the same parent as the latest head", "number", head.NumberU64(), "hash", head.Hash(), "parent", head.ParentHash())

	sequenceL1Block(t, sys, head.ParentHash())

	sys.L1Network.PrintChain()

	sequencingControl <- "resume"

	sys.L1Network.WaitForBlock()

	nhead, err := el.EthClient().InfoByNumber(ctx, head.NumberU64())
	require.NoError(t, err)

	t.Require().NotEqual(head.Hash(), nhead.Hash(), fmt.Sprintf("head and nhead should be different, head: %s, nhead: %s", head.Hash(), nhead.Hash()))

	sys.L1Network.PrintChain()
}

func sequenceL1Block(t devtest.T, sys *presets.SimpleInterop, parent common.Hash) {
	l1s := sys.Sequencer.Escape().IndividualAPI(sys.L1Network.ChainID())

	err := l1s.New(t.Ctx(), seqtypes.BuildOpts{
		Parent: parent,
	})
	require.NoError(t, err)

	err = l1s.Next(t.Ctx())
	require.NoError(t, err)
}
